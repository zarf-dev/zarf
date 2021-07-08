package cmd

import (
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/k3s"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/utils"

	"github.com/spf13/cobra"
)

var confirmInit bool
var host string

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Deploys the utility cluster on a clean linux box",
	Run: func(cmd *cobra.Command, args []string) {
		k3s.Install(host)
		utils.GeneratePKI(host)
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolVar(&confirmInit, "confirm", false, "Confirm the install action")
	initCmd.Flags().StringVar(&host, "host", "zarf-server", "Specify the host or IP for the utility cluster ingress")
	initCmd.MarkFlagRequired("confirm")
	initCmd.MarkFlagRequired("host")
}
