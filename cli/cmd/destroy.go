package cmd

import (
	"fmt"

	"github.com/defenseunicorns/zarf/cli/internal/utils"

	"github.com/spf13/cobra"
)

var confirmDestroy bool

var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Tear it all down, we'll miss you Zarf...",
	Run: func(cmd *cobra.Command, args []string) {
		burn()
		_, _ = utils.ExecCommand(true, nil, "/usr/local/bin/k3s-remove.sh")
		burn()
	},
}

func burn() {
	fmt.Println("")
	for count := 0; count < 40; count++ {
		fmt.Print("🔥")
	}
	fmt.Println("")
}

func init() {
	rootCmd.AddCommand(destroyCmd)

	destroyCmd.Flags().BoolVar(&confirmDestroy, "confirm", false, "Confirm the destroy action")
	_ = destroyCmd.MarkFlagRequired("confirm")
}
