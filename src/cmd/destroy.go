package cmd

import (
	"github.com/manifoldco/promptui"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/shift/cli/src/internal/utils"

	"github.com/spf13/cobra"
)

// destroyCmd represents the initialize command
var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Remove the k3s installation",
	Run: func(cmd *cobra.Command, args []string) {
		prompt := promptui.Prompt{
			Label:     "Are you sure you want to destroy this deployment?",
			IsConfirm: true,
		}

		result, err := prompt.Run()

		if err != nil && result == "y" {
			utils.ExecCommand([]string{}, "/usr/local/bin/k3s-uninstall.sh")
		}
	},
}

func init() {
	rootCmd.AddCommand(destroyCmd)
}
