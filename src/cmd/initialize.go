package cmd

import (
	"shift/internal/k3s"
	"shift/internal/utils"

	"github.com/spf13/cobra"
)

var isDryRun bool

// initializeCmd represents the initialize command
var initializeCmd = &cobra.Command{
	Use:   "initialize",
	Short: "Deploys the utility cluster on a clean linux box",
	Long:  ` `,
	Run: func(cmd *cobra.Command, args []string) {

		utils.RunTarballChecksumValidate()
		utils.RunPreflightChecks()

		if !isDryRun {
			utils.WriteAssets("assets/bin", "/usr/local/bin")
			utils.WriteAssets("assets/charts", "/var/lib/rancher/k3s/server/static/charts")
			utils.WriteAssets("assets/manifests", "/var/lib/rancher/k3s/server/manifests")

			k3s.Install()
		}
	},
}

func init() {
	rootCmd.AddCommand(initializeCmd)
	initializeCmd.Flags().BoolVar(&isDryRun, "dryrun", false, "Only run checksum and preflight steps")
}
