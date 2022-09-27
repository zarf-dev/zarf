package cmd

import (
	"fmt"
	"path/filepath"
	"regexp"

	"github.com/defenseunicorns/zarf/src/internal/k8s"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/pterm/pterm"

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

var packageListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"l"},
	Short:   "List out all of the packages that have been deployed to the cluster",
	Run: func(cmd *cobra.Command, args []string) {
		// Get all the deployed packages
		deployedZarfPackages, err := k8s.GetDeployedZarfPackages()
		if err != nil {
			message.Fatalf(err, "Unable to get the packages deployed to the cluster")
		}

		// Populate a pterm table of all the deployed packages
		packageTable := pterm.TableData{
			{"     Package ", "Components"},
		}

		for _, pkg := range deployedZarfPackages {
			var components []string

			for _, component := range pkg.DeployedComponents {
				components = append(components, component.Name)
			}

			packageTable = append(packageTable, pterm.TableData{{
				fmt.Sprintf("     %s", pkg.Name),
				fmt.Sprintf("%v", components),
			}}...)
		}

		// Print out the table for the user
		_ = pterm.DefaultTable.WithHasHeader().WithData(packageTable).Render()
	},
}

var packageRemoveCmd = &cobra.Command{
	Use:     "remove {PACKAGE_NAME}",
	Aliases: []string{"u"},
	Args:    cobra.ExactArgs(1),
	Short:   "Use to remove a Zarf package that has been deployed already",
	Run: func(cmd *cobra.Command, args []string) {
		err := packager.Remove(args[0])
		if err != nil {
			message.Fatalf(err, "Unable to remove the package with an error of: %#v", err)
		}
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

	if err := survey.AskOne(prompt, &path, survey.WithValidator(survey.Required)); err != nil {
		message.Fatalf(nil, "Package path selection canceled: %s", err.Error())
	}

	return path
}

func cachePathClean(cachePath string) bool {
	var isCleanPath = regexp.MustCompile(`^[a-zA-Z0-9\_\-\/\.\~]+$`).MatchString
	if !isCleanPath(cachePath) {
		message.Warnf("Invalid characters in Zarf cache path, defaulting to ~/%s", config.ZarfDefaultImageCachePath)
		return false
	}
	return true
}

func init() {
	initViper()

	rootCmd.AddCommand(packageCmd)
	packageCmd.AddCommand(packageCreateCmd)
	packageCmd.AddCommand(packageDeployCmd)
	packageCmd.AddCommand(packageInspectCmd)
	packageCmd.AddCommand(packageRemoveCmd)
	packageCmd.AddCommand(packageListCmd)

	bindCreateFlags()
	bindDeployFlags()
	bindInspectFlags()
	bindRemoveFlags()
}

func bindCreateFlags() {
	createFlags := packageCreateCmd.Flags()

	// Always require confirm flag (no viper)
	createFlags.BoolVar(&config.CommonOptions.Confirm, "confirm", false, "Confirm package creation without prompting")

	v.SetDefault(V_PKG_CREATE_SET, map[string]string{})
	v.SetDefault(V_PKG_CREATE_ZARF_CACHE, config.ZarfDefaultImageCachePath)
	v.SetDefault(V_PKG_CREATE_OUTPUT_DIRTORY, "")
	v.SetDefault(V_PKG_CREATE_SKIP_SBOM, false)
	v.SetDefault(V_PKG_CREATE_INSECURE, false)

	createFlags.StringToStringVar(&config.CreateOptions.SetVariables, "set", v.GetStringMapString(V_PKG_CREATE_SET), "Specify package variables to set on the command line (KEY=value)")
	createFlags.StringVar(&zarfImageCache, "zarf-cache", v.GetString(V_PKG_CREATE_ZARF_CACHE), "Specify the location of the Zarf image cache")
	createFlags.StringVarP(&config.CreateOptions.OutputDirectory, "output-directory", "o", v.GetString(V_PKG_CREATE_OUTPUT_DIRTORY), "Specify the output directory for the created Zarf package")
	createFlags.BoolVar(&config.CreateOptions.SkipSBOM, "skip-sbom", v.GetBool(V_PKG_CREATE_SKIP_SBOM), "Skip generating SBOM for this package")
	createFlags.BoolVar(&config.CreateOptions.Insecure, "insecure", v.GetBool(V_PKG_CREATE_INSECURE), "Allow insecure registry connections when pulling OCI images")
}

func bindDeployFlags() {
	deployFlags := packageDeployCmd.Flags()

	// Always require confirm flag (no viper)
	deployFlags.BoolVar(&config.CommonOptions.Confirm, "confirm", false, "Confirm package deployment without prompting")

	v.SetDefault(V_PKG_DEPLOY_SET, map[string]string{})
	v.SetDefault(V_PKG_DEPLOY_COMPONENTS, "")
	v.SetDefault(V_PKG_DEPLOY_INSECURE, false)
	v.SetDefault(V_PKG_DEPLOY_SHASUM, "")
	v.SetDefault(V_PKG_DEPLOY_SGET, "")

	deployFlags.StringToStringVar(&config.DeployOptions.SetVariables, "set", v.GetStringMapString(V_PKG_DEPLOY_SET), "Specify deployment variables to set on the command line (KEY=value)")
	deployFlags.StringVar(&config.DeployOptions.Components, "components", v.GetString(V_PKG_DEPLOY_COMPONENTS), "Comma-separated list of components to install.  Adding this flag will skip the init prompts for which components to install")
	deployFlags.BoolVar(&insecureDeploy, "insecure", v.GetBool(V_PKG_DEPLOY_INSECURE), "Skip shasum validation of remote package. Required if deploying a remote package and `--shasum` is not provided")
	deployFlags.StringVar(&shasum, "shasum", v.GetString(V_PKG_DEPLOY_SHASUM), "Shasum of the package to deploy. Required if deploying a remote package and `--insecure` is not provided")
	deployFlags.StringVar(&config.DeployOptions.SGetKeyPath, "sget", v.GetString(V_PKG_DEPLOY_SGET), "Path to public sget key file for remote packages signed via cosign")
}

func bindInspectFlags() {
	inspectFlags := packageInspectCmd.Flags()
	inspectFlags.BoolVarP(&packager.ViewSBOM, "sbom", "s", false, "View SBOM contents while inspecting the package")
}

func bindRemoveFlags() {
	removeFlags := packageRemoveCmd.Flags()
	removeFlags.BoolVar(&config.CommonOptions.Confirm, "confirm", false, "REQUIRED. Confirm the removal action to prevent accidental deletions")
	removeFlags.StringVar(&config.DeployOptions.Components, "components", "", "Comma-separated list of components to uninstall")
	_ = packageRemoveCmd.MarkFlagRequired("confirm")
}
