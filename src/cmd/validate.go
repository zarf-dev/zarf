package cmd

import (
	"shift/internal/utils"

	"github.com/spf13/cobra"
)

// validateCmd represents the initialize command
var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Quick tarball validation",
	Run: func(cmd *cobra.Command, args []string) {
		utils.RunTarballChecksumValidate()
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
}
