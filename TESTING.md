# Whiteout Protocol — Manual Test Guide

This document covers how to manually validate the broker↔proxy ring formation stack.
Follow this exactly and it will work. No prior context required.

---

## Prerequisites

- Go installed and on your PATH (`go version` should return something)
- Four terminal windows open
- All commands run from the repo root unless otherwise specified: `whiteout-protocol/`

---

## What You Are Testing

Three proxy nodes register with the broker. The broker groups them into a directed ring.
A client requests an assignment and receives a proxy IP, a warm spare IP, and a session token.
This validates the full signaling stack: registration → heartbeat → ring formation → assignment.

---

## Step 1 — Start the Broker

In **Terminal 1**:

```
cd broker
go run . -disable-tls -addr :8080 -disable-geoip
```

**Expected output:**
```
2026/xx/xx xx:xx:xx ring formation (initial): 0 rings active
```

The broker is now listening on port 8080. It will log ring formation every 90 seconds.
Leave this terminal running.

---

## Step 2 — Start Three Proxy Nodes

Each proxy needs a unique IP, listen port, and ID file. Open a new terminal for each.

**Terminal 2:**
```
cd proxy/cmd/whiteout-proxy
go run . -ip 127.0.0.1 -relay localhost:9999 -verbose
```

**Terminal 3:**
```
cd proxy/cmd/whiteout-proxy
go run . -ip 127.0.0.2 -relay localhost:9999 -verbose -id-file proxy-node-id-2 -listen 0.0.0.0:7001
```

**Terminal 4:**
```
cd proxy/cmd/whiteout-proxy
go run . -ip 127.0.0.3 -relay localhost:9999 -verbose -id-file proxy-node-id-3 -listen 0.0.0.0:7002
```

**Expected output per proxy (example):**
```
2026/xx/xx xx:xx:xx [whiteout-proxy] node_id=<32 hex chars> listen=0.0.0.0:7000 relay=localhost:9999 broker=http://localhost:8080
2026/xx/xx xx:xx:xx [whiteout-proxy] registered with broker
2026/xx/xx xx:xx:xx [whiteout-proxy] accepting connections on 0.0.0.0:7000
2026/xx/xx xx:xx:xx [heartbeat] ok        ← appears every 30 seconds
```

If registration fails, the broker is not running. Go back to Step 1.

Note: `localhost:9999` is a placeholder relay address. Nothing needs to be listening
there for this test — you are only validating signaling, not traffic flow.

---

## Step 3 — Wait for Ring Formation

Watch **Terminal 1** (the broker). Within 90 seconds of all three proxies registering
you will see:

```
2026/xx/xx xx:xx:xx ring formation: 1 rings active
```

If it still says `0 rings active`, wait another 90 seconds. Ring formation runs on a
90-second ticker. If it never forms, confirm all three proxies are still running and
heartbeating.

---

## Step 4 — Request an Assignment

In any terminal, from the repo root:

```
curl http://localhost:8080/v1/assign?client_id=test
```

**Expected response (HTTP 200):**
```json
{
  "ring_id": "ring-0001",
  "proxy_id": "<node 1 ID>",
  "proxy_ip": "127.0.0.1",
  "spare_id": "<node 3 ID>",
  "spare_ip": "127.0.0.3",
  "session_token": "<64 hex chars>"
}
```

This confirms:
- The broker found an active ring
- A proxy and warm spare were selected
- A real cryptographic session token was generated and stored

If you get `assignment failed`, the ring has not formed yet. Wait and retry.

---

## Cleanup

Stop all four terminals with Ctrl+C.

Delete the runtime node ID files generated during the test — these are not source files:

```
del proxy\cmd\whiteout-proxy\proxy-node-id
del proxy\cmd\whiteout-proxy\proxy-node-id-2
del proxy\cmd\whiteout-proxy\proxy-node-id-3
```

These files are already in `.gitignore` and will not be committed.

---

## What Is NOT Being Tested Here

- Actual traffic relay through the proxy (no client binary exists yet — M-003)
- WireGuard encryption layers (not yet implemented — LIM-005)
- Failover to the warm spare (untested until client exists)
- Load balancing (not implemented — LIM-003)

See `NOTES.md` for the full list of open limitations and next milestones.