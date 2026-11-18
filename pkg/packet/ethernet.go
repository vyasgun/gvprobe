// Package packet builds and parses the raw Ethernet frames gvprobe exchanges
// with gvproxy over its datapath sockets. All builders are pure functions so
// they can be unit-tested without a running gvproxy.
package packet

import (
	"fmt"
	"net"
	"slices"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// UDPEndpoints identifies the endpoints of a UDP exchange inside an Ethernet frame.
type UDPEndpoints struct {
	SrcMAC  net.HardwareAddr
	DstMAC  net.HardwareAddr
	SrcIP   net.IP // 4-byte form
	DstIP   net.IP
	SrcPort uint16
	DstPort uint16
}

// BuildUDPFrame produces an Ethernet frame wrapping an IPv4 packet
// containing a UDP datagram with the given payload.
func BuildUDPFrame(f UDPEndpoints, payload []byte) ([]byte, error) {
	if len(f.SrcMAC) != 6 {
		return nil, fmt.Errorf("invalid source MAC %q", f.SrcMAC)
	}
	if len(f.DstMAC) != 6 {
		return nil, fmt.Errorf("invalid destination MAC %q", f.DstMAC)
	}
	srcIP := f.SrcIP.To4()
	dstIP := f.DstIP.To4()
	if srcIP == nil || dstIP == nil {
		return nil, fmt.Errorf("IPv4 addresses required, got src %v dst %v", f.SrcIP, f.DstIP)
	}

	ip4 := &layers.IPv4{
		Version:  4,
		TTL:      64,
		Protocol: layers.IPProtocolUDP,
		SrcIP:    srcIP,
		DstIP:    dstIP,
	}
	udp := &layers.UDP{
		SrcPort: layers.UDPPort(f.SrcPort),
		DstPort: layers.UDPPort(f.DstPort),
	}
	if err := udp.SetNetworkLayerForChecksum(ip4); err != nil {
		return nil, fmt.Errorf("failed to set network layer for checksum: %w", err)
	}
	eth := &layers.Ethernet{
		SrcMAC:       f.SrcMAC,
		DstMAC:       f.DstMAC,
		EthernetType: layers.EthernetTypeIPv4,
	}
	return serialize(eth, ip4, udp, gopacket.Payload(payload))
}

// ICMPEndpoints identifies the endpoints of an ICMPv4 exchange.
type ICMPEndpoints struct {
	SrcMAC net.HardwareAddr
	DstMAC net.HardwareAddr
	SrcIP  net.IP
	DstIP  net.IP
}

// buildIPv4Ethernet serializes Eth > IPv4 with the given transport layers,
// optionally setting the Don't-Fragment flag.
func buildIPv4Ethernet(f ICMPEndpoints, proto layers.IPProtocol, df bool, transport ...gopacket.SerializableLayer) ([]byte, error) {
	if len(f.SrcMAC) != 6 {
		return nil, fmt.Errorf("invalid source MAC %q", f.SrcMAC)
	}
	if len(f.DstMAC) != 6 {
		return nil, fmt.Errorf("invalid destination MAC %q", f.DstMAC)
	}
	srcIP := f.SrcIP.To4()
	dstIP := f.DstIP.To4()
	if srcIP == nil || dstIP == nil {
		return nil, fmt.Errorf("IPv4 addresses required, got src %v dst %v", f.SrcIP, f.DstIP)
	}
	var flags layers.IPv4Flag
	if df {
		flags = layers.IPv4DontFragment
	}
	ip4 := &layers.IPv4{
		Version:  4,
		TTL:      64,
		Flags:    flags,
		Protocol: proto,
		SrcIP:    srcIP,
		DstIP:    dstIP,
	}
	eth := &layers.Ethernet{
		SrcMAC:       f.SrcMAC,
		DstMAC:       f.DstMAC,
		EthernetType: layers.EthernetTypeIPv4,
	}
	stack := append([]gopacket.SerializableLayer{eth, ip4}, transport...)
	return serialize(stack...)
}

// serialize writes the given layer stack (outermost first) into a single
// frame, computing length fields and checksums.
func serialize(stack ...gopacket.SerializableLayer) ([]byte, error) {
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}
	if err := gopacket.SerializeLayers(buf, opts, stack...); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Frame is a decoded inbound Ethernet frame. ParseFrame returns the variant
// matching the frame's contents. The unexported frame() method enforces the
// set, so only this package can produce implementations.
//
// The variants are either ARP, ICMP, or UDP frames.
type Frame interface{ frame() }

// ArpFrame is an Ethernet frame carrying an ARP message.
type ArpFrame struct {
	Eth *layers.Ethernet
	ARP *layers.ARP
}

func (ArpFrame) frame() {}

// IcmpFrame is an Ethernet frame carrying an IPv4 packet containing an ICMPv4
// message.
type IcmpFrame struct {
	Eth  *layers.Ethernet
	IP   *layers.IPv4
	ICMP *layers.ICMPv4
}

func (IcmpFrame) frame() {}

// Payload returns the ICMP message payload (the echo data).
func (f IcmpFrame) Payload() []byte { return f.ICMP.LayerPayload() }

// UdpFrame is an Ethernet frame carrying an IPv4 packet with a UDP datagram.
type UdpFrame struct {
	Eth *layers.Ethernet
	IP  *layers.IPv4
	UDP *layers.UDP
}

func (UdpFrame) frame() {}

// Payload returns the UDP datagram payload.
func (f UdpFrame) Payload() []byte { return f.UDP.Payload }

// UnknownFrame is a frame whose Ethernet header decoded but whose payload
// is not ARP, ICMP, or UDP (or failed to decode).
type UnknownFrame struct{ Eth *layers.Ethernet }

func (UnknownFrame) frame() {}

// ParseFrame decodes a raw Ethernet frame into the Frame variant matching
// its contents. It never panics: short or garbage input yields an error
// (upstream gvisor-tap-vsock#780 showed how dangerous unvalidated frames
// are, and gvprobe must not replicate that mistake). A DecodingLayerParser
// is used so payloads (e.g. UDP port 53) are never run through
// application-layer decoders like gopacket's DNS decoder.
func ParseFrame(frame []byte) (Frame, error) {
	if len(frame) < 14 {
		return nil, fmt.Errorf("frame too short for Ethernet header: %d bytes", len(frame))
	}
	eth := &layers.Ethernet{}
	ip4 := &layers.IPv4{}
	arp := &layers.ARP{}
	icmp := &layers.ICMPv4{}
	udp := &layers.UDP{}
	parser := gopacket.NewDecodingLayerParser(
		layers.LayerTypeEthernet, eth, ip4, arp, icmp, udp)
	decoded := []gopacket.LayerType{}
	if err := parser.DecodeLayers(frame, &decoded); err != nil {
		// No ethernet layer means the frame is invalid.
		// If it was only partially decoded, keep whatever
		// layers were successfully decoded.
		if !containsLayer(decoded, layers.LayerTypeEthernet) {
			return nil, err
		}
	}
	switch {
	case containsLayer(decoded, layers.LayerTypeUDP):
		return UdpFrame{Eth: eth, IP: ip4, UDP: udp}, nil
	case containsLayer(decoded, layers.LayerTypeICMPv4):
		return IcmpFrame{Eth: eth, IP: ip4, ICMP: icmp}, nil
	case containsLayer(decoded, layers.LayerTypeARP):
		return ArpFrame{Eth: eth, ARP: arp}, nil
	default:
		return UnknownFrame{Eth: eth}, nil
	}
}

func containsLayer(lts []gopacket.LayerType, desiredLt gopacket.LayerType) bool {
	return slices.Contains(lts, desiredLt)
}
