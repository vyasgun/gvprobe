package cmd

import (
	"errors"
	"log"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vyasgun/gvprobe/pkg/constants"
	"github.com/vyasgun/gvprobe/pkg/gvproxy"
	"github.com/vyasgun/gvprobe/pkg/gvproxy/datapath"
	"github.com/vyasgun/gvprobe/pkg/trace"
)

func NewStartCommand() *cobra.Command {
	var backend string
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start gvproxy service",
		Run: func(cmd *cobra.Command, args []string) {
			if !slices.Contains(datapath.SupportedBackends, backend) {
				log.Fatalf("backend must be one of: %s", strings.Join(datapath.SupportedBackends, ", "))
			}
			pid, err := gvproxy.Start(backend)
			if err != nil {
				if errors.Is(err, gvproxy.ErrGvproxyAlreadyRunning) {
					log.Printf("gvproxy is already running at pid %d", pid)
					return
				}
				log.Fatalf("failed to start gvproxy: %v", err)
			}
			if err := constants.WriteBackend(backend); err != nil {
				log.Printf("warning: could not save backend: %v", err)
			}
			log.Printf("gvproxy started with pid %d (backend: %s)", pid, backend)
		},
	}
	cmd.Flags().StringVar(&backend, "backend", string(trace.BackendVfkit), "gvproxy datapath backend (vfkit, bess, qemu)")
	return cmd
}
