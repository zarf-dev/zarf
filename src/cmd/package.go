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
	Short:   "Zarf package commands for creating, deploying, and inspecting packages",
}

var packageCreateCmd = &cobra.Command{
	Use:     "create [DIRECTORY]",
	Aliases: []string{"c"},
	Args:    cobra.MaximumNArgs(1),
	Short:   "Use to create a Zarf package from a given directory or the current directory",
	Long: "Builds an archive of resources and dependencies defined by the 'zarf.yaml' in the active directory.\n" +
		"Private registries and repositories are accessed via credentials in your local '~/.docker/config.json' " +
		"and '~/.git-credentials'.\n",
	Run: func(cmd *cobra.Command, args []string) {

		var baseDir string

		// If a directory was provided, use that as the base directory
		if len(args) > 0 {
			baseDir = args[0]
		}

		if zarfImageCache != config.ZarfDefaultImageCachePath && cachePathClean(zarfImageCache) {
			config.SetImageCachePath(zarfImageCache)
		}

		packager.Create(baseDir)
	},
}

var packageDeployCmd = &cobra.Command{
	Use:     "deploy [PACKAGE]",
	Aliases: []string{"d"},
	Short:   "Use to deploy a Zarf package from a local file or URL (runs offline)",
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

var packageInspectCmd = &cobra.Command{
	Use:     "inspect [PACKAGE]",
	Aliases: []string{"i"},
	Short:   "Lists the payload of a Zarf package (runs offline)",
	Long: "Lists the payload of a compiled package file (runs offline)\n" +
		"Unpacks the package tarball into a temp directory and displays the " +
		"contents of the archive.",
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
	packageCmd.AddCommand(packageInspectCmd)

	packageCreateCmd.Flags().BoolVar(&config.CommonOptions.Confirm, "confirm", false, "Confirm package creation without prompting")
	packageCreateCmd.Flags().StringVar(&config.CommonOptions.TempDirectory, "tmpdir", "", "Specify the temporary directory to use for intermediate files")
	packageCreateCmd.Flags().StringToStringVar(&config.CommonOptions.SetVariables, "set", map[string]string{}, "Specify package variables to set on the command line (KEY=value)")
	packageCreateCmd.Flags().StringVar(&zarfImageCache, "zarf-cache", config.ZarfDefaultImageCachePath, "Specify the location of the Zarf image cache")
	packageCreateCmd.Flags().StringVarP(&config.CreateOptions.OutputDirectory, "output-directory", "o", "", "Specify the output directory for the created Zarf package")
	packageCreateCmd.Flags().BoolVar(&config.CreateOptions.SkipSBOM, "skip-sbom", false, "Skip generating SBOM for this package")
	packageCreateCmd.Flags().BoolVar(&config.CreateOptions.Insecure, "insecure", false, "Allow insecure registry connections when pulling OCI images")

	packageDeployCmd.Flags().BoolVar(&config.CommonOptions.Confirm, "confirm", false, "Confirm package deployment without prompting")
	packageDeployCmd.Flags().StringVar(&config.CommonOptions.TempDirectory, "tmpdir", "", "Specify the temporary directory to use for intermediate files")
	packageDeployCmd.Flags().StringToStringVar(&config.CommonOptions.SetVariables, "set", map[string]string{}, "Specify package variables to set on the command line (KEY=value)")
	packageDeployCmd.Flags().StringVar(&config.DeployOptions.Components, "components", "", "Comma-separated list of components to install.  Adding this flag will skip the init prompts for which components to install")
	packageDeployCmd.Flags().BoolVar(&insecureDeploy, "insecure", false, "Skip shasum validation of remote package. Required if deploying a remote package and `--shasum` is not provided")
	packageDeployCmd.Flags().StringVar(&shasum, "shasum", "", "Shasum of the package to deploy. Required if deploying a remote package and `--insecure` is not provided")
	packageDeployCmd.Flags().StringVar(&config.DeployOptions.SGetKeyPath, "sget", "", "Path to public sget key file for remote packages signed via cosign")

	packageInspectCmd.Flags().StringVar(&config.CommonOptions.TempDirectory, "tmpdir", "", "Specify the temporary directory to use for intermediate files")
}
