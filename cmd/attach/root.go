package attach

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/vyasgun/gvprobe/pkg/attach"
	"github.com/vyasgun/gvprobe/pkg/constants"
)

func NewAttachCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "attach",
		Short: "Hold a persistent datapath session (simulates a VM NIC)",
		Long: `Attach opens a datapath connection to gvproxy and keeps it alive,
simulating a VM staying plugged into the virtual switch.

The backend is read from ~/.gvprobe/backend (written by 'gvprobe start').

While attached, trace commands route frames through this session
instead of opening their own connection to gvproxy.`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			backend := constants.ReadBackend()
			if err := attach.Run(ctx, backend); err != nil {
				log.Fatalf("attach: %v", err)
			}
		},
	}
}
