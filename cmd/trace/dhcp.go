package trace

import (
	"log"

	"github.com/spf13/cobra"
	"github.com/vyasgun/gvprobe/pkg/trace"
)

func NewDhcpCommand() *cobra.Command {
	var machine bool
	cmd := &cobra.Command{
		Use:   "dhcp",
		Short: "Trace DHCP via vfkit (human output by default)",
		Long: `Sends a DHCP Discover through gvproxy's vfkit unixgram path.
Default output is human readable; use --machine for layer-by-layer packet decode.`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := trace.TraceDhcp(machine); err != nil {
				log.Fatalf("failed to trace DHCP traffic: %v", err)
			}
		},
	}
	cmd.Flags().BoolVar(&machine, "machine", false, "print layer-by-layer packet decode output")
	return cmd
}
