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

	"github.com/pterm/pterm"
	"oras.land/oras-go/v2/registry"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/packager"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
		pkgConfig.CreateOpts.SetVariables = helpers.TransformAndMergeMap(
			v.GetStringMapString(common.VPkgCreateSet), pkgConfig.CreateOpts.SetVariables, strings.ToUpper)

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
		pkgConfig.PkgOpts.PackagePath = choosePackage(args)

		// Ensure uppercase keys from viper and CLI --set
		v := common.GetViper()

		// Merge the viper config file variables and provided CLI flag variables (CLI takes precedence))
		pkgConfig.PkgOpts.SetVariables = helpers.TransformAndMergeMap(
			v.GetStringMapString(common.VPkgDeploySet), pkgConfig.PkgOpts.SetVariables, strings.ToUpper)

		pkgConfig.PkgSource = pkgConfig.PkgOpts.PackagePath

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
		pkgConfig.PkgOpts.PackagePath = choosePackage(args)

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
				pkg.Data.Metadata.Version,
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
		pkgConfig.PkgOpts.PackagePath = args[0]

		// Configure the packager
		pkgClient := packager.NewOrDie(&pkgConfig)
		defer pkgClient.ClearTempPaths()

		if err := pkgClient.Remove(); err != nil {
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

		if !helpers.IsOCIURL(args[1]) {
			message.Fatal(nil, lang.CmdPackageRegistryPrefixErr)
		}
		parts := strings.Split(strings.TrimPrefix(args[1], helpers.OCIURLPrefix), "/")
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
		if !helpers.IsOCIURL(args[0]) {
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

func bindPackageFlags(v *viper.Viper) {
	packageFlags := packageCmd.PersistentFlags()
	v.SetDefault(common.VPkgOCIConcurrency, 3)
	packageFlags.IntVar(&config.CommonOptions.OCIConcurrency, "oci-concurrency", v.GetInt(common.VPkgOCIConcurrency), lang.CmdPackageFlagConcurrency)
}

func bindCreateFlags(v *viper.Viper) {
	createFlags := packageCreateCmd.Flags()

	// Always require confirm flag (no viper)
	createFlags.BoolVar(&config.CommonOptions.Confirm, "confirm", false, lang.CmdPackageCreateFlagConfirm)

	outputDirectory := v.GetString("package.create.output_directory")
	output := v.GetString(common.VPkgCreateOutput)
	if outputDirectory != "" && output == "" {
		v.Set(common.VPkgCreateOutput, outputDirectory)
	}
	createFlags.StringVar(&pkgConfig.CreateOpts.Output, "output-directory", v.GetString("package.create.output_directory"), lang.CmdPackageCreateFlagOutput)
	createFlags.StringVarP(&pkgConfig.CreateOpts.Output, "output", "o", v.GetString(common.VPkgCreateOutput), lang.CmdPackageCreateFlagOutput)

	createFlags.StringVar(&pkgConfig.CreateOpts.DifferentialData.DifferentialPackagePath, "differential", v.GetString(common.VPkgCreateDifferential), lang.CmdPackageCreateFlagDifferential)
	createFlags.StringToStringVar(&pkgConfig.CreateOpts.SetVariables, "set", v.GetStringMapString(common.VPkgCreateSet), lang.CmdPackageCreateFlagSet)
	createFlags.BoolVarP(&pkgConfig.CreateOpts.ViewSBOM, "sbom", "s", v.GetBool(common.VPkgCreateSbom), lang.CmdPackageCreateFlagSbom)
	createFlags.StringVar(&pkgConfig.CreateOpts.SBOMOutputDir, "sbom-out", v.GetString(common.VPkgCreateSbomOutput), lang.CmdPackageCreateFlagSbomOut)
	createFlags.BoolVar(&pkgConfig.CreateOpts.SkipSBOM, "skip-sbom", v.GetBool(common.VPkgCreateSkipSbom), lang.CmdPackageCreateFlagSkipSbom)
	createFlags.IntVarP(&pkgConfig.CreateOpts.MaxPackageSizeMB, "max-package-size", "m", v.GetInt(common.VPkgCreateMaxPackageSize), lang.CmdPackageCreateFlagMaxPackageSize)
	createFlags.StringVarP(&pkgConfig.CreateOpts.SigningKeyPath, "key", "k", v.GetString(common.VPkgCreateSigningKey), lang.CmdPackageCreateFlagSigningKey)
	createFlags.StringVar(&pkgConfig.CreateOpts.SigningKeyPassword, "key-pass", v.GetString(common.VPkgCreateSigningKeyPassword), lang.CmdPackageCreateFlagSigningKeyPassword)
	createFlags.StringToStringVar(&pkgConfig.CreateOpts.RegistryOverrides, "registry-override", v.GetStringMapString(common.VPkgCreateRegistryOverride), lang.CmdPackageCreateFlagRegistryOverride)

	createFlags.MarkHidden("output-directory")
}

func bindDeployFlags(v *viper.Viper) {
	deployFlags := packageDeployCmd.Flags()

	// Always require confirm flag (no viper)
	deployFlags.BoolVar(&config.CommonOptions.Confirm, "confirm", false, lang.CmdPackageDeployFlagConfirm)

	// Always require adopt-existing-resources flag (no viper)
	deployFlags.BoolVar(&pkgConfig.DeployOpts.AdoptExistingResources, "adopt-existing-resources", false, lang.CmdPackageDeployFlagAdoptExistingResources)

	deployFlags.StringToStringVar(&pkgConfig.PkgOpts.SetVariables, "set", v.GetStringMapString(common.VPkgDeploySet), lang.CmdPackageDeployFlagSet)
	deployFlags.StringVar(&pkgConfig.PkgOpts.OptionalComponents, "components", v.GetString(common.VPkgDeployComponents), lang.CmdPackageDeployFlagComponents)
	deployFlags.StringVar(&pkgConfig.PkgOpts.Shasum, "shasum", v.GetString(common.VPkgDeployShasum), lang.CmdPackageDeployFlagShasum)
	deployFlags.StringVar(&pkgConfig.PkgOpts.SGetKeyPath, "sget", v.GetString(common.VPkgDeploySget), lang.CmdPackageDeployFlagSget)
	deployFlags.StringVarP(&pkgConfig.PkgOpts.PublicKeyPath, "key", "k", v.GetString(common.VPkgDeployPublicKey), lang.CmdPackageDeployFlagPublicKey)
	deployFlags.BoolVar(&pkgConfig.PkgOpts.SkipWebhooks, "skip-webhooks", v.GetBool(common.VPkgDeploySkipWebhooks), lang.CmdPackageDeployFlagSkipWebhooks)

	deployFlags.MarkHidden("sget")
}

func bindInspectFlags(v *viper.Viper) {
	inspectFlags := packageInspectCmd.Flags()
	inspectFlags.BoolVarP(&includeInspectSBOM, "sbom", "s", false, lang.CmdPackageInspectFlagSbom)
	inspectFlags.StringVar(&outputInspectSBOM, "sbom-out", "", lang.CmdPackageInspectFlagSbomOut)
	inspectFlags.StringVarP(&inspectPublicKey, "key", "k", v.GetString(common.VPkgDeployPublicKey), lang.CmdPackageInspectFlagPublicKey)
}

func bindRemoveFlags(v *viper.Viper) {
	removeFlags := packageRemoveCmd.Flags()
	removeFlags.BoolVar(&config.CommonOptions.Confirm, "confirm", false, lang.CmdPackageRemoveFlagConfirm)
	removeFlags.StringVar(&pkgConfig.PkgOpts.OptionalComponents, "components", v.GetString(common.VPkgDeployComponents), lang.CmdPackageRemoveFlagComponents)

	_ = packageRemoveCmd.MarkFlagRequired("confirm")
}

func bindPublishFlags(v *viper.Viper) {
	publishFlags := packagePublishCmd.Flags()
	publishFlags.StringVarP(&pkgConfig.PublishOpts.SigningKeyPath, "key", "k", v.GetString(common.VPkgPublishSigningKey), lang.CmdPackagePublishFlagSigningKey)
	publishFlags.StringVar(&pkgConfig.PublishOpts.SigningKeyPassword, "key-pass", v.GetString(common.VPkgPublishSigningKeyPassword), lang.CmdPackagePublishFlagSigningKeyPassword)
}

func bindPullFlags(v *viper.Viper) {
	pullFlags := packagePullCmd.Flags()
	pullFlags.StringVarP(&pkgConfig.PullOpts.OutputDirectory, "output-directory", "o", v.GetString(common.VPkgPullOutputDir), lang.CmdPackagePullFlagOutputDirectory)
	pullFlags.StringVarP(&pkgConfig.PullOpts.PublicKeyPath, "key", "k", v.GetString(common.VPkgPullPublicKey), lang.CmdPackagePullFlagPublicKey)
}
