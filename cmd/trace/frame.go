package trace

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/vyasgun/gvprobe/internal/out"
	"github.com/vyasgun/gvprobe/pkg/constants"
	"github.com/vyasgun/gvprobe/pkg/gvproxy"
	"github.com/vyasgun/gvprobe/pkg/gvproxy/datapath"
)

// NewFrameCommand builds `gvprobe trace frame`: send a raw frame of the
// given size directly into the datapath.
func NewFrameCommand() *cobra.Command {
	var (
		size    int
		listen  bool
		timeout time.Duration
	)

	cmd := &cobra.Command{
		Use:   "frame",
		Short: "Send a raw frame into the datapath",
		Long: `Sends a raw Ethernet frame of the given size into gvproxy's datapath.

With --listen, waits afterwards for any inbound frame and decodes it
which is useful to check whether the switch is still forwarding.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			status, err := gvproxy.Status()
			if err != nil {
				return err
			}
			if status != gvproxy.Running {
				return fmt.Errorf("gvproxy is not running")
			}
			if size < 0 {
				return fmt.Errorf("size must be >= 0")
			}

			frame := make([]byte, size)
			// Fill with a recognizable pattern (guest MAC + zeros) so the
			// frame is not all-zero garbage in packet captures.
			mac := strings.Split(constants.GuestMAC, ":")
			for i := 0; i < size && i < 6; i++ {
				var b byte
				fmt.Sscanf(mac[i], "%02x", &b)
				frame[i] = b
			}

			return datapath.WithSessionOrAttach(constants.ReadBackend(), func(s datapath.Session) error {
				out.Fprintf(cmd.OutOrStdout(), "Sending %d-byte frame (backend %s)\n", size, constants.ReadBackend())
				if err := s.SendFrame(frame); err != nil {
					return fmt.Errorf("failed to send frame: %w", err)
				}
				if !listen {
					return nil
				}
				out.Fprintf(cmd.OutOrStdout(), "Listening for a reply (timeout %s)...\n", timeout)
				reply, err := s.RecvFrame(timeout)
				if err != nil {
					if ne, ok := err.(interface{ Timeout() bool }); ok && ne.Timeout() {
						out.Fprintln(cmd.OutOrStdout(), "No reply (switch forwarding may be wedged — cf. gvisor-tap-vsock#780)")
						return nil
					}
					return fmt.Errorf("failed to read reply: %w", err)
				}
				out.Fprintf(cmd.OutOrStdout(), "Received %d-byte frame in reply\n", len(reply))
				return nil
			})
		},
	}

	cmd.Flags().IntVar(&size, "size", 14, "frame size in bytes (sizes <14 are shorter than an Ethernet header)")
	cmd.Flags().BoolVar(&listen, "listen", false, "wait for an inbound frame after sending")
	cmd.Flags().DurationVar(&timeout, "timeout", 3*time.Second, "how long to listen for a reply")

	return cmd
}
