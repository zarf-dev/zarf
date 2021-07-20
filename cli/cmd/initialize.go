package cmd

import (
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/config"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/k3s"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/utils"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var confirmInit bool
var host string
var applianceMode bool
var certPublicPath string
var certPrivatePath string

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Deploys the utility cluster on a clean linux box",
	Run: func(cmd *cobra.Command, args []string) {
		k3s.Install(host, applianceMode)
		if certPublicPath != "" && certPrivatePath != "" {
			logrus.WithFields(logrus.Fields{
				"public":  certPublicPath,
				"private": certPrivatePath,
			}).Info("Injecting user-provided keypair for ingress TLS")
			utils.InjectServerCert(certPublicPath, certPrivatePath)
		} else {
			utils.GeneratePKI(host)
		}
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolVar(&confirmInit, "confirm", false, "Confirm the install action")
	initCmd.Flags().StringVar(&host, "host", "zarf-server", "Specify the host or IP for the utility cluster ingress")
	initCmd.Flags().BoolVar(&applianceMode, "appliance-mode", false, "Enable appliance mode, ensure the "+config.PackageApplianceName+" package is in the same directory")
	initCmd.Flags().StringVar(&certPublicPath, "server-crt", "", "Path to the server public key if not generating unique PKI")
	initCmd.Flags().StringVar(&certPrivatePath, "server-key", "", "Path to the server private key if not generating unique PKI")
	initCmd.MarkFlagRequired("confirm")
	initCmd.MarkFlagRequired("host")
}
