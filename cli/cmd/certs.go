package cmd

import (
	"github.com/spf13/cobra"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/utils"
)

var hostCert string

// certsCmd represents the registry command
var certsCmd = &cobra.Command{
	Use:   "regenerate-pki",
	Short: "Regenerate the pki certs for the utility cluster",
	Run: func(cmd *cobra.Command, args []string) {
		utils.GeneratePKI(hostCert)
	},
}

func init() {
	rootCmd.AddCommand(certsCmd)
	certsCmd.Flags().StringVar(&hostCert, "host", "zarf-server", "Specify the host or IP for the utility cluster ingress")
	certsCmd.MarkFlagRequired("host")
}
