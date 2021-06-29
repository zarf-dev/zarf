package cmd

import (
	"github.com/spf13/cobra"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/packager"
)

const updatePackageName = "zarf-update.tar.zst"

var packageCmd = &cobra.Command{
	Use:   "package",
	Short: "Pack and unpack updates for the Zarf utility cluster.",
}

// packageCreateCmd represents the build command
var packageCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an update package to push to the utility server (runs online)",
	Run: func(cmd *cobra.Command, args []string) {
		packager.Create(updatePackageName)
	},
}

// packageDeployCmd represents the build command
var packageDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "deploys an update package file (runs offline)",
	Run: func(cmd *cobra.Command, args []string) {
		packager.Deploy(updatePackageName)
	},
}

func init() {
	rootCmd.AddCommand(packageCmd)
	packageCmd.AddCommand(packageCreateCmd)
	packageCmd.AddCommand(packageDeployCmd)
}
