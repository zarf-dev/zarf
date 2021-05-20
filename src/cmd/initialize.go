package cmd

import (
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/shift/cli/src/internal/k3s"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/shift/cli/src/internal/utils"

	"github.com/manifoldco/promptui"
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

		prompt := promptui.Prompt{
			Label:     "Preflight check passed, continue deployment?",
			IsConfirm: true,
		}

		result, err := prompt.Run()

		if !isDryRun || err != nil && result == "y" {
			utils.PlaceAsset("bin/k3s", "/usr/local/bin/k3s")
			utils.PlaceAsset("bin/init-k3s.sh", "/usr/local/bin/init-k3s.sh")
			utils.PlaceAsset("charts", "/var/lib/rancher/k3s/server/static/charts")
			utils.PlaceAsset("manifests", "/var/lib/rancher/k3s/server/manifests")
			utils.PlaceAsset("images", "/var/lib/rancher/k3s/agent/images")

			k3s.Install()
		}
	},
}

func init() {
	rootCmd.AddCommand(initializeCmd)
	initializeCmd.Flags().BoolVar(&isDryRun, "dryrun", true, "Only run checksum and preflight steps")
}
