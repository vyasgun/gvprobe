package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	internal "github.com/vyasgun/gvprobe/internal/version"
)

func NewVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Version: %s Commit: %s\n", internal.Version, internal.Commit)
		},
	}
}
