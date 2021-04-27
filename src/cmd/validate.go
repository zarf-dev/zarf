package cmd

import (
	"shift/internal/utils"

	"github.com/spf13/cobra"
)

// validateCmd represents the initialize command
var validateCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Remove the k3s installation",
	Run: func(cmd *cobra.Command, args []string) {
		utils.ExecCommand([]string{}, "/usr/local/bin/k3s-uninstall.sh")
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
}
