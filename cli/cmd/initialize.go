package cmd

import (
	"fmt"
	"os"

	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/packager"

	"github.com/spf13/cobra"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:     "init",
	Aliases: []string{"i"},
	Short:   "Deploys the gitops service or appliance cluster on a clean linux box",
	Long:    "Flags are only required if running via automation, otherwise the init command will prompt you for your configuration choices",
	Run: func(cmd *cobra.Command, args []string) {
		zarfLogo := getLogo()
		_, _ = fmt.Fprintln(os.Stderr, zarfLogo)

		// Continue running package deploy for all components like any other package
		config.DeployOptions.PackagePath = config.PackageInitName

		// Run everything
		packager.Deploy()
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolVar(&config.DeployOptions.Confirm, "confirm", false, "Confirm the install without prompting")
	initCmd.Flags().StringVar(&config.TLS.Host, "host", "", "Specify the host or IP for the gitops service ingress.  E.g. host=10.10.10.5 or host=gitops.domain.com")
	initCmd.Flags().StringVar(&config.TLS.CertPublicPath, "server-crt", "", "Path to the server public key if not generating unique PKI")
	initCmd.Flags().StringVar(&config.TLS.CertPrivatePath, "server-key", "", "Path to the server private key if not generating unique PKI")
	initCmd.Flags().StringVar(&config.DeployOptions.Components, "components", "", "Comma-separated list of components to install.  Adding this flag will skip the init prompts for which components to install")
}
