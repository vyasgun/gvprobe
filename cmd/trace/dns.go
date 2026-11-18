package trace

import (
	"log"
	"time"

	"github.com/spf13/cobra"
	"github.com/vyasgun/gvprobe/pkg/constants"
	"github.com/vyasgun/gvprobe/pkg/trace"
)

func NewDnsCommand() *cobra.Command {
	var opts trace.DnsTraceOpts
	cmd := &cobra.Command{
		Use:   "dns <name>",
		Short: "send a DNS query through gvproxy",
		Long: `Resolves a name through gvproxy's DNS resolver (the gateway by default).

Bootstraps a guest identity first (DHCP lease, ARP for the gateway), then
sends the query as a raw UDP frame and waits for the matching reply which is
matched by UDP ports and DNS ID. NXDOMAIN is reported as a
result, not an error with one retry on timeout to account for UDP loss.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			opts.Name = args[0]
			if _, err := trace.TraceDns(opts); err != nil {
				log.Fatalf("failed to trace DNS: %v", err)
			}
		},
	}
	cmd.Flags().StringVar(&opts.Type, "type", "A", "query type (A AAAA CNAME MX NS PTR SOA SRV TXT)")
	cmd.Flags().StringVar(&opts.Server, "server", constants.GatewayIP, "resolver IPv4")
	cmd.Flags().StringVar(&opts.MAC, "mac", constants.GuestMAC, "source MAC (guest)")
	cmd.Flags().IntVar(&opts.Count, "count", 1, "number of queries to send")
	cmd.Flags().DurationVar(&opts.Timeout, "timeout", 3*time.Second, "per-query read timeout")
	cmd.Flags().BoolVar(&opts.Machine, "machine", false, "print layer-by-layer packet decode output")
	cmd.Flags().BoolVar(&opts.Quiet, "quiet", false, "suppress human-oriented output")
	return cmd
}
