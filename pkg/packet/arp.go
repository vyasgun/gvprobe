package packet

import (
	"fmt"
	"net"

	"github.com/google/gopacket/layers"
)

// BuildArpRequest builds an Ethernet broadcast frame requesting target MAC address
// from source MAC and target IP.
func BuildArpRequest(srcMAC net.HardwareAddr, senderIP, targetIP net.IP) ([]byte, error) {
	if len(srcMAC) != 6 {
		return nil, fmt.Errorf("invalid source MAC %q", srcMAC)
	}
	arp := arpLayer(layers.ARPRequest, srcMAC, zeroMAC(), senderIP, targetIP)
	eth := &layers.Ethernet{
		SrcMAC:       srcMAC,
		DstMAC:       layers.EthernetBroadcast,
		EthernetType: layers.EthernetTypeARP,
	}
	return serialize(eth, arp)
}

// BuildArpReply builds a unicast reply containing the target MAC address to the ARP request.
func BuildArpReply(srcMAC, dstMAC net.HardwareAddr, senderIP, targetIP net.IP) ([]byte, error) {
	arp := arpLayer(layers.ARPReply, srcMAC, dstMAC, senderIP, targetIP)
	eth := &layers.Ethernet{
		SrcMAC:       srcMAC,
		DstMAC:       dstMAC,
		EthernetType: layers.EthernetTypeARP,
	}
	return serialize(eth, arp)
}

// BuildGratuitousArp builds an unsolicited ARP (GARP) reply and sends it to the
// broadcast address.
func BuildGratuitousArp(srcMAC net.HardwareAddr, senderIP net.IP) ([]byte, error) {
	if len(srcMAC) != 6 {
		return nil, fmt.Errorf("invalid source MAC %q", srcMAC)
	}
	return BuildArpReply(srcMAC, layers.EthernetBroadcast, senderIP, senderIP)
}

func arpLayer(op uint16, srcHW, dstHW net.HardwareAddr, senderIP, targetIP net.IP) *layers.ARP {
	srcIP4 := ipv4(senderIP)
	dstIP4 := ipv4(targetIP)
	if dstHW == nil {
		dstHW = zeroMAC()
	}
	return &layers.ARP{
		AddrType:          layers.LinkTypeEthernet,
		Protocol:          layers.EthernetTypeIPv4,
		HwAddressSize:     6,
		ProtAddressSize:   4,
		Operation:         op,
		SourceHwAddress:   srcHW,
		SourceProtAddress: srcIP4,
		DstHwAddress:      dstHW,
		DstProtAddress:    dstIP4,
	}
}

func ipv4(ip net.IP) net.IP {
	ip4 := ip.To4()
	if ip4 == nil {
		return make(net.IP, 4)
	}
	out := make(net.IP, 4)
	copy(out, ip4)
	return out
}

func zeroMAC() net.HardwareAddr {
	return make(net.HardwareAddr, 6)
}
