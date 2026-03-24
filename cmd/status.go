package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/vyasgun/gvprobe/pkg/gvproxy"
)

func NewStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show gvproxy status",
		RunE: func(cmd *cobra.Command, args []string) error {
			status, err := gvproxy.Status()
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "gvproxy: %s\n", status)
			return err
		},
	}
}
