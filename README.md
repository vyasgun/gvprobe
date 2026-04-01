# gvprobe

A lightweight CLI for **tracing and injecting** DHCP and ARP packets into [gvisor-tap-vsock](https://github.com/containers/gvisor-tap-vsock) / `gvproxy` via the vfkit unixgram socket.

Useful for exploring and debugging gvproxy's virtual networking behavior from the host.

It lets you start/stop `gvproxy` and send real DHCP Discover and ARP request frames over the vfkit unixgram socket to observe how gvproxy processes them.

State is kept in `~/.gvprobe/` (PID file, sockets, logs).

## Quick Start

```bash
# Build
make

# Start gvproxy
gvprobe start

# Test DHCP and ARP
gvprobe trace dhcp
gvprobe trace arp --target 192.168.127.1

# Stop when done
gvprobe stop
```

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

## Development

```bash
make          # build
make lint
make fmt
```

Requires Go 1.24+.
