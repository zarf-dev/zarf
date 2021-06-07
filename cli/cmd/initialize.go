package cmd

import (
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/k3s"

	"github.com/spf13/cobra"
)

var confirmInit bool

// initializeCmd represents the initialize command
var initializeCmd = &cobra.Command{
	Use:   "initialize",
	Short: "Deploys the utility cluster on a clean linux box",
	Long:  ` `,
	Run: func(cmd *cobra.Command, args []string) {
		k3s.Install()
	},
}

func init() {
	rootCmd.AddCommand(initializeCmd)
	initializeCmd.Flags().BoolVar(&confirmInit, "confirm", false, "Confirm the install action")
	initializeCmd.MarkFlagRequired("confirm")
}
