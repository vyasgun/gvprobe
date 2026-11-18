// DNS query building and response parsing, wrapped around miekg/dns.
// gvprobe only ever transports DNS over its raw-UDP micro-stack (the
// library's client dials OS sockets, which is useless here), so this file
// exposes the small surface the probe and checks need: build a query,
// parse a response into plain structs.
package packet

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/miekg/dns"
)

// DNSType is a DNS query type. The constants below are the closed set of
// query types gvprobe supports; taking this type (rather than uint16)
// keeps callers from passing arbitrary values by accident.
type DNSType uint16

// Supported DNS query types.
const (
	DNSTypeA     DNSType = DNSType(dns.TypeA)
	DNSTypeNS    DNSType = DNSType(dns.TypeNS)
	DNSTypeCNAME DNSType = DNSType(dns.TypeCNAME)
	DNSTypeSOA   DNSType = DNSType(dns.TypeSOA)
	DNSTypePTR   DNSType = DNSType(dns.TypePTR)
	DNSTypeMX    DNSType = DNSType(dns.TypeMX)
	DNSTypeTXT   DNSType = DNSType(dns.TypeTXT)
	DNSTypeAAAA  DNSType = DNSType(dns.TypeAAAA)
	DNSTypeSRV   DNSType = DNSType(dns.TypeSRV)
)

// DNS RCODEs.
const (
	DNSRcodeSuccess  uint16 = dns.RcodeSuccess
	DNSRcodeNXDomain uint16 = dns.RcodeNameError
)

// DNSQuestion is a single query.
type DNSQuestion struct {
	Name string // dotted, e.g. "example.com."
	Type DNSType
}

// DNSRecord is one answer RR.
type DNSRecord struct {
	Name string
	Type DNSType
	TTL  uint32
	Data string // type-specific rendering (IP, target, TXT string, ...)
	TXT  []string
	Pref uint16 // MX preference
	Port uint16 // SRV port
}

// DNSMessage is a parsed response.
type DNSMessage struct {
	dns.MsgHdr // ID, Response, Opcode, Authoritative, Rcode
	Questions  []DNSQuestion
	Answers    []DNSRecord
	Authority  []DNSRecord
	Additional []DNSRecord
}

// BuildDNSQuery encodes a recursive query for name/type with a random ID.
// It returns the ID alongside the wire bytes so the caller can verify the
// response carries the same ID. A reply with a different ID belongs to
// another exchange and must not be treated as the answer.
func BuildDNSQuery(q DNSQuestion) (uint16, []byte, error) {
	if q.Name == "" {
		return 0, nil, fmt.Errorf("empty DNS name")
	}
	if len(q.Name) > 253 {
		return 0, nil, fmt.Errorf("DNS name too long: %d bytes", len(q.Name))
	}
	id := uint16(rand.Intn(1 << 16))
	m := &dns.Msg{
		MsgHdr: dns.MsgHdr{
			Id:               id,
			RecursionDesired: true,
		},
		Question: []dns.Question{{
			Name:   dns.Fqdn(q.Name),
			Qtype:  uint16(q.Type),
			Qclass: dns.ClassINET,
		}},
	}
	buf, err := m.Pack()
	if err != nil {
		return 0, nil, err
	}
	return id, buf, nil
}

// ParseDNSMessage decodes a response. It never panics on malformed input:
// the library returns errors for truncated or garbage messages.
func ParseDNSMessage(msg []byte) (*DNSMessage, error) {
	var m dns.Msg
	if err := m.Unpack(msg); err != nil {
		return nil, err
	}

	out := &DNSMessage{MsgHdr: m.MsgHdr}
	for _, q := range m.Question {
		out.Questions = append(out.Questions, DNSQuestion{Name: q.Name, Type: DNSType(q.Qtype)})
	}
	out.Answers = convertRRs(m.Answer)
	out.Authority = convertRRs(m.Ns)
	out.Additional = convertRRs(m.Extra)
	return out, nil
}

// convertRRs maps library resource records onto
// the flat DNSRecord view the checks use.
func convertRRs(rrs []dns.RR) []DNSRecord {
	var out []DNSRecord
	for _, rr := range rrs {
		rec := DNSRecord{
			Name: rr.Header().Name,
			Type: DNSType(rr.Header().Rrtype),
			TTL:  rr.Header().Ttl,
		}
		switch v := rr.(type) {
		case *dns.A:
			rec.Data = v.A.String()
		case *dns.AAAA:
			rec.Data = v.AAAA.String()
		case *dns.MX:
			rec.Pref = v.Preference
			rec.Data = v.Mx
		case *dns.SRV:
			rec.Port = v.Port
			rec.Data = v.Target
		case *dns.TXT:
			rec.TXT = v.Txt
			rec.Data = strings.Join(v.Txt, "")
		case *dns.PTR:
			rec.Data = v.Ptr
		case *dns.CNAME:
			rec.Data = v.Target
		case *dns.NS:
			rec.Data = v.Ns
		case *dns.SOA:
			rec.Data = fmt.Sprintf("%s %s serial=%d refresh=%d retry=%d expire=%d minttl=%d",
				v.Ns, v.Mbox, v.Serial, v.Refresh, v.Retry, v.Expire, v.Minttl)
		default:
			rec.Data = rr.String()
		}
		out = append(out, rec)
	}
	return out
}

// DNSTypeString renders a query type for output.
func DNSTypeString(t DNSType) string {
	return dns.Type(uint16(t)).String()
}

// DNSRcodeString renders a response code for output.
func DNSRcodeString(r uint16) string {
	if s, ok := dns.RcodeToString[int(r)]; ok {
		return s
	}
	return fmt.Sprintf("RCODE%d", r)
}
