package cmd

import (
	"github.com/spf13/cobra"
)

func NewRootCommand() *cobra.Command {
	var rootCmd = &cobra.Command{
		Use:   "gvprobe",
		Short: "gvisor-tap-vsock debug tool",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	return rootCmd
}
