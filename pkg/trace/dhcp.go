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
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/vyasgun/gvprobe/pkg/constants"
	"github.com/vyasgun/gvprobe/pkg/gvproxy"
)

// guestMAC matches gvproxy's default DHCP static lease for 192.168.127.2.
var guestMAC = net.HardwareAddr{0x5a, 0x94, 0xef, 0xe4, 0x0c, 0xee}

var vfkitRemote = &net.UnixAddr{Name: constants.VfkitSocket, Net: "unixgram"}

const dhcpReadTimeout = 3 * time.Second

func TraceDhcp(machine bool) error {
	status, err := gvproxy.Status()
	if err != nil {
		return err
	}
	if status != gvproxy.Running {
		return fmt.Errorf("gvproxy is not running")
	}

	dhcpMsg, err := buildDhcpDiscover()
	if err != nil {
		return fmt.Errorf("failed to build DHCP discover: %w", err)
	}

	frame, err := buildEthernetFrame(dhcpMsg)
	if err != nil {
		return fmt.Errorf("failed to build Ethernet frame: %w", err)
	}

	return gvproxy.WithVfkitLocal(func(c *net.UnixConn) error {
		return dhcpDiscoverRoundTrip(c, frame, dhcpReadTimeout, os.Stdout, machine)
	})
}

func dhcpDiscoverRoundTrip(c *net.UnixConn, frame []byte, readTimeout time.Duration, w io.Writer, machine bool) error {
	if machine {
		writeDhcpEthernetTrace(w, "outbound L2 frame (DHCP Discover -> vfkit)", frame)
	} else {
		writeHumanDiscover(w)
	}

	if _, err := vfkitWriteFrame(c, frame); err != nil {
		return err
	}

	_ = c.SetReadDeadline(time.Now().Add(readTimeout))
	reply := make([]byte, 65536)
	n, _, err := c.ReadFrom(reply)
	if err != nil {
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			if machine {
				fmt.Fprintf(w, "\n── inbound (no reply) ──\nno DHCP response within %s\n", readTimeout)
			} else {
				fmt.Fprintf(w, "\nNo DHCP reply within %s\n", readTimeout)
			}
			return nil
		}
		return fmt.Errorf("read DHCP reply: %w", err)
	}

	replyMsg, err := parseDhcpFromEthernet(reply[:n])
	if err != nil {
		if machine {
			writeDhcpEthernetTrace(w, "inbound L2 frame (reply from vfkit)", reply[:n])
			return nil
		}
		return fmt.Errorf("decode DHCP reply: %w", err)
	}

	if machine {
		writeDhcpEthernetTrace(w, "inbound L2 frame (reply from vfkit)", reply[:n])
		return nil
	}
	writeHumanOffer(w, replyMsg)
	return nil
}

func vfkitWriteFrame(c *net.UnixConn, frame []byte) (int, error) {
	return c.WriteTo(frame, vfkitRemote)
}

func buildDhcpDiscover() (*dhcpv4.DHCPv4, error) {
	return dhcpv4.New(
		dhcpv4.WithClientIP(net.IPv4(0, 0, 0, 0).To4()),
		dhcpv4.WithMessageType(dhcpv4.MessageTypeDiscover),
		dhcpv4.WithHwAddr(guestMAC),
		dhcpv4.WithBroadcast(true),
		dhcpv4.WithOption(dhcpv4.OptParameterRequestList(
			dhcpv4.OptionSubnetMask,
			dhcpv4.OptionRouter,
			dhcpv4.OptionDomainNameServer,
			dhcpv4.OptionInterfaceMTU,
		)))
}

func buildEthernetFrame(dhcpMsg *dhcpv4.DHCPv4) ([]byte, error) {
	dhcpBytes := dhcpMsg.ToBytes()

	ip4 := &layers.IPv4{
		Version:  4,
		TTL:      64,
		Protocol: layers.IPProtocolUDP,
		SrcIP:    dhcpMsg.ClientIPAddr,
		DstIP:    dhcpMsg.ServerIPAddr,
	}

	udp := &layers.UDP{
		SrcPort: layers.UDPPort(dhcpv4.ClientPort),
		DstPort: layers.UDPPort(dhcpv4.ServerPort),
	}

	if err := udp.SetNetworkLayerForChecksum(ip4); err != nil {
		return nil, fmt.Errorf("failed to set network layer for checksum: %w", err)
	}
	eth := &layers.Ethernet{
		SrcMAC:       guestMAC,
		DstMAC:       layers.EthernetBroadcast,
		EthernetType: layers.EthernetTypeIPv4,
	}

	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}
	if err := gopacket.SerializeLayers(buf, opts, eth, ip4, udp, gopacket.Payload(dhcpBytes)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func parseDhcpFromEthernet(packet []byte) (*dhcpv4.DHCPv4, error) {
	pkt := gopacket.NewPacket(packet, layers.LayerTypeEthernet, gopacket.Default)
	if udpLayer := pkt.Layer(layers.LayerTypeUDP); udpLayer != nil {
		udp := udpLayer.(*layers.UDP)
		return dhcpv4.FromBytes(udp.Payload)
	}
	return nil, fmt.Errorf("no UDP layer in Ethernet frame")
}

func writeDhcpEthernetTrace(w io.Writer, title string, frame []byte) {
	fmt.Fprintf(w, "\n-- %s --\n", title)
	fmt.Fprintf(w, "%d byte(s)\n\n", len(frame))

	pkt := gopacket.NewPacket(frame, layers.LayerTypeEthernet, gopacket.Default)
	if errLayer := pkt.ErrorLayer(); errLayer != nil {
		fmt.Fprintf(w, "(layer decode error: %v)\n\n", errLayer.Error())
	}
	fmt.Fprintln(w, strings.TrimSpace(pkt.String()))

	if d, err := parseDhcpFromEthernet(frame); err == nil {
		fmt.Fprintln(w)
		fmt.Fprintln(w, d.Summary())
	}
	fmt.Fprintln(w)
}

func writeHumanDiscover(w io.Writer) {
	fmt.Fprintln(w, "DHCP Trace")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "-> Sent DHCP Discover")
	fmt.Fprintf(w, "   Client MAC       : %s\n", guestMAC.String())
	fmt.Fprintln(w, "   Asking for IP")
}

func writeHumanOffer(w io.Writer, msg *dhcpv4.DHCPv4) {
	fmt.Fprintln(w)
	fmt.Fprintf(w, "-> Received DHCP %s from gvproxy\n", msg.MessageType())
	fmt.Fprintf(w, "   Offered IP       : %s\n", msg.YourIPAddr)
	fmt.Fprintf(w, "   Subnet Mask      : %s\n", maskString(msg.SubnetMask()))
	fmt.Fprintf(w, "   Gateway          : %s\n", firstIP(msg.Router()))
	fmt.Fprintf(w, "   DNS Server       : %s\n", firstIP(msg.DNS()))
	fmt.Fprintf(w, "   Lease Time       : %s\n", humanLease(msg.IPAddressLeaseTime(0)))
	fmt.Fprintln(w)
	fmt.Fprintln(w, "DHCP handshake successful")
}

func firstIP(ips []net.IP) string {
	if len(ips) == 0 || ips[0] == nil {
		return "n/a"
	}
	return ips[0].String()
}

func maskString(mask net.IPMask) string {
	if len(mask) == 0 {
		return "n/a"
	}
	return net.IP(mask).String()
}

func humanLease(d time.Duration) string {
	if d <= 0 {
		return "n/a"
	}
	if d%time.Hour == 0 {
		h := int(d / time.Hour)
		if h == 1 {
			return "1 hour"
		}
		return fmt.Sprintf("%d hours", h)
	}
	if d%time.Minute == 0 {
		return fmt.Sprintf("%d minutes", int(d/time.Minute))
	}
	return d.String()
}
