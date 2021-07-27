package cmd

import (
	"github.com/spf13/cobra"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/utils"
)

var pkiHostIdentity string

var pkiCmd = &cobra.Command{
	Use:   "pki",
	Short: "PKI-related commands",
}

// pkiRegenerate represents the registry command
var pkiRegenerate = &cobra.Command{
	Use:   "regenerate",
	Short: "Regenerate the pki certs for the cluster ingress",
	Run: func(cmd *cobra.Command, args []string) {
		utils.GeneratePKI(utils.PKIConfig{
			Host: pkiHostIdentity,
		})
	},
}

var pkiImport = &cobra.Command{
	Use:   "import",
	Short: "Import an existing key pair for the cluster ingress",
	Run: func(cmd *cobra.Command, args []string) {
		pkiConfig := utils.PKIConfig{
			CertPublicPath:  certPublicPath,
			CertPrivatePath: certPrivatePath,
		}
		utils.HandlePKI(pkiConfig)
	},
}

func init() {
	rootCmd.AddCommand(pkiCmd)
	pkiCmd.AddCommand(pkiRegenerate)
	pkiCmd.AddCommand(pkiImport)
	pkiRegenerate.Flags().StringVar(&pkiHostIdentity, "host", "zarf-server", "Specify the host or IP for the utility cluster ingress")
	pkiRegenerate.MarkFlagRequired("host")
	pkiImport.Flags().StringVar(&certPublicPath, "server-crt", "", "Path to the server public key if not generating unique PKI")
	pkiImport.Flags().StringVar(&certPrivatePath, "server-key", "", "Path to the server private key if not generating unique PKI")

}
