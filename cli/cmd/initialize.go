package cmd

import (
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/k3s"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/utils"

	"github.com/spf13/cobra"
)

var confirmInit bool
var host string

// initializeCmd represents the initialize command
var initializeCmd = &cobra.Command{
	Use:   "initialize",
	Short: "Deploys the utility cluster on a clean linux box",
	Run: func(cmd *cobra.Command, args []string) {
		k3s.Install(host)
		utils.GeneratePKI(host)
	},
}

func init() {
	rootCmd.AddCommand(initializeCmd)
	initializeCmd.Flags().BoolVar(&confirmInit, "confirm", false, "Confirm the install action")
	initializeCmd.Flags().StringVar(&host, "host", "zarf-server", "Specify the host or IP for the utility cluster ingress")
	initializeCmd.MarkFlagRequired("confirm")
	initializeCmd.MarkFlagRequired("host")
}
