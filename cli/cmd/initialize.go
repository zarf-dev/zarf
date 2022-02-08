package cmd

import (
	"net"
	"os"
	"path/filepath"

	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/packager"

	"github.com/defenseunicorns/zarf/cli/internal/pki"
	"github.com/defenseunicorns/zarf/cli/internal/utils"

	"github.com/AlecAivazis/survey/v2"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const invalidHostMessage = "The hostname provided (%v) was not a valid hostname. The hostname can only contain: 'a-z', 'A-Z', '0-9', '-', and '.' characters as defined by RFC-1035.  If using localhost, you must use the 127.0.0.1.\n"

var initOptions = packager.InstallOptions{}
var state = config.ZarfState{
	Kind: "ZarfState",
}

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Deploys the gitops service or appliance cluster on a clean linux box",
	Long:  "Flags are only required if running via automation, otherwise the init command will prompt you for your configuration choices",
	Run: func(cmd *cobra.Command, args []string) {

		if !initOptions.Confirmed {
			var confirm bool
			prompt := &survey.Confirm{
				Message: "⚠️  This will initialize a new Zarf deployment on this machine which will make changes to your filesystem. You should not run zarf init more than once without first running zarf destroy.  Do you want to continue?",
			}
			_ = survey.AskOne(prompt, &confirm)
			if !confirm {
				// Gracefully exit because they didn't want to play after all :-/
				os.Exit(0)
			}
		}

		handleTLSOptions()
		pki.HandlePKI()
		packager.Install(&initOptions)
	},
}

// Check for cert paths provided via automation (both required)
func hasCertPaths() bool {
	return state.TLS.CertPrivatePath != "" && state.TLS.CertPublicPath != ""
}

// Ask user if they will be importing or generating certs, return true if importing certs
func promptIsImportCerts() bool {
	var mode int

	if hasCertPaths() {
		return true
	}

	if initOptions.Confirmed {
		// Assume generate on confirmed without cert paths
		return false
	}

	// Determine flow for generate or import
	modePrompt := &survey.Select{
		Message: "Will Zarf be generating a TLS chain or importing an existing ingress cert?",
		Options: []string{
			"Generate TLS chain with an ephemeral CA",
			"Import user-provided cert keypair",
		},
	}
	_ = survey.AskOne(modePrompt, &mode)

	return mode == 1
}

// Ask user for the public and private key paths to import into the cluster
func promptCertPaths() {
	prompt := &survey.Input{
		Message: "Enter a file path to the ingress public key",
		Suggest: func(toComplete string) []string {
			// Give some suggestions to users
			files, _ := filepath.Glob(toComplete + "*")
			return files
		},
	}
	_ = survey.AskOne(prompt, &state.TLS.CertPublicPath, survey.WithValidator(survey.Required))

	prompt.Message = "Enter a file path to the ingress private key"
	_ = survey.AskOne(prompt, &state.TLS.CertPrivatePath, survey.WithValidator(survey.Required))
}

// Ask user for the hostname or ip if not provided via automation and validate the input
func promptAndValidateHost() {
	if state.TLS.Host == "" {
		if initOptions.Confirmed {
			// Fail if host is not provided on confirm
			logrus.Fatalf(invalidHostMessage, state.TLS.Host)
		}

		// If not provided, always ask for a host entry to avoid having to guess which entry in a cert if provided
		prompt := &survey.Input{
			Message: "Enter a host DNS entry or IP Address for the cluster ingress. If using localhost, use 127.0.0.1",
			Suggest: func(toComplete string) []string {
				var suggestions []string
				// Create a list of IPs to add to the suggestion box
				interfaces, err := net.InterfaceAddrs()
				if err == nil {
					for _, iface := range interfaces {
						// Conver the CIRD to the IP string if valid
						ip, _, _ := net.ParseCIDR(iface.String())
						if utils.ValidHostname(ip.String()) {
							suggestions = append(suggestions, ip.String())
						}
					}
				}
				// Add the localhost hostname as well
				hostname, _ := os.Hostname()
				if hostname != "" {
					suggestions = append(suggestions, hostname)
				}

				return suggestions
			},
		}
		err := survey.AskOne(prompt, &state.TLS.Host, survey.WithValidator(survey.Required))
		if err != nil && err.Error() == os.Interrupt.String() {
			// Handle CTRL+C
			os.Exit(0)
		}
	}

	if !utils.ValidHostname(state.TLS.Host) {
		// When hitting an invalid hostname...
		if initOptions.Confirmed {
			// ...if using automation end it all
			logrus.Fatalf(invalidHostMessage, state.TLS.Host)
		}
		// ...otherwise, warn user, reset the field, and cycle the function
		logrus.Warnf(invalidHostMessage, state.TLS.Host)
		state.TLS.Host = ""
		promptAndValidateHost()
	}
}

func handleTLSOptions() {

	// Get and validate host
	promptAndValidateHost()

	// Get the cert path if this is an import
	if promptIsImportCerts() && !hasCertPaths() {
		promptCertPaths()
	}

	// Persist the config the ZarfState
	if err := config.WriteState(state); err != nil {
		logrus.Debug(err)
		logrus.Fatal("Unable to save the zarf state file.")
	}
}

func init() {

	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolVar(&initOptions.Confirmed, "confirm", false, "Confirm the install without prompting")
	initCmd.Flags().StringVar(&state.TLS.Host, "host", "", "Specify the host or IP for the gitops service ingress.  E.g. host=10.10.10.5 or host=gitops.domain.com")
	initCmd.Flags().StringVar(&state.TLS.CertPublicPath, "server-crt", "", "Path to the server public key if not generating unique PKI")
	initCmd.Flags().StringVar(&state.TLS.CertPrivatePath, "server-key", "", "Path to the server private key if not generating unique PKI")
	initCmd.Flags().StringVar(&initOptions.Components, "components", "", "Comma-separated list of components to install.  Adding this flag will skip the init prompts for which components to install")
}
