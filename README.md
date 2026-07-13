# Whiteout Protocol

A privacy-focused network relay system. Client traffic is routed through
volunteer proxy nodes to an exit server using double-layer WireGuard
encryption, so no single node in the path can link who you are to what
you're accessing.

> [!WARNING]
> This software is **experimental and has not been tested or audited**. It
> has not received any external security review and may contain
> vulnerabilities. Do not use it for sensitive use cases, and do not rely
> on its security or anonymity properties until it has been reviewed.
>
> No crawling, scraping, archiving, or use of this repository's contents
> for AI/ML training is permitted.

## How It Works
```
client ──► proxy node ──► exit server ──► destination
```
- **Client** encrypts traffic in two WireGuard layers and sends it to an
  assigned proxy node.
- **Proxy node** sees the client's IP but not the destination. It strips
  the outer layer and relays to the exit server.
- **Exit server** sees the destination but not the client's IP. It strips
  the inner layer and forwards traffic onward.
- **Broker** handles node registration, ring formation, and proxy
  assignment. It sits entirely off the traffic path and never sees user
  traffic.

### Rings

Proxy nodes are organized into directed rings (A→B→C→A). Rings provide
correlation resistance that a flat proxy pool cannot: no single proxy
holds enough information to correlate a client with a destination. For a
privacy-first threat model, this is a non-negotiable design decision,
analogous in spirit to Tor's 3-hop circuit design.

## Structure of this Repository

- `broker/` — node registration, heartbeat and stale eviction, directed
  ring formation, proxy assignment, session token generation
- `proxy/` — standalone proxy node: ID persistence, broker registration,
  heartbeat, TCP relay
- `client/` — client binary (in development)
- `common/` — shared libraries

Portions of this repository are inherited from the Snowflake project and
are being replaced. Snowflake-derived code (`server/`, `probetest/`,
`doc/`, and parts of `client/`) is parked and will be removed once the
protocol is functionally complete.

## Status

Experimental. Under active development.

- Broker core — registration, heartbeat, eviction, ring formation,
  assignment, session tokens
- Proxy node — registration, heartbeat, relay, graceful shutdown;
  validated end-to-end with a three-node ring
- Client binary — not yet started
- WireGuard tunneling — relay is currently raw TCP

Known limitations and design decisions are tracked in
[NOTES.md](NOTES.md).

## License

See [LICENSE](LICENSE).