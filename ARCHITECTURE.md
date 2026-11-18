# ARCHITECTURE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

gvprobe is a CLI for **functional testing and tracing** of [gvisor-tap-vsock](https://github.com/containers/gvisor-tap-vsock) / `gvproxy`. It simulates a guest VM's NIC from the host: raw Ethernet frames are exchanged with gvproxy over its VM datapath sockets, with a "guest micro-stack" (DHCP lease, ARP, ICMP, UDP) on top.

State is kept in `~/.gvprobe/` (PID file, sockets, logs, backend name).

## Build & Development Commands

```bash
make             # Build binary to bin/gvprobe
make install     # Install to $GOPATH/bin
make lint        # Run golangci-lint
make fmt         # Format code with goimports
```

**Requirements:** Go 1.24+

**Lint tools:** Pinned versions are installed to `tools/bin/` via `tools/tools.mk`:
- golangci-lint v2.11.4
- goimports (latest)

## Architecture

### CLI Structure

Built with cobra. Command hierarchy:
```
gvprobe
├── start          # Start gvproxy process (--backend vfkit|bess|qemu)
├── stop           # Stop gvproxy
├── status         # Check gvproxy status
├── version        # Show version info
├── attach         # Hold a persistent datapath session (proxy for trace)
├── trace          # One-off packet traces
│   ├── dhcp       # Send DHCP Discover packets
│   └── arp        # Send ARP who-has requests
└── agent          # AI-assisted analysis (Vertex AI)
    └── dhcp
```

Commands are registered in `main.go:init()`. Each command lives in `cmd/` or `cmd/<group>/`.

### Core Packages

**`pkg/packet/`** — Pure frame builders/parsers (no I/O)
- `ethernet.go`: UDP/ICMP frame builders, `ParseFrame` (never panics on short/garbage input — see gvisor-tap-vsock#780)
- `arp.go`, `dhcp.go`, `icmp.go`: protocol-specific builders
- `dns.go`: dependency-free DNS wire codec (query build, response parse incl. compression pointers, TXT character-strings)

**`pkg/gvproxy/`** — gvproxy lifecycle and HTTP services
- `start.go`/`stop.go`/`status.go`/`pid.go`: process management
- `config.go`: gvproxy command line (via vendored upstream `pkg/types`)
- `datapath_client.go`: client-side sockets (vfkit local bind, bess, qemu)
- `leases.go`: `GET /services/dhcp/leases` lookup
- `forwarder.go`: port-forward REST client (`/services/forwarder/{all,expose,unexpose}`)

**`pkg/gvproxy/datapath/`** — Session abstraction
- `session.go`: `Session` interface (`SendFrame`/`RecvFrame`), `WithSession`, `WithSessionOrAttach` (routes through the attach proxy when `attach.sock` exists)
- Backends: vfkit (unixgram `WriteTo`), bess (unixpacket, Linux-oriented), qemu (stream with 4-byte big-endian length prefix)

**`pkg/attach/`** — Persistent session proxy
- `attach.go`/`server.go`: holds one datapath Session open (qemu/bess gvproxy can exit on client disconnect) and proxies frames over `attach.sock`. Wire protocol per round-trip: `[4B BE timeout_ms][4B BE frame_len][frame]` → `[1B status][4B BE len][payload]`

**`pkg/trace/`** — One-off trace commands (DHCP/ARP), consumers of `pkg/packet`

**`pkg/constants/`** — Paths, subnet (192.168.127.0/24, guest .2, gateway .1), services API URL, backend persistence (`~/.gvprobe/backend`)

**`internal/version/`**, **`internal/out/`** — ldflags version; best-effort output helpers

**`pkg/ai/`** — Anthropic via Vertex AI (used by `agent dhcp`)

### gvproxy Communication

1. gvproxy listens on one datapath socket per backend (`vfkit.sock` unixgram, `bess.sock` unixpacket, `qemu.sock` stream)
2. gvprobe connects/binds lazily; `WithSessionOrAttach` prefers the attach proxy when running
3. Raw Ethernet frames (gopacket) are exchanged; gvproxy's switch learns our MAC into its CAM table from the first frame
4. gvproxy's HTTP services API (`http://0.0.0.0:5555`) is used for DHCP leases and port forwarding

### Upstream regression notes

Known gvisor-tap-vsock issues worth probing: #780 (short frames), #772 (MX NODATA), #612 (SOA/SRV/PTR), #563/#562 (TXT).

Note: ARP replies for guest IPs may be dropped by gvproxy's anti-spoofing protection. Gateway ARP works reliably. Never attempt to bypass the anti-spoofing protection.

## Key Implementation Details

- **Process isolation:** Each gvprobe invocation is a separate process; state is shared only via the filesystem (PID files, sockets, backend file).
- **Lazy socket binding:** `datapath_client.go` uses sync.Mutex to bind client sockets once per process.
- **PID management:** stale PIDs are detected via process existence checks.
- **Parse safety:** all frame/message parsers must return errors, never panic, on malformed input — gvprobe must not replicate the class of bug it probes (upstream #780).
- **Version injection:** `Makefile` uses `-ldflags` to inject git tag/commit into `internal/version`.
