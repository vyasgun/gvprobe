package cmd

import (
	"errors"
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
				if errors.Is(err, gvproxy.ErrGvproxyAlreadyRunning) {
					log.Printf("gvproxy is already running at pid %d", pid)
					return
				}
				log.Fatalf("failed to start gvproxy: %v", err)
			}
			log.Printf("gvproxy started with pid: %d", pid)
		},
	}
}
