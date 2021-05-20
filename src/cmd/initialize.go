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

		if isDryRun {
			prompt := promptui.Prompt{
				Label:     "Preflight check passed, continue deployment?",
				IsConfirm: true,
			}

			result, err := prompt.Run()

			if err != nil && result == "y" {
				k3s.Install()
			}
		} else {
			k3s.Install()
		}
	},
}

func init() {
	rootCmd.AddCommand(initializeCmd)
	initializeCmd.Flags().BoolVar(&isDryRun, "dryrun", true, "Only run checksum and preflight steps")
}
