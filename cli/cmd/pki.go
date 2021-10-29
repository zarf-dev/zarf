package cmd

import (
	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
	"github.com/sirupsen/logrus"
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
		// Prompt for a hostname if it wasn't provided as a command flag
		if pkiOptions.Host == "" {
			prompt := &survey.Input{
				Message: "Enter a host DNS entry or IP Address for the gitops service ingress",
			}
			_ = survey.AskOne(prompt, &pkiOptions.Host, survey.WithValidator(survey.Required))
		}

		// Verify the hostname provided is valid
		if !utils.CheckHostName(pkiOptions.Host) {
			logrus.Fatalf("The hostname provided (%v) was not a valid hostname. The hostname can only contain: 'a-z', 'A-Z', '0-9', '-', and '.' characters.\n", pkiOptions.Host)
		}

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

	pkiRegenerate.Flags().StringVar(&pkiOptions.Host, "host", "", "Specify the host or IP for the gitops service ingress")

	pkiImport.Flags().StringVar(&pkiOptions.CertPublicPath, "server-crt", "", "Path to the server public key if not generating unique PKI")
	pkiImport.Flags().StringVar(&pkiOptions.CertPrivatePath, "server-key", "", "Path to the server private key if not generating unique PKI")
}
