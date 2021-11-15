package cmd

import (
	"path/filepath"

	"github.com/defenseunicorns/zarf/cli/internal/k3s"
	"github.com/defenseunicorns/zarf/cli/internal/pki"
	"github.com/defenseunicorns/zarf/cli/internal/utils"

	"github.com/AlecAivazis/survey/v2"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var initOptions = k3s.InstallOptions{}

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Deploys the gitops service or appliance cluster on a clean linux box",
	Long:  "Flags are only required if running via automation, otherwise the init command will prompt you for your configuration choices",
	Run: func(cmd *cobra.Command, args []string) {
		handleTLSOptions()
		pki.HandlePKI(&initOptions.PKI)
		k3s.Install(&initOptions)
	},
}

func handleTLSOptions() {
	// Check to see if the certpaths or host entries are set as flags first
	if initOptions.PKI.CertPublicPath == "" && initOptions.PKI.Host == "" {

		const Generate = 0

		var tlsMode int

		// Determine flow for generate or import
		modePrompt := &survey.Select{
			Message: "Will Zarf be generating a TLS chain or importing an existing ingress cert?",
			Options: []string{
				"Generate TLS chain with an ephemeral CA",
				"Import user-provided cert keypair",
			},
		}
		_ = survey.AskOne(modePrompt, &tlsMode)

		if tlsMode == Generate {
			// Generate mode requires a host entry
			prompt := &survey.Input{
				Message: "Enter a host DNS entry or IP Address for the cluster ingress",
			}
			_ = survey.AskOne(prompt, &initOptions.PKI.Host, survey.WithValidator(survey.Required))
		} else {
			// Import mode requires the public and private key paths
			prompt := &survey.Input{
				Message: "Enter a file path to the ingress public key",
				Suggest: func(toComplete string) []string {
					// Give some suggestions to users
					files, _ := filepath.Glob(toComplete + "*")
					return files
				},
			}
			_ = survey.AskOne(prompt, &initOptions.PKI.CertPublicPath, survey.WithValidator(survey.Required))

			prompt.Message = "Enter a file path to the ingress private key"
			_ = survey.AskOne(prompt, &initOptions.PKI.CertPrivatePath, survey.WithValidator(survey.Required))
		}
	}
	if !utils.CheckHostName(initOptions.PKI.Host) {
		logrus.Fatalf("The hostname provided (%v) was not a valid hostname. The hostname can only contain: 'a-z', 'A-Z', '0-9', '-', and '.' characters.\n", initOptions.PKI.Host)
	}
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolVar(&initOptions.Confirmed, "confirm", false, "Confirm the install without prompting")
	initCmd.Flags().StringVar(&initOptions.PKI.Host, "host", "", "Specify the host or IP for the gitops service ingress.  E.g. host=10.10.10.5 or host=gitops.domain.com")
	initCmd.Flags().StringVar(&initOptions.PKI.CertPublicPath, "server-crt", "", "Path to the server public key if not generating unique PKI")
	initCmd.Flags().StringVar(&initOptions.PKI.CertPrivatePath, "server-key", "", "Path to the server private key if not generating unique PKI")
	initCmd.Flags().StringVar(&initOptions.Components, "components", "", "Comma-separated list of components to install.  Adding this flag will skip the init prompts for which components to install")
}
