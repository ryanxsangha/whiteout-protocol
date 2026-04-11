// whiteout-proxy is the Whiteout Protocol relay node.
// It registers with the broker, maintains a heartbeat, and relays
// encrypted client traffic to the configured upstream.
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
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

	nodeID, err := loadOrCreateID(*flagIDFile)
	if err != nil {
		log.Fatalf("node ID: %v", err)
	}
	log.Printf("[whiteout-proxy] node_id=%s", nodeID)
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
