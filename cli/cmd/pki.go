package cmd

import (
	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/message"
	"github.com/defenseunicorns/zarf/cli/internal/message/tls"
	"github.com/defenseunicorns/zarf/cli/internal/pki"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
	"github.com/spf13/cobra"
)

var pkiCmd = &cobra.Command{
	Use:   "pki",
	Short: "PKI-related commands",
}

var pkiRegenerate = &cobra.Command{
	Use:   "regenerate",
	Short: "Regenerate the pki certs for the cluster ingress",
	Run: func(cmd *cobra.Command, args []string) {
		// Prompt for a hostname if it wasn't provided as a command flag
		if config.TLS.Host == "" {
			prompt := &survey.Input{
				Message: "Enter a host DNS entry or IP Address for the gitops service ingress. If using localhost, use " + config.IPV4Localhost,
			}
			_ = survey.AskOne(prompt, &config.TLS.Host, survey.WithValidator(survey.Required))
		}

		// Verify the hostname provided is valid
		if !utils.ValidHostname(config.TLS.Host) {
			message.Fatalf(nil, tls.InvalidHostMessage, config.TLS.Host)
		}

		pki.GeneratePKI()
	},
}

var pkiImport = &cobra.Command{
	Use:   "import",
	Short: "Import an existing key pair for the cluster ingress",
	Run: func(cmd *cobra.Command, args []string) {
		pki.HandlePKI()
	},
}

func init() {
	rootCmd.AddCommand(pkiCmd)
	pkiCmd.AddCommand(pkiRegenerate)
	pkiCmd.AddCommand(pkiImport)

	pkiRegenerate.Flags().StringVar(&config.TLS.Host, "host", "", "Specify the host or IP for the gitops service ingress")
	pkiImport.Flags().StringVar(&config.TLS.CertPublicPath, "server-crt", "", "Path to the server public key if not generating unique PKI")
	pkiImport.Flags().StringVar(&config.TLS.CertPrivatePath, "server-key", "", "Path to the server private key if not generating unique PKI")
}
