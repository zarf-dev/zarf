package cmd

import (
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/config"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/k3s"

	"github.com/spf13/cobra"
)

var initOptions = k3s.InstallOptions{}

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Deploys the utility cluster or appliance cluster on a clean linux box",
	Run: func(cmd *cobra.Command, args []string) {
		k3s.Install(initOptions)
	},
}

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().BoolVar(&initOptions.Confirmed, "confirm", false, "Confirm the install without prompting")
	initCmd.Flags().StringVar(&initOptions.PKI.Host, "host", "zarf-server", "Specify the host or IP for the utility cluster ingress")
	initCmd.Flags().BoolVar(&initOptions.ApplianceMode, "appliance-mode", false, "Enable appliance mode, ensure the "+config.PackageApplianceName+" package is in the same directory")
	initCmd.Flags().StringVar(&initOptions.PKI.CertPublicPath, "server-crt", "", "Path to the server public key if not generating unique PKI")
	initCmd.Flags().StringVar(&initOptions.PKI.CertPrivatePath, "server-key", "", "Path to the server private key if not generating unique PKI")
	initCmd.MarkFlagRequired("host")
}
