package trace

import (
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
Default output is human readable; use --machine for raw packet decode.`,
		Run: func(cmd *cobra.Command, args []string) {
			if opts.Count < 1 {
				log.Fatalf("count must be at least 1")
			}
			if err := trace.TraceDhcp(opts); err != nil {
				log.Fatalf("failed to trace DHCP traffic: %v", err)
			}
		},
	}

	cmd.Flags().BoolVar(&opts.Machine, "machine", false, "print layer-by-layer packet decode output")
	cmd.Flags().StringVar(&opts.MAC, "mac", constants.GuestMAC, "override guest MAC address (default: 5a:94:ef:e4:0c:ee)")
	cmd.Flags().IntVar(&opts.Count, "count", 1, "number of DHCP requests to send")

	return cmd
}
