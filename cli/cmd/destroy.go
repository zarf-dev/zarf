package cmd

import (
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/k3s"

	"github.com/spf13/cobra"
)

var confirmDestroy bool

var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Tear it all down, we'll miss you Zarf...",
	Run: func(cmd *cobra.Command, args []string) {
		k3s.RemoveAll()
	},
}

func init() {
	rootCmd.AddCommand(destroyCmd)

	destroyCmd.Flags().BoolVar(&confirmDestroy, "confirm", false, "Confirm the destroy action")
	_ = destroyCmd.MarkFlagRequired("confirm")
}
