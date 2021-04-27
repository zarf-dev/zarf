package cmd

import (
	"shift/internal/utils"

	"github.com/spf13/cobra"
)

// destroyCmd represents the initialize command
var destroyCmd = &cobra.Command{
	Use:   "validate",
	Short: "Quick tarball validation",
	Run: func(cmd *cobra.Command, args []string) {
		utils.RunTarballChecksumValidate()
	},
}

func init() {
	rootCmd.AddCommand(destroyCmd)
}
