package cmd

import (
	"fmt"
	"os"

	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/message"
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
		zarfLogo := message.GetLogo()
		_, _ = fmt.Fprintln(os.Stderr, zarfLogo)

		// Continue running package deploy for all components like any other package
		config.DeployOptions.PackagePath = fmt.Sprintf("zarf-init-%s.tar.zst", config.GetArch())

		// Run everything
		packager.Deploy()
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolVar(&config.DeployOptions.Confirm, "confirm", false, "Confirm the install without prompting")
	initCmd.Flags().StringVar(&config.DeployOptions.Components, "components", "", "Comma-separated list of components to install.  Adding this flag will skip the init prompts for which components to install")

	addZarfStateOverrideFlags(initCmd, false)

}

// Add flags to override values that will be added to ZarfState during an init
// An options is included to make the overwrite flags hidden
func addZarfStateOverrideFlags(commad *cobra.Command, makeHidden bool) {
	commad.Flags().StringVar(&config.DeployOptions.Distro, "distro", "", "Kubernetes distro Zarf is deploying to")
	commad.Flags().StringVar(&config.DeployOptions.StorageClass, "storage-class", "", "Describe the StorageClass to be used")
	commad.Flags().StringVar(&config.DeployOptions.Secret, "secret", "", "Root secret value that is used to 'seed' other secrets")
	commad.Flags().StringVar(&config.DeployOptions.NodePort, "nodeport", "", "Nodeport to access the Zarf container registry")
	commad.Flags().StringVar(&config.DeployOptions.Architecture, "architecture", "", "Architecture for OCI images")

	if makeHidden {
		packageDeployCmd.Flags().MarkHidden("distro")
		packageDeployCmd.Flags().MarkHidden("storage-class")
		packageDeployCmd.Flags().MarkHidden("secret")
		packageDeployCmd.Flags().MarkHidden("nodeport")
		packageDeployCmd.Flags().MarkHidden("architecture")
	}
}
