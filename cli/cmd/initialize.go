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

var initBootstrap = &cobra.Command{
	Use:    "bootstrap [PATH] [SEEDIMAGES]",
	Hidden: true,
	Short:  "Internal command used to setup the in-cluster registry",
	Args:   cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		// Convert to path and image list
		path, images := args[0], args[1:]
		packager.LoadInternalSeedRegistry(path, images)
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.AddCommand(initBootstrap)
	initCmd.Flags().BoolVar(&config.DeployOptions.Confirm, "confirm", false, "Confirm the install without prompting")
	initCmd.Flags().StringVar(&config.DeployOptions.Components, "components", "", "Comma-separated list of components to install.  Adding this flag will skip the init prompts for which components to install")
}
