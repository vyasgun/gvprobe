package trace

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"
	"github.com/vyasgun/gvprobe/pkg/constants"
	"github.com/vyasgun/gvprobe/pkg/trace"
)

func NewDhcpCommand() *cobra.Command {
	var opts trace.DhcpTraceOpts

	cmd := &cobra.Command{
		Use:   "dhcp",
		Short: "Trace DHCP via vfkit (human output by default)",
		Long: `Sends one or more DHCP Discover packets through gvproxy's vfkit unixgram path.
Default output is human readable; use --machine for raw packet decode.
Use --quiet for one line per request without the full trace narrative.`,
		Run: func(cmd *cobra.Command, args []string) {
			if opts.Count < 1 {
				log.Fatalf("count must be at least 1")
			}
			rows, err := trace.TraceDhcp(opts)
			if err != nil {
				log.Fatalf("failed to trace DHCP traffic: %v", err)
			}
			if opts.Quiet && !opts.Machine {
				for i, r := range rows {
					fmt.Printf("[%d] MAC=%s → Offered IP=%s   [%s]\n", i+1, r.MAC, r.QuietOfferedIP(), r.QuietLeaseTag())
				}
			}
		},
	}

	cmd.Flags().BoolVar(&opts.Machine, "machine", false, "print layer-by-layer packet decode output")
	cmd.Flags().BoolVarP(&opts.Quiet, "quiet", "q", false, "suppress full trace; print one compact line per request (ignored with --machine)")
	cmd.Flags().StringVar(&opts.MAC, "mac", constants.GuestMAC, "override guest MAC address (default: 5a:94:ef:e4:0c:ee)")
	cmd.Flags().IntVar(&opts.Count, "count", 1, "number of DHCP requests to send")

	return cmd
}
