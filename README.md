# gvprobe

A CLI for **functional testing and tracing** of [gvisor-tap-vsock](https://github.com/containers/gvisor-tap-vsock) / `gvproxy`. It simulates a guest VM's NIC from the host: it acquires a DHCP lease and speaks ARP/ICMP/UDP over gvproxy's datapath sockets (vfkit, bess, qemu).

Useful for exploring and debugging gvproxy's virtual networking (upstream asks for exactly this in [containers/gvisor-tap-vsock#412](https://github.com/containers/gvisor-tap-vsock/issues/412)).

State is kept in `~/.gvprobe/` (PID file, sockets, logs).

## Quick Start

```bash
# Build
make

# Start gvproxy
gvprobe start

# Trace individual exchanges
gvprobe trace dhcp
gvprobe trace arp --target 192.168.127.1

# Stop when done
gvprobe stop
```

```

Exit codes: `0` all passed/skipped, `1` at least one failure, `2` setup error (gvproxy not running, bad flags).

Checks that guard a specific upstream issue record the issue URL in `--json` output, so results double as regression evidence. Current coverage includes [#780](https://github.com/containers/gvisor-tap-vsock/issues/780) (short-frame robustness), [#772](https://github.com/containers/gvisor-tap-vsock/issues/772) (MX NODATA vs NXDOMAIN), [#612](https://github.com/containers/gvisor-tap-vsock/issues/612) (SOA/SRV/PTR), [#563](https://github.com/containers/gvisor-tap-vsock/issues/563)/[#562](https://github.com/containers/gvisor-tap-vsock/issues/562) (TXT handling).

**Warning:** `--tags regression` includes destructive robustness probes (e.g. short frames) that can crash or deadlock older gvproxy builds. Run them against throwaway gvproxy instances.

## trace dhcp

Sends DHCP Discover packet(s). Supports custom MAC and multiple requests.

```bash
gvprobe trace dhcp                    # single request, human readable
gvprobe trace dhcp --count 5          # send 5 requests
gvprobe trace dhcp --mac aa:bb:cc:dd:ee:ff
gvprobe trace dhcp --machine          # raw gopacket decode
```

## trace arp

Sends ARP "who-has" request. Defaults to the gvproxy gateway (192.168.127.1).

```bash
gvprobe trace arp
gvprobe trace arp --target 192.168.127.1
gvprobe trace arp --mac 5a:94:ef:e4:0c:ee --target 192.168.127.1
```

Note: ARP replies for guest IPs may be dropped by gvproxy's anti-spoofing protection. Gateway ARP works reliably.

## attach

`gvprobe attach` holds a long-lived datapath session so qemu/bess backends don't see a disconnect between trace commands. Trace and test commands route through it automatically when it is running.

## Development

```bash
make          # build
make lint
make fmt
```

Requires Go 1.24+.
