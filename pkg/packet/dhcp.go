package packet

import (
	"fmt"
	"net"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/insomniacslk/dhcp/dhcpv4"
)

// BuildDhcpDiscover builds a DHCP Discover message for the given client MAC.
func BuildDhcpDiscover(mac net.HardwareAddr) (*dhcpv4.DHCPv4, error) {
	if len(mac) != 6 {
		return nil, fmt.Errorf("invalid MAC %q", mac)
	}
	return dhcpv4.New(
		dhcpv4.WithClientIP(net.IPv4(0, 0, 0, 0).To4()),
		dhcpv4.WithMessageType(dhcpv4.MessageTypeDiscover),
		dhcpv4.WithHwAddr(mac),
		dhcpv4.WithBroadcast(true),
		dhcpv4.WithOption(dhcpv4.OptParameterRequestList(
			dhcpv4.OptionSubnetMask,
			dhcpv4.OptionRouter,
			dhcpv4.OptionDomainNameServer,
			dhcpv4.OptionInterfaceMTU,
		)))
}

// BuildDhcpDiscoverFrame builds an Ethernet frame wrapping a UDP
// packet containing the given DHCP Discover message.
func BuildDhcpDiscoverFrame(dhcpMsg *dhcpv4.DHCPv4) ([]byte, error) {
	udp := &layers.UDP{
		SrcPort: layers.UDPPort(dhcpv4.ClientPort),
		DstPort: layers.UDPPort(dhcpv4.ServerPort),
	}
	ip4 := &layers.IPv4{
		Version:  4,
		TTL:      64,
		Protocol: layers.IPProtocolUDP,
		SrcIP:    net.IPv4(0, 0, 0, 0).To4(),
		DstIP:    net.IPv4(255, 255, 255, 255).To4(),
	}
	if err := udp.SetNetworkLayerForChecksum(ip4); err != nil {
		return nil, fmt.Errorf("failed to set network layer for checksum: %w", err)
	}
	eth := &layers.Ethernet{
		SrcMAC:       dhcpMsg.ClientHWAddr,
		DstMAC:       layers.EthernetBroadcast,
		EthernetType: layers.EthernetTypeIPv4,
	}
	return serialize(eth, ip4, udp, gopacket.Payload(dhcpMsg.ToBytes()))
}

// ParseDhcpFromFrame extracts the DHCPv4 message from a UDP Ethernet frame.
func ParseDhcpFromFrame(frame []byte) (*dhcpv4.DHCPv4, error) {
	p, err := ParseFrame(frame)
	if err != nil {
		return nil, err
	}
	udp, ok := p.(UdpFrame)
	if !ok {
		return nil, fmt.Errorf("no UDP layer in Ethernet frame")
	}
	return dhcpv4.FromBytes(udp.Payload())
}
