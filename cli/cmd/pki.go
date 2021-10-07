package cmd

import (
	"github.com/defenseunicorns/zarf/cli/internal/utils"
	"github.com/spf13/cobra"
)

var pkiOptions = utils.PKIConfig{}

var pkiCmd = &cobra.Command{
	Use:   "pki",
	Short: "PKI-related commands",
}

var pkiRegenerate = &cobra.Command{
	Use:   "regenerate",
	Short: "Regenerate the pki certs for the cluster ingress",
	Run: func(cmd *cobra.Command, args []string) {
		utils.GeneratePKI(pkiOptions)
	},
}

var pkiImport = &cobra.Command{
	Use:   "import",
	Short: "Import an existing key pair for the cluster ingress",
	Run: func(cmd *cobra.Command, args []string) {
		utils.HandlePKI(pkiOptions)
	},
}

func init() {
	rootCmd.AddCommand(pkiCmd)
	pkiCmd.AddCommand(pkiRegenerate)
	pkiCmd.AddCommand(pkiImport)

	pkiRegenerate.Flags().StringVar(&pkiOptions.Host, "host", "zarf-server", "Specify the host or IP for the gitops service ingress")
	_ = pkiRegenerate.MarkFlagRequired("host")

	pkiImport.Flags().StringVar(&pkiOptions.CertPublicPath, "server-crt", "", "Path to the server public key if not generating unique PKI")
	pkiImport.Flags().StringVar(&pkiOptions.CertPrivatePath, "server-key", "", "Path to the server private key if not generating unique PKI")
}
