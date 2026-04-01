package cmd

import (
	"log"

	"github.com/spf13/cobra"
	"github.com/vyasgun/gvprobe/internal/out"
	"github.com/vyasgun/gvprobe/pkg/gvproxy"
)

func NewStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show gvproxy status",
		Run: func(cmd *cobra.Command, args []string) {
			status, err := gvproxy.Status()
			if err != nil {
				log.Fatalf("failed to get gvproxy status: %v", err)
			}
			out.Fprintf(cmd.OutOrStdout(), "gvproxy: %s\n", status)
		},
	}
}
