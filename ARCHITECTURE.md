# ARCHITECTURE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

gvprobe is a lightweight CLI for **tracing and injecting** DHCP and ARP packets into [gvisor-tap-vsock](https://github.com/containers/gvisor-tap-vsock) / `gvproxy` via the vfkit unixgram socket. It's used for exploring and debugging gvproxy's virtual networking behavior from the host.

State is kept in `~/.gvprobe/` (PID file, sockets, logs).

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
├── start          # Start gvproxy process
├── stop           # Stop gvproxy
├── status         # Check gvproxy status
├── version        # Show version info
└── trace          # Trace/inject packets
    ├── dhcp       # Send DHCP Discover packets
    └── arp        # Send ARP who-has requests
```

Commands are registered in `main.go:init()`. Each command lives in `cmd/` or `cmd/trace/`.

### Core Packages

**`pkg/gvproxy/`** — gvproxy lifecycle management
- `start.go`: Launch gvproxy subprocess with default config
- `stop.go`: Stop via PID file
- `status.go`: Check if running
- `pid.go`: PID file read/write/validation
- `config.go`: gvproxy configuration (network settings, sockets)
- `vfkit_local.go`: Manage vfkit-local.sock (client-side unixgram socket)

**`pkg/trace/`** — Packet injection and tracing
- `dhcp.go`: Build DHCP Discover packets, send via vfkit socket, parse Offers, check lease state via HTTP API
- `arp.go`: Build ARP requests, send/receive via vfkit socket

**`pkg/constants/`** — Paths and defaults
- Config dir: `~/.gvprobe/`
- Sockets: `vfkit.sock` (gvproxy listens), `vfkit-local.sock` (gvprobe binds)
- Default guest: MAC `5a:94:ef:e4:0c:ee`, IP `192.168.127.2`
- Gateway: `192.168.127.1`
- gvproxy services API: gvprobe starts gvproxy with `-services tcp://0.0.0.0:5555`; HTTP clients use `GvproxyURL` (`http://0.0.0.0:5555`) for `/services/dhcp/leases`.

**`internal/version/`** — Version/commit injected via ldflags at build time

**`internal/out/`** — Best-effort `Fprintf` / `Fprintln` for CLI output (errors from stdout are ignored on purpose).

### gvproxy Communication

gvprobe talks to gvproxy via **unixgram sockets**:
1. gvproxy listens on `~/.gvprobe/vfkit.sock` (configured via `--vfkit-listen`)
2. gvprobe binds `~/.gvprobe/vfkit-local.sock` lazily on first trace command
3. Raw Ethernet frames (built with gopacket) are sent/received over these sockets
4. DHCP lease state is queried via gvproxy's HTTP API at `/services/dhcp/leases`

The vfkit socket protocol emulates the vfkit VM interface, allowing host-side packet injection for testing.

### DHCP Tracing Flow

1. Check if lease exists for MAC via HTTP GET to `/services/dhcp/leases`
2. Build DHCP Discover message (using insomniacslk/dhcp library)
3. Wrap in Ethernet → IPv4 → UDP layers (using gopacket)
4. Send raw frame over vfkit-local.sock → vfkit.sock
5. Read reply with timeout (default 3s)
6. Parse reply, extract Offer
7. Report whether lease was reused or newly assigned

Supports:
- Custom MAC addresses
- Multiple requests (`--count N`) to test lease stability
- Human-readable output (default) or raw gopacket decode (`--machine`)

### ARP Tracing Flow

Similar to DHCP but simpler:
1. Build ARP "who-has" request for target IP (default: gateway 192.168.127.1)
2. Send over vfkit socket
3. Parse ARP reply

Note: ARP replies for guest IPs may be dropped by gvproxy's anti-spoofing protection. Gateway ARP works reliably.

## Key Implementation Details

- **Process isolation:** Each gvprobe invocation is a separate process. `start` and `trace` don't share state except via filesystem (PID file, sockets).
- **Lazy socket binding:** `vfkit_local.go` uses sync.Mutex to ensure vfkit-local.sock is bound once per process on first `WithVfkitLocal()` call.
- **PID management:** `gvproxy.pid` stores running gvproxy PID. Stale PIDs are detected via process existence check before starting new instance.
- **Version injection:** `Makefile` uses `-ldflags` to inject git tag/commit into `internal/version`.
