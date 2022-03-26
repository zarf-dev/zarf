package cmd

import (
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/packager"
	"github.com/spf13/cobra"
)

var insecureDeploy bool
var shasum string

var packageCmd = &cobra.Command{
	Use:     "package",
	Aliases: []string{"p"},
	Short:   "Pack and unpack updates for the Zarf gitops service.",
}

var packageCreateCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"c"},
	Short:   "Create an update package to push to the gitops server (runs online)",
	Run: func(cmd *cobra.Command, args []string) {
		packager.Create()
	},
}

var packageDeployCmd = &cobra.Command{
	Use:     "deploy [PACKAGE]",
	Aliases: []string{"d"},
	Short:   "Deploys an update package from a local file or URL (runs offline)",
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var done func()
		packageName := choosePackage(args)
		config.DeployOptions.PackagePath, done = packager.HandleIfURL(packageName, shasum, insecureDeploy)
		defer done()
		packager.Deploy()
	},
}

var packageInspectCmd = &cobra.Command{
	Use:     "inspect [PACKAGE]",
	Aliases: []string{"i"},
	Short:   "lists the payload of an update package file (runs offline)",
	Args:    cobra.MaximumNArgs(1),
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
			files, _ := filepath.Glob(config.PackagePrefix + toComplete + "*.tar*")
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

	packageCreateCmd.Flags().BoolVar(&config.DeployOptions.Confirm, "confirm", false, "Confirm package creation without prompting")
	packageDeployCmd.Flags().BoolVar(&config.DeployOptions.Confirm, "confirm", false, "Confirm package deployment without prompting")
	packageDeployCmd.Flags().StringVar(&config.DeployOptions.Components, "components", "", "Comma-separated list of components to install.  Adding this flag will skip the init prompts for which components to install")
	packageDeployCmd.Flags().BoolVar(&insecureDeploy, "insecure", false, "Skip shasum validation of remote package. Required if deploying a remote package and `--shasum` is not provided")
	packageDeployCmd.Flags().StringVar(&shasum, "shasum", "", "Shasum of the package to deploy. Required if deploying a remote package and `--insecure` is not provided")

	// Make flags for initializing hidden
	addZarfStateOverrideFlags(packageDeployCmd, true)
}
