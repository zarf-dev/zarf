package cmd

import (
	"github.com/spf13/cobra"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/config"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/packager"
)

var packageAdditionalConfig string

var packageCmd = &cobra.Command{
	Use:   "package",
	Short: "Pack and unpack updates for the Zarf utility cluster.",
}

// packageCreateCmd represents the build command
var packageCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an update package to push to the utility server (runs online)",
	Run: func(cmd *cobra.Command, args []string) {
		if packageAdditionalConfig != "" {
			config.DynamicConfigLoad(packageAdditionalConfig)
		}
		if config.IsZarfInitConfig() {
			packager.Create(config.PackageInitName)
		} else {
			packager.Create(config.PackageUpdateName)
		}
	},
}

// packageDeployCmd represents the build command
var packageDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "deploys an update package file (runs offline)",
	Run: func(cmd *cobra.Command, args []string) {
		packager.Deploy(config.PackageUpdateName)
	},
}

func init() {
	rootCmd.AddCommand(packageCmd)
	packageCmd.AddCommand(packageCreateCmd)
	packageCmd.AddCommand(packageDeployCmd)
	packageCreateCmd.Flags().StringVar(&packageAdditionalConfig, "config", "", "Provide an additional config file to merge with the default config")
}
