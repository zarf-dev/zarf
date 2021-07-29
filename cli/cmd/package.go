package cmd

import (
	"github.com/spf13/cobra"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/config"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/packager"
)

var packageAdditionalConfig string
var confirmCreate bool
var confirmDeploy bool

var packageCmd = &cobra.Command{
	Use:   "package",
	Short: "Pack and unpack updates for the Zarf utility cluster.",
}

var packageCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an update package to push to the utility server (runs online)",
	Run: func(cmd *cobra.Command, args []string) {
		if packageAdditionalConfig != "" {
			config.DynamicConfigLoad(packageAdditionalConfig)
		}
		if config.IsZarfInitConfig() {
			if config.IsApplianceMode() {
				packager.Create(config.PackageApplianceName, confirmCreate)
			} else {
				packager.Create(config.PackageInitName, confirmCreate)
			}
		} else {
			packager.Create(config.PackageUpdateName, confirmCreate)
		}
	},
}

var packageDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "deploys an update package file (runs offline)",
	Run: func(cmd *cobra.Command, args []string) {
		packager.Deploy(config.PackageUpdateName, confirmDeploy)
	},
}

var packageInspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "lists the paylod of an update package file (runs offline)",
	Run: func(cmd *cobra.Command, args []string) {
		packager.Inspect(config.PackageUpdateName)
	},
}

func init() {
	rootCmd.AddCommand(packageCmd)
	packageCmd.AddCommand(packageCreateCmd)
	packageCmd.AddCommand(packageDeployCmd)
	packageCmd.AddCommand(packageInspectCmd)

	packageCreateCmd.Flags().StringVar(&packageAdditionalConfig, "config", "", "Provide an additional config file to merge with the default config")
	packageCreateCmd.Flags().BoolVar(&confirmCreate, "confirm", false, "Confirm package creation without prompting")
	packageDeployCmd.Flags().BoolVar(&confirmDeploy, "confirm", false, "Confirm package deployment without prompting")
}
