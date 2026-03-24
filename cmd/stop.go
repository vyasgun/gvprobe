package cmd

import (
	"log"

	"github.com/spf13/cobra"
	"github.com/vyasgun/gvprobe/pkg/gvproxy"
)

func NewStopCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop gvproxy service",
		Run: func(cmd *cobra.Command, args []string) {
			if err := gvproxy.Stop(); err != nil {
				log.Fatalf("failed to stop gvproxy: %v", err)
			}
			log.Printf("gvproxy stopped")
		},
	}
}
