# Whiteout Protocol Dev Notes

---

## Milestone Log

### [M-001] Broker Core — Complete
Node registration, heartbeat, stale eviction, ring formation, proxy assignment,
real session token generation, and session lookup are all implemented and tested.
Broker is ephemeral in-memory by design (DEC-002).

### [M-002] Proxy Node — Complete (2026-04-11)
`proxy/cmd/whiteout-proxy` is implemented and validated end-to-end against a live broker.
Three proxy nodes registered, a ring formed, and a full assignment was returned including
ring ID, proxy ID, proxy IP, spare ID, spare IP, and a real crypto/rand session token.
The broker↔proxy contract is proven.

**What was validated:**
- Node ID generated via crypto/rand, persisted to disk, stable across restarts
- Registration hits `/v1/register` on startup successfully
- Heartbeat fires every 30s, keeps node alive in broker's nodeIndex
- TCP relay listener accepts connections and splices bidirectionally to upstream
- Graceful shutdown on SIGINT/SIGTERM drains goroutines cleanly
- Three nodes formed `ring-0001` within 90s of registration
- `/v1/assign` returned a complete ProxyAssignment with valid session token

### [M-003] Client Binary — Not Started
The client is the final piece of the core triangle. It must:
- Hit `/v1/assign` and parse the ProxyAssignment response
- Connect to `proxy_ip:listen_port` using the session token
- Push traffic through the proxy to the upstream relay

Once client↔proxy↔relay is end-to-end, the core protocol is proven in traffic,
not just in signaling.

---

## Known Limitations

### [LIM-001] Eviction / Formation Race
**Type:** Known Limitation
**Component:** broker/ring.go, broker/broker.go
**Status:** Open

`StartEviction` and `StartRingFormation` run on independent tickers with no coordination.
Formation can build rings around nodes eviction is about to delete. The mutex prevents data
corruption but not logical inconsistency — ring structs can hold stale node references after
eviction clears `nodeIndex`.

**Fix:** Make eviction ring-aware. On node deletion, remove from ring and trigger re-ring of
that ring only. Make formation incremental — only touch degraded or unformed rings.

---

### [LIM-002] SessionToken is a Placeholder
**Type:** Known Limitation
**Component:** broker/ring.go — AssignProxy
**Status:** Closed

`SessionToken` is currently `"token-" + clientID`. No real session is created or stored.
Turbotunnel ClientID session continuity is not yet implemented.

**Fix:** Generate a real token (crypto/rand), store a Session struct in RingRegistry,
wire up session lookup for reconnection.

---

### [LIM-003] AssignProxy Has No Load Balancing
**Type:** Known Limitation
**Component:** broker/ring.go — AssignProxy
**Status:** Open

Assignment picks the first eligible node in the first available ring. NodeScore fields
(Uptime, Bandwidth, Latency, Composite) exist on the struct but are never populated or used.

**Fix:** Implement score population via heartbeat telemetry, weight assignment toward
high-composite nodes. Premium tier should prefer high-uptime rings.

---

### [LIM-004] Global Ring Re-Formation Creates Churn
**Type:** Known Limitation
**Component:** broker/broker.go — StartRingFormation
**Status:** Open

Every 90s all rings are torn down and rebuilt globally regardless of stability. At scale
this forces unnecessary re-routing for nodes that haven't changed.

**Fix:** Incremental formation — only re-ring degraded rings. Depends on LIM-001 fix.

---

### [LIM-005] Proxy Relay is Raw TCP — No WireGuard Yet
**Type:** Known Limitation
**Component:** proxy/cmd/whiteout-proxy
**Status:** Open

The proxy currently accepts raw TCP connections and splices them to the upstream relay.
The double-layer WireGuard encryption described in DEC-003 is not yet wired in.
The outer tunnel (client→proxy) and inner tunnel (proxy→exit) are both missing.

**Fix:** Wire WireGuard handshake into the proxy relay layer once the client binary
exists and the end-to-end traffic path is established.

---

## Architecture Decisions

### [DEC-001] Eviction Timeout Set to 3 Minutes
**Type:** Architecture Decision
**Component:** broker/broker.go — StartEviction

Originally 60s. Bumped to 3m to give nodes margin above the 30s eviction ticker cadence.
Production nodes heartbeat every 30s so 60s was too tight for any variance.

**Revisit:** When heartbeat interval becomes configurable, eviction timeout should scale
proportionally (e.g. 3× heartbeat interval).

---

### [DEC-002] Broker is Ephemeral In-Memory Only
**Type:** Architecture Decision
**Component:** broker/

Deliberate. Broker stores only IPs, ring assignments, and timestamps — nothing persists to
disk. Structural defense against Anom-style centralized compromise. A subpoena or server
seizure yields nothing actionable.

---

### [DEC-003] Double-Layer WireGuard Encryption
**Type:** Architecture Decision
**Component:** Architecture

Inner WireGuard tunnel to broker-run exit servers. Outer WireGuard tunnel to proxy for NAT
traversal. Neither layer alone exposes the full path. Proxy sees client IP but not
destination. Exit server sees destination but not client IP.

---

### [DEC-004] Warm Spare Assigned Within Ring
**Type:** Architecture Decision
**Component:** broker/ring.go — FormRings

WarmSpare is the node 2 hops ahead within the same ring, not an external standby. Keeps the
spare already in the trust group and avoids cold failover latency.

---

### [DEC-005] Discarded Transport Stack Components
**Type:** Architecture Decision
**Component:** Architecture

The following were evaluated and discarded during design:
- SOCKS5 / PT stdio — incompatible with the broker blind-relay model
- smux + KCP stack — replaced by turbotunnel ClientID for session continuity
- AMP rendezvous — unnecessary complexity for current threat model

---

### [DEC-006] Directed Ring Topology (A→B→C→A)
**Type:** Architecture Decision
**Component:** Architecture

Nodes are organized into directed rings rather than a flat proxy pool. Each node has a fixed
NextHop and WarmSpare. Correlation resistance comes from broker cross-pairing — the broker
avoids assigning rings where nodes share observable network proximity.

---

### [DEC-007] Proxy Node ID Persisted to Disk
**Type:** Architecture Decision
**Component:** proxy/cmd/whiteout-proxy — loadOrCreateID

Node IDs are generated via crypto/rand on first startup and written to a local file
(default: `proxy-node-id`). On restart the same ID is reused so the broker treats the
node as continuous rather than re-registering a stranger. The file is excluded from
version control — it is a runtime artifact, not source.

**Revisit:** In a production deployment, node ID should be provisioned at deploy time
rather than generated opportunistically, so IDs are tracked and attributable.

---