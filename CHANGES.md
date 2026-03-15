# Whiteout Protocol — Divergence from Snowflake

## Removed
- PT stdio / SOCKS5 client interface
- Browser-based ephemeral proxy
- smux + KCP transport stack
- AMP rendezvous

## Replaced With
- TUN device + WireGuard client tunnel
- Persistent Go proxy daemon
- WireGuard transport
- Direct HTTPS broker rendezvous

## Added
- Ring-based peer assignment
- Node quality scoring
- Warm spare proxy
- Session continuity across proxy churn