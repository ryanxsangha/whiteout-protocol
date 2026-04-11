// whiteout-proxy is the Whiteout Protocol relay node.
// It registers with the broker, maintains a heartbeat, and relays
// encrypted client traffic to the configured upstream.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

var (
	flagBroker   = flag.String("broker", "http://localhost:8080", "broker base URL (no trailing slash)")
	flagListen   = flag.String("listen", "0.0.0.0:7000", "address:port to accept client connections on")
	flagRelay    = flag.String("relay", "", "upstream relay address to forward traffic to (host:port, required)")
	flagPublicIP = flag.String("ip", "", "public IP to advertise to the broker (required)")
	flagNATType  = flag.String("nat-type", "unrestricted", "NAT type to report: unrestricted | restricted | symmetric")
	flagIDFile   = flag.String("id-file", "proxy-node-id", "file to persist the node ID across restarts")
	flagVerbose  = flag.Bool("verbose", false, "verbose connection logging")
)

func main() {
	flag.Parse()

	if *flagRelay == "" {
		log.Fatal("flag -relay is required (e.g. 10.0.0.1:51820)")
	}
	if *flagPublicIP == "" {
		log.Fatal("flag -ip is required (public IP this node is reachable at)")
	}

	nodeID, err := loadOrCreateID(*flagIDFile)
	if err != nil {
		log.Fatalf("node ID: %v", err)
	}
	log.Printf("[whiteout-proxy] node_id=%s listen=%s relay=%s broker=%s",
		nodeID, *flagListen, *flagRelay, *flagBroker)

	if err := register(nodeID, *flagPublicIP, *flagNATType); err != nil {
		log.Fatalf("broker registration failed: %v", err)
	}
	log.Println("[whiteout-proxy] registered with broker")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		runHeartbeat(ctx, nodeID)
	}()

	ln, err := net.Listen("tcp", *flagListen)
	if err != nil {
		log.Fatalf("listen %s: %v", *flagListen, err)
	}
	log.Printf("[whiteout-proxy] accepting connections on %s", *flagListen)

	wg.Add(1)
	go func() {
		defer wg.Done()
		runListener(ctx, ln)
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Println("[whiteout-proxy] shutting down…")
	cancel()
	ln.Close()
	wg.Wait()
	log.Println("[whiteout-proxy] stopped")
}

// loadOrCreateID reads a node ID from path, or generates and writes a new one.
// ID is 32 hex chars (16 random bytes) — stable across restarts.
func loadOrCreateID(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		id := string(data)
		if len(id) == 32 {
			return id, nil
		}
		log.Printf("[id] existing ID at %s looks malformed (%d chars), regenerating", path, len(id))
	}

	b := make([]byte, 16)
	f, err := os.Open("/dev/urandom")
	if err != nil {
		return "", fmt.Errorf("open /dev/urandom: %w", err)
	}
	defer f.Close()
	if _, err := io.ReadFull(f, b); err != nil {
		return "", fmt.Errorf("read urandom: %w", err)
	}
	id := fmt.Sprintf("%x", b)

	if err := os.WriteFile(path, []byte(id), 0600); err != nil {
		log.Printf("[id] warn: could not persist node ID to %s: %v", path, err)
	}
	return id, nil
}

// register sends a one-shot registration request to the broker.
func register(nodeID, ip, natType string) error {
	url := fmt.Sprintf("%s/v1/register?node_id=%s&ip=%s&nat_type=%s",
		*flagBroker, nodeID, ip, natType)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("broker returned %d: %s", resp.StatusCode, body)
	}
	return nil
}

// runHeartbeat pings /v1/heartbeat every 30s until ctx is cancelled.
// Logs warnings on failure but never kills the process — transient broker
// blips should not take down a relay node.
func runHeartbeat(ctx context.Context, nodeID string) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			url := fmt.Sprintf("%s/v1/heartbeat?node_id=%s", *flagBroker, nodeID)
			resp, err := http.Get(url)
			if err != nil {
				log.Printf("[heartbeat] warn: %v", err)
				continue
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				log.Printf("[heartbeat] warn: broker returned %d", resp.StatusCode)
			} else if *flagVerbose {
				log.Println("[heartbeat] ok")
			}
		}
	}
}

// runListener accepts connections until ctx is cancelled.
func runListener(ctx context.Context, ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				log.Printf("[listener] accept error: %v", err)
				return
			}
		}
		go handleConn(ctx, conn)
	}
}

// handleConn dials the upstream relay and splices the two connections
// bidirectionally. Uses CloseWrite so neither side hangs on EOF.
func handleConn(ctx context.Context, client net.Conn) {
	defer client.Close()

	if *flagVerbose {
		log.Printf("[relay] new connection from %s", client.RemoteAddr())
	}

	relay, err := net.DialTimeout("tcp", *flagRelay, 10*time.Second)
	if err != nil {
		log.Printf("[relay] dial %s failed: %v", *flagRelay, err)
		return
	}
	defer relay.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, _ = io.Copy(relay, client)
		if tc, ok := relay.(*net.TCPConn); ok {
			tc.CloseWrite()
		}
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(client, relay)
		if tc, ok := client.(*net.TCPConn); ok {
			tc.CloseWrite()
		}
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		if *flagVerbose {
			log.Printf("[relay] connection from %s closed cleanly", client.RemoteAddr())
		}
	case <-ctx.Done():
		client.Close()
		relay.Close()
	}
}
