package main

import (
	"os"

	"github.com/vyasgun/gvprobe/cmd"
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
		os.MkdirAll(constants.GvprobeConfigDir, 0755)
	}
	rootCmd.AddCommand(cmd.NewVersionCommand())
	rootCmd.AddCommand(cmd.NewStartCommand())
	rootCmd.AddCommand(cmd.NewStopCommand())
	rootCmd.AddCommand(cmd.NewStatusCommand())
}
