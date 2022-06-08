package cmd

import (
	"github.com/defenseunicorns/zarf/src/internal/agent"
	"github.com/spf13/cobra"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Runs the zarf agent",
	Long: "NOTE: This command is a hidden command and generally shouldn't be run by a human.\n" +
		"This command starts up a http webhook that Zarf deployments use to mutate pods to conform " +
		"with the Zarf container registry and Gitea server URLs.",
	// this command should not be advertised on the cli as it has no value outside the k8s env
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {
		agent.StartWebhook()
	},
}

func init() {
	rootCmd.AddCommand(agentCmd)
}
