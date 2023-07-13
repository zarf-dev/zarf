// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/defenseunicorns/zarf/src/cmd/common"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/pterm/pterm"
	"oras.land/oras-go/v2/registry"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/packager"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/spf13/cobra"
	spf13viper "github.com/spf13/viper"
)

var includeInspectSBOM bool
var outputInspectSBOM string
var inspectPublicKey string

var packageCmd = &cobra.Command{
	Use:     "package",
	Aliases: []string{"p"},
	Short:   lang.CmdPackageShort,
}

var packageCreateCmd = &cobra.Command{
	Use:     "create [ DIRECTORY ]",
	Aliases: []string{"c"},
	Args:    cobra.MaximumNArgs(1),
	Short:   lang.CmdPackageCreateShort,
	Long:    lang.CmdPackageCreateLong,
	Run: func(cmd *cobra.Command, args []string) {

		var baseDir string

		// If a directory was provided, use that as the base directory
		if len(args) > 0 {
			baseDir = args[0]
		}

		var isCleanPathRegex = regexp.MustCompile(`^[a-zA-Z0-9\_\-\/\.\~\\:]+$`)
		if !isCleanPathRegex.MatchString(config.CommonOptions.CachePath) {
			message.Warnf(lang.CmdPackageCreateCleanPathErr, config.ZarfDefaultCachePath)
			config.CommonOptions.CachePath = config.ZarfDefaultCachePath
		}

		// Ensure uppercase keys from viper
		v := common.GetViper()
		viperConfig := helpers.TransformMapKeys(v.GetStringMapString(common.V_PKG_CREATE_SET), strings.ToUpper)
		pkgConfig.CreateOpts.SetVariables = helpers.MergeMap(viperConfig, pkgConfig.CreateOpts.SetVariables)

		// Configure the packager
		pkgClient := packager.NewOrDie(&pkgConfig)
		defer pkgClient.ClearTempPaths()

		// Create the package
		if err := pkgClient.Create(baseDir); err != nil {
			message.Fatalf(err, lang.CmdPackageCreateErr, err.Error())
		}
	},
}

var packageDeployCmd = &cobra.Command{
	Use:     "deploy [ PACKAGE ]",
	Aliases: []string{"d"},
	Short:   lang.CmdPackageDeployShort,
	Long:    lang.CmdPackageDeployLong,
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		pkgConfig.DeployOpts.PackagePath = choosePackage(args)

		// Ensure uppercase keys from viper and CLI --set
		v := common.GetViper()
		viperConfigSetVariables := helpers.TransformMapKeys(v.GetStringMapString(common.V_PKG_DEPLOY_SET), strings.ToUpper)
		pkgConfig.DeployOpts.SetVariables = helpers.TransformMapKeys(pkgConfig.DeployOpts.SetVariables, strings.ToUpper)

		// Merge the viper config file variables and provided CLI flag variables (CLI takes precedence))
		pkgConfig.DeployOpts.SetVariables = helpers.MergeMap(viperConfigSetVariables, pkgConfig.DeployOpts.SetVariables)

		pkgConfig.PkgSourcePath = pkgConfig.DeployOpts.PackagePath

		// Configure the packager
		pkgClient := packager.NewOrDie(&pkgConfig)
		defer pkgClient.ClearTempPaths()

		// Deploy the package
		if err := pkgClient.Deploy(); err != nil {
			message.Fatalf(err, lang.CmdPackageDeployErr, err.Error())
		}
	},
}

var packageInspectCmd = &cobra.Command{
	Use:     "inspect [ PACKAGE ]",
	Aliases: []string{"i"},
	Short:   lang.CmdPackageInspectShort,
	Long:    lang.CmdPackageInspectLong,
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		pkgConfig.DeployOpts.PackagePath = choosePackage(args)

		// Configure the packager
		pkgClient := packager.NewOrDie(&pkgConfig)
		defer pkgClient.ClearTempPaths()

		// Inspect the package
		if err := pkgClient.Inspect(includeInspectSBOM, outputInspectSBOM, inspectPublicKey); err != nil {
			message.Fatalf(err, lang.CmdPackageInspectErr, err.Error())
		}
	},
}

var packageListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"l"},
	Short:   lang.CmdPackageListShort,
	Run: func(cmd *cobra.Command, args []string) {
		// Get all the deployed packages
		deployedZarfPackages, errs := cluster.NewClusterOrDie().GetDeployedZarfPackages()
		if len(errs) > 0 && len(deployedZarfPackages) == 0 {
			message.Fatalf(errs, lang.CmdPackageListNoPackageWarn)
		}

		// Populate a pterm table of all the deployed packages
		packageTable := pterm.TableData{
			{"     Package ", "Version", "Components"},
		}

		for _, pkg := range deployedZarfPackages {
			var components []string

			for _, component := range pkg.DeployedComponents {
				components = append(components, component.Name)
			}

			packageTable = append(packageTable, pterm.TableData{{
				fmt.Sprintf("     %s", pkg.Name),
				fmt.Sprintf("%s", pkg.Data.Metadata.Version),
				fmt.Sprintf("%v", components),
			}}...)
		}

		// Print out the table for the user
		_ = pterm.DefaultTable.WithHasHeader().WithData(packageTable).Render()

		// Print out any unmarshalling errors
		if len(errs) > 0 {
			message.Fatalf(errs, lang.CmdPackageListUnmarshalErr)
		}
	},
}

var packageRemoveCmd = &cobra.Command{
	Use:     "remove { PACKAGE_NAME | PACKAGE_FILE } --confirm",
	Aliases: []string{"u"},
	Args:    cobra.ExactArgs(1),
	Short:   lang.CmdPackageRemoveShort,
	Run: func(cmd *cobra.Command, args []string) {
		pkgName := args[0]

		// Configure the packager
		pkgClient := packager.NewOrDie(&pkgConfig)
		defer pkgClient.ClearTempPaths()

		if err := pkgClient.Remove(pkgName); err != nil {
			message.Fatalf(err, lang.CmdPackageRemoveErr, err.Error())
		}
	},
}

var packagePublishCmd = &cobra.Command{
	Use:     "publish { PACKAGE | SKELETON DIRECTORY } REPOSITORY",
	Short:   lang.CmdPackagePublishShort,
	Example: lang.CmdPackagePublishExample,
	Args:    cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		pkgConfig.PublishOpts.PackagePath = choosePackage(args)

		if !utils.IsOCIURL(args[1]) {
			message.Fatal(nil, lang.CmdPackageRegistryPrefixErr)
		}
		parts := strings.Split(strings.TrimPrefix(args[1], utils.OCIURLPrefix), "/")
		ref := registry.Reference{
			Registry:   parts[0],
			Repository: strings.Join(parts[1:], "/"),
		}
		err := ref.ValidateRegistry()
		if err != nil {
			message.Fatalf(nil, "%s", err.Error())
		}

		pkgConfig.PublishOpts.PackageDestination = ref.String()

		// Configure the packager
		pkgClient := packager.NewOrDie(&pkgConfig)
		defer pkgClient.ClearTempPaths()

		// Publish the package
		if err := pkgClient.Publish(); err != nil {
			message.Fatalf(err, lang.CmdPackagePublishErr, err.Error())
		}
	},
}

var packagePullCmd = &cobra.Command{
	Use:     "pull REFERENCE",
	Short:   lang.CmdPackagePullShort,
	Example: lang.CmdPackagePullExample,
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if !utils.IsOCIURL(args[0]) {
			message.Fatal(nil, lang.CmdPackageRegistryPrefixErr)
		}

		pkgConfig.PullOpts.PackageSource = args[0]

		// Configure the packager
		pkgClient := packager.NewOrDie(&pkgConfig)
		defer pkgClient.ClearTempPaths()

		// Pull the package
		if err := pkgClient.Pull(); err != nil {
			message.Fatalf(err, lang.CmdPackagePullErr, err.Error())
		}
	},
}

func choosePackage(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	var path string
	prompt := &survey.Input{
		Message: lang.CmdPackageChoose,
		Suggest: func(toComplete string) []string {
			files, _ := filepath.Glob(config.ZarfPackagePrefix + toComplete + "*.tar")
			gzFiles, _ := filepath.Glob(config.ZarfPackagePrefix + toComplete + "*.tar.zst")
			partialFiles, _ := filepath.Glob(config.ZarfPackagePrefix + toComplete + "*.part000")

			files = append(files, gzFiles...)
			files = append(files, partialFiles...)
			return files
		},
	}

	if err := survey.AskOne(prompt, &path, survey.WithValidator(survey.Required)); err != nil {
		message.Fatalf(nil, lang.CmdPackageChooseErr, err.Error())
	}

	return path
}

func init() {
	v := common.InitViper()

	rootCmd.AddCommand(packageCmd)
	packageCmd.AddCommand(packageCreateCmd)
	packageCmd.AddCommand(packageDeployCmd)
	packageCmd.AddCommand(packageInspectCmd)
	packageCmd.AddCommand(packageRemoveCmd)
	packageCmd.AddCommand(packageListCmd)
	packageCmd.AddCommand(packagePublishCmd)
	packageCmd.AddCommand(packagePullCmd)

	bindPackageFlags(v)
	bindCreateFlags(v)
	bindDeployFlags(v)
	bindInspectFlags(v)
	bindRemoveFlags(v)
	bindPublishFlags(v)
	bindPullFlags(v)
}

func bindPackageFlags(v *spf13viper.Viper) {
	packageFlags := packageCmd.PersistentFlags()
	v.SetDefault(common.V_PKG_OCI_CONCURRENCY, 3)
	packageFlags.IntVar(&config.CommonOptions.OCIConcurrency, "oci-concurrency", v.GetInt(common.V_PKG_OCI_CONCURRENCY), lang.CmdPackageFlagConcurrency)
}

func bindCreateFlags(v *spf13viper.Viper) {
	createFlags := packageCreateCmd.Flags()

	// Always require confirm flag (no viper)
	createFlags.BoolVar(&config.CommonOptions.Confirm, "confirm", false, lang.CmdPackageCreateFlagConfirm)

	v.SetDefault(common.V_PKG_CREATE_SET, map[string]string{})
	v.SetDefault(common.V_PKG_CREATE_OUTPUT, "")
	v.SetDefault(common.V_PKG_CREATE_SBOM, false)
	v.SetDefault(common.V_PKG_CREATE_SBOM_OUTPUT, "")
	v.SetDefault(common.V_PKG_CREATE_SKIP_SBOM, false)
	v.SetDefault(common.V_PKG_CREATE_MAX_PACKAGE_SIZE, 0)
	v.SetDefault(common.V_PKG_CREATE_SIGNING_KEY, "")

	outputDirectory := v.GetString("package.create.output_directory")
	output := v.GetString(common.V_PKG_CREATE_OUTPUT)
	if outputDirectory != "" && output == "" {
		v.Set(common.V_PKG_CREATE_OUTPUT, outputDirectory)
	}
	createFlags.StringVar(&pkgConfig.CreateOpts.Output, "output-directory", v.GetString("package.create.output_directory"), lang.CmdPackageCreateFlagOutput)
	createFlags.StringVarP(&pkgConfig.CreateOpts.Output, "output", "o", v.GetString(common.V_PKG_CREATE_OUTPUT), lang.CmdPackageCreateFlagOutput)

	createFlags.StringVar(&pkgConfig.CreateOpts.DifferentialData.DifferentialPackagePath, "differential", v.GetString(common.V_PKG_CREATE_DIFFERENTIAL), lang.CmdPackageCreateFlagDifferential)
	createFlags.StringToStringVar(&pkgConfig.CreateOpts.SetVariables, "set", v.GetStringMapString(common.V_PKG_CREATE_SET), lang.CmdPackageCreateFlagSet)
	createFlags.BoolVarP(&pkgConfig.CreateOpts.ViewSBOM, "sbom", "s", v.GetBool(common.V_PKG_CREATE_SBOM), lang.CmdPackageCreateFlagSbom)
	createFlags.StringVar(&pkgConfig.CreateOpts.SBOMOutputDir, "sbom-out", v.GetString(common.V_PKG_CREATE_SBOM_OUTPUT), lang.CmdPackageCreateFlagSbomOut)
	createFlags.BoolVar(&pkgConfig.CreateOpts.SkipSBOM, "skip-sbom", v.GetBool(common.V_PKG_CREATE_SKIP_SBOM), lang.CmdPackageCreateFlagSkipSbom)
	createFlags.IntVarP(&pkgConfig.CreateOpts.MaxPackageSizeMB, "max-package-size", "m", v.GetInt(common.V_PKG_CREATE_MAX_PACKAGE_SIZE), lang.CmdPackageCreateFlagMaxPackageSize)
	createFlags.StringVarP(&pkgConfig.CreateOpts.SigningKeyPath, "key", "k", v.GetString(common.V_PKG_CREATE_SIGNING_KEY), lang.CmdPackageCreateFlagSigningKey)
	createFlags.StringVar(&pkgConfig.CreateOpts.SigningKeyPassword, "key-pass", v.GetString(common.V_PKG_CREATE_SIGNING_KEY_PASSWORD), lang.CmdPackageCreateFlagSigningKeyPassword)
	createFlags.StringToStringVar(&pkgConfig.CreateOpts.RegistryOverrides, "registry-override", v.GetStringMapString(common.V_PKG_CREATE_REGISTRY_OVERRIDE), lang.CmdPackageCreateFlagRegistryOverride)

	createFlags.MarkHidden("output-directory")
}

func bindDeployFlags(v *spf13viper.Viper) {
	deployFlags := packageDeployCmd.Flags()

	// Always require confirm flag (no viper)
	deployFlags.BoolVar(&config.CommonOptions.Confirm, "confirm", false, lang.CmdPackageDeployFlagConfirm)

	// Always require adopt-existing-resources flag (no viper)
	deployFlags.BoolVar(&pkgConfig.DeployOpts.AdoptExistingResources, "adopt-existing-resources", false, lang.CmdPackageDeployFlagAdoptExistingResources)

	v.SetDefault(common.V_PKG_DEPLOY_SET, map[string]string{})
	v.SetDefault(common.V_PKG_DEPLOY_COMPONENTS, "")
	v.SetDefault(common.V_PKG_DEPLOY_SHASUM, "")
	v.SetDefault(common.V_PKG_DEPLOY_SGET, "")
	v.SetDefault(common.V_PKG_DEPLOY_PUBLIC_KEY, "")

	deployFlags.StringToStringVar(&pkgConfig.DeployOpts.SetVariables, "set", v.GetStringMapString(common.V_PKG_DEPLOY_SET), lang.CmdPackageDeployFlagSet)
	deployFlags.StringVar(&pkgConfig.DeployOpts.Components, "components", v.GetString(common.V_PKG_DEPLOY_COMPONENTS), lang.CmdPackageDeployFlagComponents)
	deployFlags.StringVar(&pkgConfig.DeployOpts.Shasum, "shasum", v.GetString(common.V_PKG_DEPLOY_SHASUM), lang.CmdPackageDeployFlagShasum)
	deployFlags.StringVar(&pkgConfig.DeployOpts.SGetKeyPath, "sget", v.GetString(common.V_PKG_DEPLOY_SGET), lang.CmdPackageDeployFlagSget)
	deployFlags.StringVarP(&pkgConfig.DeployOpts.PublicKeyPath, "key", "k", v.GetString(common.V_PKG_DEPLOY_PUBLIC_KEY), lang.CmdPackageDeployFlagPublicKey)
}

func bindInspectFlags(v *spf13viper.Viper) {
	inspectFlags := packageInspectCmd.Flags()
	inspectFlags.BoolVarP(&includeInspectSBOM, "sbom", "s", false, lang.CmdPackageInspectFlagSbom)
	inspectFlags.StringVar(&outputInspectSBOM, "sbom-out", "", lang.CmdPackageInspectFlagSbomOut)
	inspectFlags.StringVarP(&inspectPublicKey, "key", "k", v.GetString(common.V_PKG_DEPLOY_PUBLIC_KEY), lang.CmdPackageInspectFlagPublicKey)
}

func bindRemoveFlags(v *spf13viper.Viper) {
	removeFlags := packageRemoveCmd.Flags()
	removeFlags.BoolVar(&config.CommonOptions.Confirm, "confirm", false, lang.CmdPackageRemoveFlagConfirm)
	removeFlags.StringVar(&pkgConfig.DeployOpts.Components, "components", v.GetString(common.V_PKG_DEPLOY_COMPONENTS), lang.CmdPackageRemoveFlagComponents)
	_ = packageRemoveCmd.MarkFlagRequired("confirm")
}

func bindPublishFlags(v *spf13viper.Viper) {
	publishFlags := packagePublishCmd.Flags()
	publishFlags.StringVarP(&pkgConfig.PublishOpts.SigningKeyPath, "key", "k", v.GetString(common.V_PKG_PUBLISH_SIGNING_KEY), lang.CmdPackagePublishFlagSigningKey)
	publishFlags.StringVar(&pkgConfig.PublishOpts.SigningKeyPassword, "key-pass", v.GetString(common.V_PKG_PUBLISH_SIGNING_KEY_PASSWORD), lang.CmdPackagePublishFlagSigningKeyPassword)
}

func bindPullFlags(v *spf13viper.Viper) {
	pullFlags := packagePullCmd.Flags()
	v.SetDefault(common.V_PKG_PULL_OUTPUT_DIR, "")
	pullFlags.StringVarP(&pkgConfig.PullOpts.OutputDirectory, "output-directory", "o", v.GetString(common.V_PKG_PULL_OUTPUT_DIR), lang.CmdPackagePullFlagOutputDirectory)
	pullFlags.StringVarP(&pkgConfig.PullOpts.PublicKeyPath, "key", "k", v.GetString(common.V_PKG_PULL_PUBLIC_KEY), lang.CmdPackagePullFlagPublicKey)
}
