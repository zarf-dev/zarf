package cmd

import (
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/config"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/packager"
)

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
		if config.IsZarfInitConfig() {
			packager.Create(config.PackageInitName, confirmCreate)
		} else {
			packageName := config.GetPackageName()
			packager.Create(packageName, confirmCreate)
		}
	},
}

var packageDeployCmd = &cobra.Command{
	Use:   "deploy PACKAGE",
	Short: "deploys an update package file (runs offline)",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		packageName := choosePackage(args)
		packager.Deploy(packageName, confirmDeploy)
	},
}

var packageInspectCmd = &cobra.Command{
	Use:   "inspect PACKAGE",
	Short: "lists the paylod of an update package file (runs offline)",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		packageName := choosePackage(args)
		packager.Inspect(packageName)
	},
}

func choosePackage(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	var path string
	prompt := &survey.Input{
		Message: "Choose or type the package file",
		Suggest: func(toComplete string) []string {
			files, _ := filepath.Glob("zarf-package-" + toComplete + "*.tar.zst")
			return files
		},
	}
	_ = survey.AskOne(prompt, &path, survey.WithValidator(survey.Required))
	return path
}

func init() {
	rootCmd.AddCommand(packageCmd)
	packageCmd.AddCommand(packageCreateCmd)
	packageCmd.AddCommand(packageDeployCmd)
	packageCmd.AddCommand(packageInspectCmd)

	packageCreateCmd.Flags().BoolVar(&confirmCreate, "confirm", false, "Confirm package creation without prompting")
	packageDeployCmd.Flags().BoolVar(&confirmDeploy, "confirm", false, "Confirm package deployment without prompting")
}
