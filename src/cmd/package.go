package cmd

import (
	"fmt"
	"path/filepath"
	"regexp"

	"github.com/defenseunicorns/zarf/src/internal/message"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/packager"
	"github.com/spf13/cobra"
)

var insecureDeploy bool
var shasum string
var zarfImageCache string

var packageCmd = &cobra.Command{
	Use:     "package",
	Aliases: []string{"p"},
	Short:   "Pack and unpack updates for the Zarf gitops service.",
}

var packageCreateCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"c"},
	Short:   "Create an update package to push to the gitops server (runs online)",
	Long: "Builds a tarball of resources and dependencies defined by the 'zarf.yaml' located in the working directory.\n" +
		"Private registries and repositories are accessed via credentials in your local '~/.docker/config.json' " +
		"and '~/.git-credentials'.\n",
	Run: func(cmd *cobra.Command, args []string) {
		if cmd.Flag("zarf-cache").Changed && cachePathClean(zarfImageCache) {
			config.SetImageCachePath(zarfImageCache)
		}
		packager.Create()
	},
}

var packageDeployCmd = &cobra.Command{
	Use:     "deploy [PACKAGE]",
	Aliases: []string{"d"},
	Short:   "Deploys an update package from a local file or URL (runs offline)",
	Long:    "Uses current kubecontext to deploy the packaged tarball onto a k8s cluster.",
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var done func()
		packageName := choosePackage(args)
		config.DeployOptions.PackagePath, done = packager.HandleIfURL(packageName, shasum, insecureDeploy)
		defer done()
		packager.Deploy()
	},
}

var packageGenerateCmd = &cobra.Command{
	Use:     "generate [NAME]",
	Aliases: []string{"g"},
	Short:   "Creates a Zarf package directory.",
	Long:    "This command creates a Zarf package directory along with the common files and directories used in a Zarf package.",
	Args:    cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		o := config.GenerateOptions
		o.PackageName = args[0]
		PackageName := filepath.Base(o.PackageName)
		packager.Generate(PackageName, filepath.Dir(o.PackageName))
	},
}

var packageInspectCmd = &cobra.Command{
	Use:     "inspect [PACKAGE]",
	Aliases: []string{"i"},
	Short:   "Lists the payload of a compiled package file (runs offline)",
	Long: "Lists the payload of a compiled package file (runs offline)\n" +
		"Unpacks the package tarball into a temp directory and displays the " +
		"contents of the zarf.yaml package definition as well as the version " +
		"of the Zarf CLI that was used to build the package.",
	Args: cobra.MaximumNArgs(1),
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

func cachePathClean(cachePath string) bool {
	var isCleanPath = regexp.MustCompile(`^[a-zA-Z0-9\_\-\/\.\~]+$`).MatchString
	if !isCleanPath(cachePath) {
		message.Warn(fmt.Sprintf("Invalid characters in Zarf cache path, defaulting to ~/%s", config.ZarfDefaultImageCachePath))
		return false
	}
	return true
}

func init() {
	rootCmd.AddCommand(packageCmd)
	packageCmd.AddCommand(packageCreateCmd)
	packageCmd.AddCommand(packageDeployCmd)
	packageCmd.AddCommand(packageGenerateCmd)
	packageCmd.AddCommand(packageInspectCmd)

	packageCreateCmd.Flags().BoolVar(&config.DeployOptions.Confirm, "confirm", false, "Confirm package creation without prompting")
	packageCreateCmd.Flags().StringVar(&zarfImageCache, "zarf-cache", config.ZarfDefaultImageCachePath, "Specify the location of the Zarf image cache")
	packageCreateCmd.Flags().BoolVar(&config.CreateOptions.SkipSBOM, "skip-sbom", false, "Skip generating SBOM for this package")
	packageCreateCmd.Flags().BoolVar(&config.CreateOptions.Insecure, "insecure", false, "Allow insecure registry connections when pulling OCI images")

	packageDeployCmd.Flags().BoolVar(&config.DeployOptions.Confirm, "confirm", false, "Confirm package deployment without prompting")
	packageDeployCmd.Flags().StringVar(&config.DeployOptions.Components, "components", "", "Comma-separated list of components to install.  Adding this flag will skip the init prompts for which components to install")
	packageDeployCmd.Flags().BoolVar(&insecureDeploy, "insecure", false, "Skip shasum validation of remote package. Required if deploying a remote package and `--shasum` is not provided")
	packageDeployCmd.Flags().StringVar(&shasum, "shasum", "", "Shasum of the package to deploy. Required if deploying a remote package and `--insecure` is not provided")
}
