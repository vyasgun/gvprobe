package agent

import "github.com/spf13/cobra"

func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "agent",
		Short: "AI agent for gvprobe",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	rootCmd.AddCommand(NewAgentDhcpCommand())
	return rootCmd
}
