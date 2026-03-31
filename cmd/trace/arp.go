package trace

import (
	"log"

	"github.com/spf13/cobra"
	"github.com/vyasgun/gvprobe/pkg/constants"
	"github.com/vyasgun/gvprobe/pkg/trace"
)

func NewArpCommand() *cobra.Command {
	var opts trace.ArpTraceOpts
	cmd := &cobra.Command{
		Use:   "arp",
		Short: "Trace ARP via vfkit",
		Long: `Sends one ARP request through gvproxy's vfkit unixgram path.	
Default output is human readable; use --machine for raw packet decode.

Example:
  gvprobe trace arp --target 192.168.127.1 --sender 192.168.127.2 --mac 5a:94:ef:e4:0c:ee
`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := trace.TraceArp(opts); err != nil {
				log.Fatalf("failed to trace ARP: %v", err)
			}
		},
	}
	cmd.Flags().StringVar(&opts.Target, "target", constants.GatewayIP, "IPv4 to resolve (who-has)")
	cmd.Flags().StringVar(&opts.SenderIP, "sender", constants.GuestIP, "ARP requester IPv4 (SPA in request)")
	cmd.Flags().StringVar(&opts.MAC, "mac", constants.GuestMAC, "source MAC (requester)")
	cmd.Flags().BoolVar(&opts.Machine, "machine", false, "print layer-by-layer packet decode output")
	return cmd
}
