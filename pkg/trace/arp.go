package trace

import (
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/vyasgun/gvprobe/pkg/gvproxy"
)

type ArpTraceOpts struct {
	Target   string // who-has (query) IPv4
	SenderIP string
	MAC      string
	Machine  bool
}

const arpReadTimeout = 3 * time.Second

func TraceArp(opts ArpTraceOpts) error {
	status, err := gvproxy.Status()
	if err != nil {
		return err
	}
	if status != gvproxy.Running {
		return fmt.Errorf("gvproxy is not running")
	}
	mac, err := net.ParseMAC(opts.MAC)
	if err != nil {
		return fmt.Errorf("invalid MAC %q: %w", opts.MAC, err)
	}
	target := net.ParseIP(opts.Target)
	if target == nil {
		return fmt.Errorf("invalid IP %q", opts.Target)
	}
	target4 := target.To4()
	if target4 == nil {
		return fmt.Errorf("ARP trace needs an IPv4 address, got %q", opts.Target)
	}

	sender := net.ParseIP(opts.SenderIP)
	if sender == nil {
		return fmt.Errorf("invalid sender IP %q", opts.SenderIP)
	}
	sender4 := sender.To4()
	if sender4 == nil {
		return fmt.Errorf("ARP sender must be IPv4, got %q", opts.SenderIP)
	}

	arp := buildArpRequest(mac, sender4, target4)
	eth := &layers.Ethernet{
		SrcMAC:       mac,
		DstMAC:       layers.EthernetBroadcast,
		EthernetType: layers.EthernetTypeARP,
	}

	buf := gopacket.NewSerializeBuffer()
	serializeOpts := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: false}
	if err := gopacket.SerializeLayers(buf, serializeOpts, eth, arp); err != nil {
		return fmt.Errorf("failed to serialize ARP packet: %w", err)
	}

	if err := gvproxy.WithVfkitLocal(func(c *net.UnixConn) error {
		w := os.Stdout
		if opts.Machine {
			writeArpEthernetTrace(w, "outbound L2 frame (ARP request)", buf.Bytes())
		} else {
			writeHumanArpRequest(w, opts.Target, mac.String())
		}

		if _, err := c.WriteTo(buf.Bytes(), vfkitRemote); err != nil {
			return fmt.Errorf("failed to write ARP packet: %w", err)
		}

		_ = c.SetReadDeadline(time.Now().Add(arpReadTimeout))
		reply := make([]byte, 65536)
		n, _, err := c.ReadFrom(reply)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				if !opts.Machine {
					fmt.Fprintf(w, "\nNo ARP reply within %s\n", arpReadTimeout)
					return nil
				}
			}
			return fmt.Errorf("failed to read ARP reply: %w", err)
		}

		replyMsg, err := parseArpFromEthernet(reply[:n])
		if err != nil {
			return fmt.Errorf("failed to parse ARP reply: %w", err)
		}
		if opts.Machine {
			writeArpEthernetTrace(w, "inbound L2 frame (ARP reply)", reply[:n])
		} else {
			writeHumanArpReply(w, replyMsg)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to trace ARP: %w", err)
	}
	return nil
}

func buildArpRequest(mac net.HardwareAddr, sender4, target4 net.IP) *layers.ARP {
	srcIP := make(net.IP, 4)
	copy(srcIP, sender4)
	dstIP := make(net.IP, 4)
	copy(dstIP, target4)
	dstHW := make(net.HardwareAddr, 6)

	return &layers.ARP{
		AddrType:          layers.LinkTypeEthernet,
		Protocol:          layers.EthernetTypeIPv4,
		HwAddressSize:     6,
		ProtAddressSize:   4,
		Operation:         layers.ARPRequest,
		SourceHwAddress:   mac,
		SourceProtAddress: srcIP,
		DstHwAddress:      dstHW,
		DstProtAddress:    dstIP,
	}
}
func parseArpFromEthernet(packet []byte) (*layers.ARP, error) {
	pkt := gopacket.NewPacket(packet, layers.LayerTypeEthernet, gopacket.Default)
	if arpLayer := pkt.Layer(layers.LayerTypeARP); arpLayer != nil {
		return arpLayer.(*layers.ARP), nil
	}
	return nil, fmt.Errorf("no ARP layer in Ethernet frame")
}

func writeHumanArpRequest(w io.Writer, targetIP, sourceMAC string) {
	fmt.Fprintf(w, "ARP Trace — Target: %s\n\n", targetIP)
	fmt.Fprintf(w, "Sent ARP Request (Who has %s?)\n", targetIP)
	fmt.Fprintf(w, "   Source MAC : %s\n", sourceMAC)
}

func writeArpEthernetTrace(w io.Writer, title string, frame []byte) {
	fmt.Fprintf(w, "\n-- %s --\n", title)
	fmt.Fprintf(w, "%d byte(s)\n\n", len(frame))

	pkt := gopacket.NewPacket(frame, layers.LayerTypeEthernet, gopacket.Default)
	if errLayer := pkt.ErrorLayer(); errLayer != nil {
		fmt.Fprintf(w, "(layer decode error: %v)\n\n", errLayer.Error())
	}
	fmt.Fprintln(w, strings.TrimSpace(pkt.String()))
}

func writeHumanArpReply(w io.Writer, replyMsg *layers.ARP) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "-> Received ARP Reply")
	// Reply: Sender = responder; Target = original requester (RFC 826).
	fmt.Fprintf(w, "   Sender MAC      : %s\n", net.HardwareAddr(replyMsg.SourceHwAddress).String())
	fmt.Fprintf(w, "   Sender IP       : %s\n", net.IP(replyMsg.SourceProtAddress).String())
	fmt.Fprintf(w, "   Target MAC      : %s\n", net.HardwareAddr(replyMsg.DstHwAddress).String())
	fmt.Fprintf(w, "   Target IP       : %s  (requester)\n", net.IP(replyMsg.DstProtAddress).String())
	fmt.Fprintln(w)
	fmt.Fprintln(w, "ARP handshake successful")
}
