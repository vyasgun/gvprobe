package main

import (
	"os"

	"github.com/vyasgun/gvprobe/cmd"
)

var rootCmd = cmd.NewRootCommand()

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(cmd.NewVersionCommand())
}
