package trace

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/vyasgun/gvprobe/pkg/constants"
	"github.com/vyasgun/gvprobe/pkg/gvproxy"
)

var vfkitRemote = &net.UnixAddr{Name: constants.VfkitSocket, Net: "unixgram"}

const dhcpReadTimeout = 3 * time.Second

type DhcpTraceOpts struct {
	MAC     string
	Count   int
	Machine bool
}

// TraceDhcp is the main entry point.
func TraceDhcp(opts DhcpTraceOpts) error {
	status, err := gvproxy.Status()
	if err != nil {
		return err
	}
	if status != gvproxy.Running {
		return fmt.Errorf("gvproxy is not running")
	}

	if opts.Count < 1 {
		opts.Count = 1
	}

	w := os.Stdout
	if !opts.Machine {
		fmt.Fprintf(w, "DHCP Trace — MAC: %s (%d requests)\n\n", opts.MAC, opts.Count)
	}

	var nReused, nNew, nDone int

	err = gvproxy.WithVfkitLocal(func(c *net.UnixConn) error {
		for run := 1; run <= opts.Count; run++ {
			dhcpMsg, err := buildDhcpDiscover(opts.MAC)
			if err != nil {
				return fmt.Errorf("failed to build DHCP discover: %w", err)
			}
			frame, err := buildEthernetFrame(dhcpMsg)
			if err != nil {
				return fmt.Errorf("failed to build Ethernet frame: %w", err)
			}

			reused, completed, err := dhcpDiscoverRoundTrip(c, frame, dhcpReadTimeout, run, opts, w)
			if err != nil {
				return err
			}
			if !opts.Machine && completed {
				nDone++
				if reused {
					nReused++
				} else {
					nNew++
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	if !opts.Machine && opts.Count > 1 {
		writeFinalSummary(w, opts.Count, nReused, nNew, nDone)
	}
	return nil
}

// dhcpDiscoverRoundTrip sends one frame and reads the Offer (or times out).
// reusedBefore is whether a lease already existed for this MAC before the discover.
// completed is true when a DHCP reply was decoded successfully.
func dhcpDiscoverRoundTrip(c *net.UnixConn, frame []byte, readTimeout time.Duration, run int, opts DhcpTraceOpts, w io.Writer) (reusedBefore bool, completed bool, err error) {
	foundLease, leaseIP, err := leaseForMAC(opts.MAC)
	if err != nil {
		return false, false, err
	}

	if opts.Machine {
		writeDhcpEthernetTrace(w, fmt.Sprintf("outbound L2 frame (run %d)", run), frame)
	} else if opts.Count == 1 {
		writeHumanDiscover(w, opts.MAC)
	}

	if _, err := c.WriteTo(frame, vfkitRemote); err != nil {
		return false, false, err
	}

	_ = c.SetReadDeadline(time.Now().Add(readTimeout))
	reply := make([]byte, 65536)
	n, _, err := c.ReadFrom(reply)
	if err != nil {
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			if opts.Machine {
				fmt.Fprintf(w, "\n── inbound (no reply) ──\nno DHCP response within %s\n", readTimeout)
			} else if opts.Count > 1 {
				fmt.Fprintln(w, "(no reply within timeout)")
			} else {
				fmt.Fprintf(w, "\nNo DHCP reply within %s\n", readTimeout)
			}
			return false, false, nil
		}
		return false, false, err
	}

	replyMsg, err := parseDhcpFromEthernet(reply[:n])
	if err != nil {
		if opts.Machine {
			writeDhcpEthernetTrace(w, fmt.Sprintf("inbound L2 frame (run %d)", run), reply[:n])
			return false, true, nil
		}
		return false, false, fmt.Errorf("decode DHCP reply: %w", err)
	}

	if opts.Machine {
		writeDhcpEthernetTrace(w, fmt.Sprintf("inbound L2 frame (run %d)", run), reply[:n])
		return false, true, nil
	}

	multi := opts.Count > 1
	if !multi && foundLease && leaseIP != replyMsg.YourIPAddr.String() {
		return false, false, fmt.Errorf("lease IP mismatch: %s != %s", leaseIP, replyMsg.YourIPAddr.String())
	}

	if multi {
		tag := "New"
		if foundLease {
			tag = "Reused"
		}
		fmt.Fprintf(w, "[%d] Sent Discover → Offer %s   [%s]\n", run, replyMsg.YourIPAddr, tag)
		return foundLease, true, nil
	}

	writeHumanOffer(w, replyMsg, foundLease, leaseIP)
	return foundLease, true, nil
}

func leaseForMAC(mac string) (found bool, leaseIP string, err error) {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(constants.LeasesURL)
	if err != nil {
		return false, "", fmt.Errorf("failed to get leases: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, "", fmt.Errorf("failed to read leases: %w", err)
	}
	var leases map[string]string
	if err := json.Unmarshal(body, &leases); err != nil {
		return false, "", fmt.Errorf("failed to unmarshal leases: %w", err)
	}
	for ip, m := range leases {
		if strings.EqualFold(m, mac) {
			return true, ip, nil
		}
	}
	return false, "", nil
}

// writeFinalSummary closes human multi-run output (one informative line).
func writeFinalSummary(w io.Writer, totalReq, nReused, nNew, nDone int) {
	fmt.Fprintln(w)
	if nDone == 0 {
		fmt.Fprintf(w, "Summary: No DHCP replies received (%d request(s))\n", totalReq)
		return
	}
	switch {
	case nReused == nDone && nNew == 0:
		fmt.Fprintf(w, "Summary: Lease consistently reused (%d/%d times)\n", nReused, nDone)
	case nNew == nDone && nReused == 0:
		fmt.Fprintf(w, "Summary: New lease each time (%d/%d times)\n", nNew, nDone)
	default:
		fmt.Fprintf(w, "Summary: Mixed — reused %d, new %d (of %d replies)\n", nReused, nNew, nDone)
	}
	if nDone < totalReq {
		fmt.Fprintf(w, "Note: %d request(s) had no reply or timed out\n", totalReq-nDone)
	}
}

func buildDhcpDiscover(mac string) (*dhcpv4.DHCPv4, error) {
	hw, err := net.ParseMAC(mac)
	if err != nil {
		return nil, fmt.Errorf("invalid MAC %q: %w", mac, err)
	}
	return dhcpv4.New(
		dhcpv4.WithClientIP(net.IPv4(0, 0, 0, 0).To4()),
		dhcpv4.WithMessageType(dhcpv4.MessageTypeDiscover),
		dhcpv4.WithHwAddr(hw),
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
		SrcIP:    net.IPv4(0, 0, 0, 0).To4(),
		DstIP:    net.IPv4(255, 255, 255, 255).To4(),
	}

	udp := &layers.UDP{
		SrcPort: layers.UDPPort(dhcpv4.ClientPort),
		DstPort: layers.UDPPort(dhcpv4.ServerPort),
	}

	if err := udp.SetNetworkLayerForChecksum(ip4); err != nil {
		return nil, fmt.Errorf("failed to set network layer for checksum: %w", err)
	}
	eth := &layers.Ethernet{
		SrcMAC:       dhcpMsg.ClientHWAddr,
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

func writeHumanDiscover(w io.Writer, mac string) {
	fmt.Fprintln(w, "-> Sent DHCP Discover")
	fmt.Fprintf(w, "   Client MAC       : %s\n", mac)
	fmt.Fprintln(w, "   Asking for IP")
}

func writeHumanOffer(w io.Writer, msg *dhcpv4.DHCPv4, hadLease bool, priorLeaseIP string) {
	fmt.Fprintln(w)
	fmt.Fprintf(w, "-> Received DHCP %s from gvproxy\n", msg.MessageType())
	fmt.Fprintf(w, "   Offered IP       : %s\n", msg.YourIPAddr)
	fmt.Fprintf(w, "   Subnet Mask      : %s\n", maskString(msg.SubnetMask()))
	fmt.Fprintf(w, "   Gateway          : %s\n", firstIP(msg.Router()))
	fmt.Fprintf(w, "   DNS Server       : %s\n", firstIP(msg.DNS()))
	fmt.Fprintf(w, "   Lease Time       : %s\n", humanLease(msg.IPAddressLeaseTime(0)))
	fmt.Fprintf(w, "   Transaction ID   : %s\n", msg.TransactionID.String())
	if hadLease {
		fmt.Fprintf(w, "   Lease state      : Reusing existing lease (%s)\n", priorLeaseIP)
	} else {
		fmt.Fprintf(w, "   Lease state      : New lease (offered %s)\n", msg.YourIPAddr)
	}
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
