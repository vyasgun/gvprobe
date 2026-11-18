package trace

import (
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/vyasgun/gvprobe/internal/out"
	"github.com/vyasgun/gvprobe/pkg/constants"
	"github.com/vyasgun/gvprobe/pkg/gvproxy"
	"github.com/vyasgun/gvprobe/pkg/gvproxy/datapath"
	"github.com/vyasgun/gvprobe/pkg/packet"
)

const (
	dnsDefaultTimeout = 3 * time.Second
	// dnsMaxAttempts retries a query once on timeout — UDP is lossy and
	// gvproxy may drop the first frame.
	dnsMaxAttempts = 2
)

type DnsTraceOpts struct {
	Name    string
	Type    string // query type (A, AAAA, MX, ...); defaults to A
	Server  string // resolver IPv4 (defaults to the gateway)
	MAC     string
	Count   int
	Timeout time.Duration
	Machine bool
	Quiet   bool
	Backend
}

// DnsQueryResult is one query round-trip. Rcode is always set when Err is
// nil — NXDOMAIN is a result, not an error.
type DnsQueryResult struct {
	Name    string
	Type    packet.DNSType
	Rcode   string
	Answers []string
	RTT     time.Duration
	Err     error
}

// guest is the identity one trace run needs: a DHCP lease plus the gateway's
// MAC to address frames to. Acquired fresh per invocation — the same
// one-shot model as the other trace commands.
type guest struct {
	ip         net.IP
	mac        net.HardwareAddr
	gatewayIP  net.IP
	gatewayMAC net.HardwareAddr
}

// dnsTypes is the closed set of query types the --type flag accepts.
var dnsTypes = map[string]packet.DNSType{
	"A": packet.DNSTypeA, "NS": packet.DNSTypeNS, "CNAME": packet.DNSTypeCNAME,
	"SOA": packet.DNSTypeSOA, "PTR": packet.DNSTypePTR, "MX": packet.DNSTypeMX,
	"TXT": packet.DNSTypeTXT, "AAAA": packet.DNSTypeAAAA, "SRV": packet.DNSTypeSRV,
}

func parseDNSType(s string) (packet.DNSType, error) {
	t, ok := dnsTypes[strings.ToUpper(s)]
	if !ok {
		names := make([]string, 0, len(dnsTypes))
		for name := range dnsTypes {
			names = append(names, name)
		}
		sort.Strings(names)
		return 0, fmt.Errorf("unsupported query type %q (supported: %s)", s, strings.Join(names, " "))
	}
	return t, nil
}

// TraceDns resolves a name through gvproxy's resolver and reports the
// round-trip.
func TraceDns(opts DnsTraceOpts) ([]DnsQueryResult, error) {
	status, err := gvproxy.Status()
	if err != nil {
		return nil, err
	}
	if status != gvproxy.Running {
		return nil, fmt.Errorf("gvproxy is not running")
	}
	if opts.Name == "" {
		return nil, fmt.Errorf("a DNS name is required")
	}
	qtype, err := parseDNSType(opts.Type)
	if err != nil {
		return nil, err
	}
	server := net.ParseIP(opts.Server)
	if server == nil || server.To4() == nil {
		return nil, fmt.Errorf("resolver must be an IPv4 address, got %q", opts.Server)
	}
	if opts.Count < 1 {
		opts.Count = 1
	}
	if opts.Timeout <= 0 {
		opts.Timeout = dnsDefaultTimeout
	}
	if opts.Backend == "" {
		opts.Backend = Backend(constants.ReadBackend())
	}

	suppressHuman := opts.Quiet && !opts.Machine
	w := io.Writer(os.Stdout)
	if suppressHuman {
		w = io.Discard
	}
	multi := opts.Count > 1
	if !opts.Machine && !suppressHuman {
		out.Fprintf(w, "DNS Trace — %s %s → %s:53\n\n", opts.Name, packet.DNSTypeString(qtype), server)
	}

	var results []DnsQueryResult
	err = datapath.WithSessionOrAttach(string(opts.Backend), func(s datapath.Session) error {
		g, err := acquireGuest(s, opts.MAC, w, opts.Machine)
		if err != nil {
			return err
		}
		// Frames always route via the gateway; the resolver may differ from it.
		if g.gatewayIP.String() != server.String() {
			g.gatewayIP = server.To4()
		}
		for run := 1; run <= opts.Count; run++ {
			res := dnsRoundTrip(s, g, opts, qtype, run, w)
			results = append(results, res)
			if !opts.Machine && !suppressHuman {
				if multi {
					writeDnsMultiRow(w, run, res)
				} else {
					writeDnsHumanReply(w, res)
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if !opts.Machine && !suppressHuman && multi {
		writeDnsSummary(w, results)
	}
	return results, nil
}

// dnsRoundTrip sends one query (retrying once on timeout) and returns the
// parsed reply. Only replies carrying our transaction ID count — anything
// else belongs to another exchange.
func dnsRoundTrip(s datapath.Session, g *guest, opts DnsTraceOpts, qtype packet.DNSType, run int, w io.Writer) DnsQueryResult {
	res := DnsQueryResult{Name: opts.Name, Type: qtype}

	id, query, err := packet.BuildDNSQuery(packet.DNSQuestion{Name: opts.Name, Type: qtype})
	if err != nil {
		res.Err = err
		return res
	}
	srcPort := uint16(20000 + rand.Intn(40000))
	frame, err := packet.BuildUDPFrame(packet.UDPEndpoints{
		SrcMAC: g.mac, DstMAC: g.gatewayMAC,
		SrcIP: g.ip, DstIP: g.gatewayIP,
		SrcPort: srcPort, DstPort: 53,
	}, query)
	if err != nil {
		res.Err = err
		return res
	}

	if opts.Machine {
		writeDnsEthernetTrace(w, fmt.Sprintf("outbound L2 frame (query, run %d)", run), frame)
	} else if opts.Count == 1 {
		out.Fprintf(w, "query      %s %s → %s:53, id %#04x\n", opts.Name, packet.DNSTypeString(qtype), g.gatewayIP, id)
	}

	start := time.Now()
	for attempt := 1; attempt <= dnsMaxAttempts; attempt++ {
		if err := s.SendFrame(frame); err != nil {
			res.Err = err
			return res
		}
		msg, replyFrame, err := readDnsReply(s, opts.Timeout, srcPort, id)
		if err == nil {
			res.RTT = time.Since(start)
			res.Rcode = packet.DNSRcodeString(uint16(msg.Rcode))
			for _, rr := range msg.Answers {
				res.Answers = append(res.Answers, renderRecord(rr))
			}
			if opts.Machine {
				writeDnsEthernetTrace(w, fmt.Sprintf("inbound L2 frame (reply, run %d)", run), replyFrame)
				writeDnsMachineReply(w, msg)
			}
			return res
		}
		if !isTimeout(err) || attempt == dnsMaxAttempts {
			res.Err = err
			return res
		}
		if !opts.Machine && opts.Count == 1 {
			out.Fprintf(w, "retry      no reply within %s, retrying\n", opts.Timeout)
		}
	}
	return res
}

// readDnsReply reads frames until one is a DNS response to our query:
// UDP from port 53 back to our source port, with our transaction ID set.
// Frames that don't match are skipped, not errors.
func readDnsReply(s datapath.Session, timeout time.Duration, srcPort uint16, id uint16) (*packet.DNSMessage, []byte, error) {
	deadline := time.Now().Add(timeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, nil, timeoutError(timeout)
		}
		frame, err := s.RecvFrame(remaining)
		if err != nil {
			return nil, nil, err
		}
		p, err := packet.ParseFrame(frame)
		if err != nil {
			continue
		}
		udp, ok := p.(packet.UdpFrame)
		if !ok || udp.UDP.SrcPort != 53 || udp.UDP.DstPort != layers.UDPPort(srcPort) {
			continue
		}
		msg, err := packet.ParseDNSMessage(udp.Payload())
		if err != nil || !msg.Response || msg.Id != id {
			continue
		}
		return msg, frame, nil
	}
}

// acquireGuest bootstraps the identity this run needs: DHCP lease, then ARP
// for the gateway's MAC. Frames other than the awaited reply are skipped.
func acquireGuest(s datapath.Session, macStr string, w io.Writer, machine bool) (*guest, error) {
	mac, err := net.ParseMAC(macStr)
	if err != nil {
		return nil, fmt.Errorf("invalid MAC %q: %w", macStr, err)
	}
	g := &guest{mac: mac, gatewayIP: net.ParseIP(constants.GatewayIP).To4()}

	// DHCP: an offer for our MAC carries the lease and usually the router.
	start := time.Now()
	discover, err := packet.BuildDhcpDiscover(mac)
	if err != nil {
		return nil, err
	}
	frame, err := packet.BuildDhcpDiscoverFrame(discover)
	if err != nil {
		return nil, err
	}
	if err := s.SendFrame(frame); err != nil {
		return nil, err
	}
	offer, err := readDhcpOffer(s, dnsDefaultTimeout)
	if err != nil {
		return nil, fmt.Errorf("no DHCP offer: %w", err)
	}
	g.ip = offer.YourIPAddr.To4()
	if routers := offer.Router(); len(routers) > 0 {
		g.gatewayIP = routers[0].To4()
	}
	dhcpRTT := time.Since(start)

	// ARP: the gateway's MAC is the responder's source hardware address.
	start = time.Now()
	arpFrame, err := packet.BuildArpRequest(mac, g.ip, g.gatewayIP)
	if err != nil {
		return nil, err
	}
	if err := s.SendFrame(arpFrame); err != nil {
		return nil, err
	}
	gatewayMAC, err := readArpReply(s, dnsDefaultTimeout)
	if err != nil {
		return nil, fmt.Errorf("no ARP reply from gateway: %w", err)
	}

	g.gatewayMAC = gatewayMAC
	arpRTT := time.Since(start)

	if !machine {
		out.Fprintf(w, "lease      %s acquired via DHCP (%s)\n", g.ip, dhcpRTT.Round(time.Millisecond))
		out.Fprintf(w, "gateway    %s is-at %s (%s)\n", g.gatewayIP, g.gatewayMAC, arpRTT.Round(time.Millisecond))
	}
	return g, nil
}

// readDhcpOffer reads until a parseable DHCP offer with an address arrives.
func readDhcpOffer(s datapath.Session, timeout time.Duration) (*dhcpv4.DHCPv4, error) {
	deadline := time.Now().Add(timeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, timeoutError(timeout)
		}
		frame, err := s.RecvFrame(remaining)
		if err != nil {
			return nil, err
		}
		msg, err := packet.ParseDhcpFromFrame(frame)
		if err == nil && len(msg.YourIPAddr.To4()) == 4 {
			return msg, nil
		}
	}
}

// readArpReply reads until an ARP reply arrives and returns the responder's MAC.
func readArpReply(s datapath.Session, timeout time.Duration) (net.HardwareAddr, error) {
	deadline := time.Now().Add(timeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, timeoutError(timeout)
		}
		frame, err := s.RecvFrame(remaining)
		if err != nil {
			return nil, err
		}
		p, err := packet.ParseFrame(frame)
		if err != nil {
			continue
		}
		if arp, ok := p.(packet.ArpFrame); ok && arp.ARP.Operation == layers.ARPReply {
			return net.HardwareAddr(arp.ARP.SourceHwAddress), nil
		}
	}
}

// renderRecord flattens one answer RR for display.
func renderRecord(r packet.DNSRecord) string {
	switch r.Type {
	case packet.DNSTypeTXT:
		return fmt.Sprintf("%q", strings.Join(r.TXT, " "))
	case packet.DNSTypeMX:
		return fmt.Sprintf("%d %s", r.Pref, r.Data)
	case packet.DNSTypeSRV:
		return fmt.Sprintf("%s:%d", r.Data, r.Port)
	default:
		return r.Data
	}
}

func writeDnsHumanReply(w io.Writer, res DnsQueryResult) {
	switch {
	case res.Err != nil:
		out.Fprintf(w, "reply      error: %v\n\n", res.Err)
	case len(res.Answers) == 0:
		out.Fprintf(w, "reply      %s, 0 answers in %s\n\n", res.Rcode, res.RTT.Round(time.Millisecond))
	default:
		out.Fprintf(w, "reply      %s, %d answer(s): %s in %s\n\n",
			res.Rcode, len(res.Answers), strings.Join(res.Answers, ", "), res.RTT.Round(time.Millisecond))
	}
}

func writeDnsMultiRow(w io.Writer, run int, res DnsQueryResult) {
	switch {
	case res.Err != nil:
		out.Fprintf(w, "%d  %s %s → error (%v)\n", run, res.Name, packet.DNSTypeString(res.Type), res.Err)
	case len(res.Answers) == 0:
		out.Fprintf(w, "%d  %s %s → %s (%s)\n", run, res.Name, packet.DNSTypeString(res.Type), res.Rcode, res.RTT.Round(time.Millisecond))
	default:
		out.Fprintf(w, "%d  %s %s → %s (%s)\n", run, res.Name, packet.DNSTypeString(res.Type), strings.Join(res.Answers, ", "), res.RTT.Round(time.Millisecond))
	}
}

func writeDnsSummary(w io.Writer, results []DnsQueryResult) {
	var rtts []time.Duration
	errors, replies := 0, 0
	for _, res := range results {
		if res.Err != nil {
			errors++
			continue
		}
		replies++
		rtts = append(rtts, res.RTT)
	}
	out.Fprintln(w)
	out.Fprintf(w, "Summary: %d sent, %d replies, %d error(s)\n", len(results), replies, errors)
	if len(rtts) > 0 {
		min, max := rtts[0], rtts[0]
		total := time.Duration(0)
		for _, r := range rtts {
			if r < min {
				min = r
			}
			if r > max {
				max = r
			}
			total += r
		}
		out.Fprintf(w, "rtt min/avg/max %s/%s/%s\n",
			min.Round(time.Millisecond), (total / time.Duration(len(rtts))).Round(time.Millisecond), max.Round(time.Millisecond))
	}
}

func writeDnsEthernetTrace(w io.Writer, title string, frame []byte) {
	out.Fprintf(w, "\n-- %s --\n", title)
	out.Fprintf(w, "%d byte(s)\n\n", len(frame))
	pkt := gopacket.NewPacket(frame, layers.LayerTypeEthernet, gopacket.Default)
	if errLayer := pkt.ErrorLayer(); errLayer != nil {
		out.Fprintf(w, "(layer decode error: %v)\n\n", errLayer.Error())
	}
	out.Fprintln(w, strings.TrimSpace(pkt.String()))
}

func writeDnsMachineReply(w io.Writer, msg *packet.DNSMessage) {
	out.Fprintln(w)
	out.Fprintf(w, "rcode=%s answers=%d id=%#04x\n", packet.DNSRcodeString(uint16(msg.Rcode)), len(msg.Answers), msg.Id)
	for _, rr := range msg.Answers {
		out.Fprintf(w, "  %s %s ttl=%d %s\n", rr.Name, packet.DNSTypeString(rr.Type), rr.TTL, renderRecord(rr))
	}
}

// isTimeout reports whether err is a read timeout.
func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

// timeoutError builds the sentinel-style timeout readDnsReply & co return.
func timeoutError(d time.Duration) error {
	return &netTimeoutError{d}
}

type netTimeoutError struct{ d time.Duration }

func (e *netTimeoutError) Error() string { return fmt.Sprintf("read timeout after %s", e.d) }
func (e *netTimeoutError) Timeout() bool { return true }
func (e *netTimeoutError) Temporary() bool { return true }
