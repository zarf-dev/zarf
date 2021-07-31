package cmd

import (
	"path/filepath"

	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/k3s"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

var initModeSelect string
var initOptions = k3s.InstallOptions{}

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Deploys the utility cluster or appliance cluster on a clean linux box",
	Long:  "Flags are only required if running via automation, otherwise the init command will prompt you for your configuration choices",
	Run: func(cmd *cobra.Command, args []string) {
		handleModeChoice()
		handleTLSOptions()
		k3s.Install(initOptions)
	},
}

func handleModeChoice() {
	const (
		Appliance int = iota
		Standard
	)

	modes := map[string]int{
		"appliance": Appliance,
		"standard":  Standard,
	}

	mode, validMode := modes[initModeSelect]

	// Only allow a valid mode, otherwise prompt the user
	if !validMode {
		modePrompt := &survey.Select{
			Message: "What mode will Zarf be initalizing in?",
			Options: []string{
				"Appliance Mode (single-node system or custom config)",
				"Standard Mode (utility cluster with gitea and image registry)",
			},
		}
		survey.AskOne(modePrompt, &mode)
	}

	initOptions.ApplianceMode = mode == Appliance
}

func handleTLSOptions() {

	// Check to see if the certpaths or host entries are set as flags first
	if initOptions.PKI.CertPublicPath == "" && initOptions.PKI.Host == "" {

		const (
			Generate int = iota
			Import
		)

		var tlsMode int

		// Determine flow for generate or import
		modePrompt := &survey.Select{
			Message: "Will Zarf be generating a TLS chain or importing an existing ingress cert?",
			Options: []string{
				"Generate TLS chain with an ephemeral CA",
				"Import user-provided cert keypair",
			},
		}
		survey.AskOne(modePrompt, &tlsMode)

		if tlsMode == Generate {
			// Generate mode requires a host entry
			prompt := &survey.Input{
				Message: "Enter a host DNS entry or IP Address for the cluster ingress",
			}
			survey.AskOne(prompt, &initOptions.PKI.Host, survey.WithValidator(survey.Required))
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
			survey.AskOne(prompt, &initOptions.PKI.CertPublicPath, survey.WithValidator(survey.Required))

			prompt.Message = "Enter a file path to the ingress private key"
			survey.AskOne(prompt, &initOptions.PKI.CertPrivatePath, survey.WithValidator(survey.Required))
		}
	}
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolVar(&initOptions.Confirmed, "confirm", false, "Confirm the install without prompting")
	initCmd.Flags().StringVar(&initOptions.PKI.Host, "host", "", "Specify the host or IP for the utility cluster ingress.  E.g. host=10.10.10.5 or host=utility.domain.com")
	initCmd.Flags().StringVar(&initModeSelect, "mode", "", "Configure the type of cluster Zarf will initialize.  Valid options are [appliance] or [standard], e.g. mode=standard")
	initCmd.Flags().StringVar(&initOptions.PKI.CertPublicPath, "server-crt", "", "Path to the server public key if not generating unique PKI")
	initCmd.Flags().StringVar(&initOptions.PKI.CertPrivatePath, "server-key", "", "Path to the server private key if not generating unique PKI")
}
