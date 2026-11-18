package packet

import (
	"fmt"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// ICMPID identifies an ICMP echo request/reply pair.
type ICMPID struct {
	ID  uint16
	Seq uint16
}

// BuildEchoRequest builds Eth > IPv4 > ICMP echo request with the given payload
// size (data is a repeating byte pattern so replies can be verified intact).
func BuildEchoRequest(f ICMPEndpoints, id ICMPID, size int, df bool) ([]byte, error) {
	if size < 0 {
		return nil, fmt.Errorf("negative ICMP payload size %d", size)
	}
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i % 251)
	}
	icmp := &layers.ICMPv4{
		TypeCode: layers.CreateICMPv4TypeCode(layers.ICMPv4TypeEchoRequest, 0),
		Id:       id.ID,
		Seq:      id.Seq,
	}
	frame, err := buildIPv4Ethernet(f, layers.IPProtocolICMPv4, df, icmp, gopacket.Payload(data))
	if err != nil {
		return nil, err
	}
	return frame, nil
}

// MatchEchoReply reports whether f is an ICMP echo reply for the given
// id/seq.
func MatchEchoReply(f IcmpFrame, id ICMPID) bool {
	return f.ICMP.TypeCode.Type() == layers.ICMPv4TypeEchoReply &&
		f.ICMP.Id == id.ID &&
		f.ICMP.Seq == id.Seq
}
