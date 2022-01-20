package tls

import (
	"net"
	"os"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/message"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
)

const InvalidHostMessage = "The hostname provided (%s) was not a valid hostname. The hostname can only contain: 'a-z', 'A-Z', '0-9', '-', and '.' characters as defined by RFC-1035.  If using localhost, you must use the 127.0.0.1.\n"

// HasCertPaths Check for cert paths provided via automation (both required)
func HasCertPaths() bool {
	return config.TLS.CertPrivatePath != "" && config.TLS.CertPublicPath != ""
}

// PromptIsImportCerts Ask user if they will be importing or generating certs, return true if importing certs
func PromptIsImportCerts(confirmed bool) bool {
	var mode int

	if HasCertPaths() {
		return true
	}

	if confirmed {
		// Assume generate on confirmed without cert paths
		return false
	}

	message.Question(`
		Zarf needs a valid TLS certificate and key to serve content.  This can be automatically generated
		for you, but will require you to provide the generated certificate authority public key to any
		systems that will connect to this cluster.  Failure to do so may generating a warning for users or
		fail to connect to the cluster. You can also provide your own X509 certificates instead.`)

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

// PromptCertPaths Ask user for the public and private key paths to import into the cluster
func PromptCertPaths() {
	prompt := &survey.Input{
		Message: "Enter a file path to the ingress public key",
		Suggest: func(toComplete string) []string {
			// Give some suggestions to users
			files, _ := filepath.Glob(toComplete + "*")
			return files
		},
	}
	_ = survey.AskOne(prompt, &config.TLS.CertPublicPath, survey.WithValidator(survey.Required))

	prompt.Message = "Enter a file path to the ingress private key"
	_ = survey.AskOne(prompt, &config.TLS.CertPrivatePath, survey.WithValidator(survey.Required))
}

// PromptAndValidateHost Ask user for the hostname or ip if not provided via automation and validate the input
func PromptAndValidateHost(confirmed bool) {
	if config.TLS.Host == "" {
		if confirmed {
			// Fail if host is not provided on confirm
			message.Fatalf(nil, InvalidHostMessage, config.TLS.Host)
		}

		message.Question(`
			Zarf needs to know what static IP address or DNS name will be exposed for traffic
			routed into the cluster. This will be how you connect to the cluster and if importing a
			certificate should match the Subject Alternate Name specified in that certificate.`)

		message.Note(" Note: if using localhost, be sure to choose " + config.IPV4Localhost)

		// If not provided, always ask for a host entry to avoid having to guess which entry in a cert if provided
		prompt := &survey.Input{
			Message: "What IP address or DNS name do you want to use?",
			Suggest: func(toComplete string) []string {
				var suggestions []string
				// Create a list of IPs to add to the suggestion box
				interfaces, err := net.InterfaceAddrs()
				if err == nil {
					for _, iface := range interfaces {
						// Convert the CIDR to the IP string if valid
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
		err := survey.AskOne(prompt, &config.TLS.Host, survey.WithValidator(survey.Required))
		if err != nil && err.Error() == os.Interrupt.String() {
			// Handle CTRL+C
			os.Exit(0)
		}
	}

	if !utils.ValidHostname(config.TLS.Host) {
		// When hitting an invalid hostname...
		if confirmed {
			// ...if using automation end it all
			message.Fatalf(nil, InvalidHostMessage, config.TLS.Host)
		}
		// ...otherwise, warn user, reset the field, and cycle the function
		message.Fatalf(nil, InvalidHostMessage, config.TLS.Host)
		config.TLS.Host = ""
		PromptAndValidateHost(confirmed)
	}
}

func HandleTLSOptions(confirmed bool) {
	// Get and validate host
	PromptAndValidateHost(confirmed)

	// Get the cert path if this is an import
	if PromptIsImportCerts(confirmed) && !HasCertPaths() {
		PromptCertPaths()
	}
}
