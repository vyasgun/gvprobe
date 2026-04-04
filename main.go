package main

import (
	"log"
	"os"

	"github.com/vyasgun/gvprobe/cmd"
	"github.com/vyasgun/gvprobe/cmd/agent"
	"github.com/vyasgun/gvprobe/cmd/trace"
	"github.com/vyasgun/gvprobe/pkg/constants"
)

var rootCmd = cmd.NewRootCommand()

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	if _, err := os.Stat(constants.GvprobeConfigDir); os.IsNotExist(err) {
		if err := os.MkdirAll(constants.GvprobeConfigDir, 0755); err != nil {
			log.Fatalf("failed to create config directory: %v", err)
		}
	}
	rootCmd.AddCommand(cmd.NewVersionCommand())
	rootCmd.AddCommand(cmd.NewStartCommand())
	rootCmd.AddCommand(cmd.NewStopCommand())
	rootCmd.AddCommand(cmd.NewStatusCommand())
	rootCmd.AddCommand(trace.NewRootCommand())
	rootCmd.AddCommand(agent.NewRootCommand())
}
