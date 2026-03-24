package cmd

import (
	"log"

	"github.com/spf13/cobra"
	"github.com/vyasgun/gvprobe/pkg/gvproxy"
)

func NewStartCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start gvproxy service",
		Run: func(cmd *cobra.Command, args []string) {
			var err error
			var pid int
			if pid, err = gvproxy.Start(); err != nil {
				log.Fatalf("failed to start gvproxy: %v", err)
			}
			log.Printf("gvproxy started with pid: %d", pid)
		},
	}
}
