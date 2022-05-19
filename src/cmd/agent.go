package cmd

import (
	"github.com/defenseunicorns/zarf/src/internal/agent"
	"github.com/spf13/cobra"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Runs the zarf agent",
	// this command should not be advertised on the cli as it has no value outside the k8s env
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {
		agent.StartWebhook()
	},
}

func init() {
	rootCmd.AddCommand(agentCmd)
}
