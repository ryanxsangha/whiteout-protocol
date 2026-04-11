// whiteout-proxy is the Whiteout Protocol relay node.
// It registers with the broker, maintains a heartbeat, and relays
// encrypted client traffic to the configured upstream.
package main

import (
	"flag"
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
}
