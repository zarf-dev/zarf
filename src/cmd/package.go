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
	"github.com/defenseunicorns/zarf/src/pkg/packager/sources"
	"github.com/defenseunicorns/zarf/src/types"

	"oras.land/oras-go/v2/registry"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/packager"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

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
		pkgConfig.CreateOpts.BaseDir = common.SetBaseDirectory(args)

		var isCleanPathRegex = regexp.MustCompile(`^[a-zA-Z0-9\_\-\/\.\~\\:]+$`)
		if !isCleanPathRegex.MatchString(config.CommonOptions.CachePath) {
			message.Warnf(lang.CmdPackageCreateCleanPathErr, config.ZarfDefaultCachePath)
			config.CommonOptions.CachePath = config.ZarfDefaultCachePath
		}

		v := common.GetViper()
		pkgConfig.CreateOpts.SetVariables = helpers.TransformAndMergeMap(
			v.GetStringMapString(common.VPkgCreateSet), pkgConfig.CreateOpts.SetVariables, strings.ToUpper)

		pkgClient := packager.NewOrDie(&pkgConfig)
		defer pkgClient.ClearTempPaths()

		if err := pkgClient.Create(cmd.Context()); err != nil {
			message.Fatalf(err, lang.CmdPackageCreateErr, err.Error())
		}
	},
}

var packageDeployCmd = &cobra.Command{
	Use:     "deploy [ PACKAGE_SOURCE ]",
	Aliases: []string{"d"},
	Short:   lang.CmdPackageDeployShort,
	Long:    lang.CmdPackageDeployLong,
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		pkgConfig.PkgOpts.PackageSource = choosePackage(args)

		v := common.GetViper()
		pkgConfig.PkgOpts.SetVariables = helpers.TransformAndMergeMap(
			v.GetStringMapString(common.VPkgDeploySet), pkgConfig.PkgOpts.SetVariables, strings.ToUpper)

		pkgClient := packager.NewOrDie(&pkgConfig)
		defer pkgClient.ClearTempPaths()

		ctx := cmd.Context()

		if err := pkgClient.Deploy(ctx); err != nil {
			message.Fatalf(err, lang.CmdPackageDeployErr, err.Error())
		}
	},
}

var packageMirrorCmd = &cobra.Command{
	Use:     "mirror-resources [ PACKAGE_SOURCE ]",
	Aliases: []string{"mr"},
	Short:   lang.CmdPackageMirrorShort,
	Long:    lang.CmdPackageMirrorLong,
	Example: lang.CmdPackageMirrorExample,
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		pkgConfig.PkgOpts.PackageSource = choosePackage(args)

		pkgClient := packager.NewOrDie(&pkgConfig)
		defer pkgClient.ClearTempPaths()

		ctx := cmd.Context()

		if err := pkgClient.Mirror(ctx); err != nil {
			message.Fatalf(err, lang.CmdPackageDeployErr, err.Error())
		}
	},
}

var packageInspectCmd = &cobra.Command{
	Use:     "inspect [ PACKAGE_SOURCE ]",
	Aliases: []string{"i"},
	Short:   lang.CmdPackageInspectShort,
	Long:    lang.CmdPackageInspectLong,
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		pkgConfig.PkgOpts.PackageSource = choosePackage(args)

		src := identifyAndFallbackToClusterSource()

		pkgClient := packager.NewOrDie(&pkgConfig, packager.WithSource(src))
		defer pkgClient.ClearTempPaths()

		if err := pkgClient.Inspect(cmd.Context()); err != nil {
			message.Fatalf(err, lang.CmdPackageInspectErr, err.Error())
		}
	},
	ValidArgsFunction: getPackageCompletionArgs,
}

var packageListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"l", "ls"},
	Short:   lang.CmdPackageListShort,
	Run: func(cmd *cobra.Command, _ []string) {
		ctx := cmd.Context()
		deployedZarfPackages, err := common.NewClusterOrDie(ctx).GetDeployedZarfPackages(ctx)
		if err != nil && len(deployedZarfPackages) == 0 {
			message.Fatalf(err, lang.CmdPackageListNoPackageWarn)
		}

		// Populate a matrix of all the deployed packages
		packageData := [][]string{}

		for _, pkg := range deployedZarfPackages {
			var components []string

			for _, component := range pkg.DeployedComponents {
				components = append(components, component.Name)
			}

			packageData = append(packageData, []string{
				pkg.Name, pkg.Data.Metadata.Version, fmt.Sprintf("%v", components),
			})
		}

		header := []string{"Package", "Version", "Components"}
		message.Table(header, packageData)

		// Print out any unmarshalling errors
		if err != nil {
			message.Fatalf(err, lang.CmdPackageListUnmarshalErr)
		}
	},
}

var packageRemoveCmd = &cobra.Command{
	Use:     "remove { PACKAGE_SOURCE | PACKAGE_NAME } --confirm",
	Aliases: []string{"u", "rm"},
	Args:    cobra.MaximumNArgs(1),
	Short:   lang.CmdPackageRemoveShort,
	Run: func(cmd *cobra.Command, args []string) {
		pkgConfig.PkgOpts.PackageSource = choosePackage(args)

		src := identifyAndFallbackToClusterSource()

		pkgClient := packager.NewOrDie(&pkgConfig, packager.WithSource(src))
		defer pkgClient.ClearTempPaths()

		ctx := cmd.Context()

		if err := pkgClient.Remove(ctx); err != nil {
			message.Fatalf(err, lang.CmdPackageRemoveErr, err.Error())
		}
	},
	ValidArgsFunction: getPackageCompletionArgs,
}

var packagePublishCmd = &cobra.Command{
	Use:     "publish { PACKAGE_SOURCE | SKELETON DIRECTORY } REPOSITORY",
	Short:   lang.CmdPackagePublishShort,
	Example: lang.CmdPackagePublishExample,
	Args:    cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		pkgConfig.PkgOpts.PackageSource = args[0]

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

		if helpers.IsDir(pkgConfig.PkgOpts.PackageSource) {
			pkgConfig.CreateOpts.BaseDir = pkgConfig.PkgOpts.PackageSource
			pkgConfig.CreateOpts.IsSkeleton = true
		}

		pkgConfig.PublishOpts.PackageDestination = ref.String()

		pkgClient := packager.NewOrDie(&pkgConfig)
		defer pkgClient.ClearTempPaths()

		if err := pkgClient.Publish(cmd.Context()); err != nil {
			message.Fatalf(err, lang.CmdPackagePublishErr, err.Error())
		}
	},
}

var packagePullCmd = &cobra.Command{
	Use:     "pull PACKAGE_SOURCE",
	Short:   lang.CmdPackagePullShort,
	Example: lang.CmdPackagePullExample,
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		pkgConfig.PkgOpts.PackageSource = args[0]

		pkgClient := packager.NewOrDie(&pkgConfig)
		defer pkgClient.ClearTempPaths()

		if err := pkgClient.Pull(cmd.Context()); err != nil {
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
			zstFiles, _ := filepath.Glob(config.ZarfPackagePrefix + toComplete + "*.tar.zst")
			splitFiles, _ := filepath.Glob(config.ZarfPackagePrefix + toComplete + "*.part000")

			files = append(files, zstFiles...)
			files = append(files, splitFiles...)
			return files
		},
	}

	if err := survey.AskOne(prompt, &path, survey.WithValidator(survey.Required)); err != nil {
		message.Fatalf(nil, lang.CmdPackageChooseErr, err.Error())
	}

	return path
}

func identifyAndFallbackToClusterSource() (src sources.PackageSource) {
	var err error
	identifiedSrc := sources.Identify(pkgConfig.PkgOpts.PackageSource)
	if identifiedSrc == "" {
		message.Debugf(lang.CmdPackageClusterSourceFallback, pkgConfig.PkgOpts.PackageSource)
		src, err = sources.NewClusterSource(&pkgConfig.PkgOpts)
		if err != nil {
			message.Fatalf(err, lang.CmdPackageInvalidSource, pkgConfig.PkgOpts.PackageSource, err.Error())
		}
	}
	return src
}

func getPackageCompletionArgs(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	var pkgCandidates []string

	c, err := cluster.NewCluster()
	if err != nil {
		return pkgCandidates, cobra.ShellCompDirectiveDefault
	}

	ctx := cmd.Context()

	deployedZarfPackages, _ := c.GetDeployedZarfPackages(ctx)
	// Populate list of package names
	for _, pkg := range deployedZarfPackages {
		pkgCandidates = append(pkgCandidates, pkg.Name)
	}

	return pkgCandidates, cobra.ShellCompDirectiveDefault
}

func init() {
	v := common.InitViper()

	rootCmd.AddCommand(packageCmd)
	packageCmd.AddCommand(packageCreateCmd)
	packageCmd.AddCommand(packageDeployCmd)
	packageCmd.AddCommand(packageMirrorCmd)
	packageCmd.AddCommand(packageInspectCmd)
	packageCmd.AddCommand(packageRemoveCmd)
	packageCmd.AddCommand(packageListCmd)
	packageCmd.AddCommand(packagePublishCmd)
	packageCmd.AddCommand(packagePullCmd)

	bindPackageFlags(v)
	bindCreateFlags(v)
	bindDeployFlags(v)
	bindMirrorFlags(v)
	bindInspectFlags(v)
	bindRemoveFlags(v)
	bindPublishFlags(v)
	bindPullFlags(v)
}

func bindPackageFlags(v *viper.Viper) {
	packageFlags := packageCmd.PersistentFlags()
	packageFlags.IntVar(&config.CommonOptions.OCIConcurrency, "oci-concurrency", v.GetInt(common.VPkgOCIConcurrency), lang.CmdPackageFlagConcurrency)
	packageFlags.StringVarP(&pkgConfig.PkgOpts.PublicKeyPath, "key", "k", v.GetString(common.VPkgPublicKey), lang.CmdPackageFlagFlagPublicKey)
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

	createFlags.StringVar(&pkgConfig.CreateOpts.DifferentialPackagePath, "differential", v.GetString(common.VPkgCreateDifferential), lang.CmdPackageCreateFlagDifferential)
	createFlags.StringToStringVar(&pkgConfig.CreateOpts.SetVariables, "set", v.GetStringMapString(common.VPkgCreateSet), lang.CmdPackageCreateFlagSet)
	createFlags.BoolVarP(&pkgConfig.CreateOpts.ViewSBOM, "sbom", "s", v.GetBool(common.VPkgCreateSbom), lang.CmdPackageCreateFlagSbom)
	createFlags.StringVar(&pkgConfig.CreateOpts.SBOMOutputDir, "sbom-out", v.GetString(common.VPkgCreateSbomOutput), lang.CmdPackageCreateFlagSbomOut)
	createFlags.BoolVar(&pkgConfig.CreateOpts.SkipSBOM, "skip-sbom", v.GetBool(common.VPkgCreateSkipSbom), lang.CmdPackageCreateFlagSkipSbom)
	createFlags.IntVarP(&pkgConfig.CreateOpts.MaxPackageSizeMB, "max-package-size", "m", v.GetInt(common.VPkgCreateMaxPackageSize), lang.CmdPackageCreateFlagMaxPackageSize)
	createFlags.StringToStringVar(&pkgConfig.CreateOpts.RegistryOverrides, "registry-override", v.GetStringMapString(common.VPkgCreateRegistryOverride), lang.CmdPackageCreateFlagRegistryOverride)
	createFlags.StringVarP(&pkgConfig.CreateOpts.Flavor, "flavor", "f", v.GetString(common.VPkgCreateFlavor), lang.CmdPackageCreateFlagFlavor)

	createFlags.StringVar(&pkgConfig.CreateOpts.SigningKeyPath, "signing-key", v.GetString(common.VPkgCreateSigningKey), lang.CmdPackageCreateFlagSigningKey)
	createFlags.StringVar(&pkgConfig.CreateOpts.SigningKeyPassword, "signing-key-pass", v.GetString(common.VPkgCreateSigningKeyPassword), lang.CmdPackageCreateFlagSigningKeyPassword)

	createFlags.StringVarP(&pkgConfig.CreateOpts.SigningKeyPath, "key", "k", v.GetString(common.VPkgCreateSigningKey), lang.CmdPackageCreateFlagDeprecatedKey)
	createFlags.StringVar(&pkgConfig.CreateOpts.SigningKeyPassword, "key-pass", v.GetString(common.VPkgCreateSigningKeyPassword), lang.CmdPackageCreateFlagDeprecatedKeyPassword)

	createFlags.IntVar(&pkgConfig.PkgOpts.Retries, "retries", v.GetInt(common.VPkgRetries), lang.CmdPackageFlagRetries)

	err := createFlags.MarkHidden("output-directory")
	if err != nil {
		message.Fatal(err, err.Error())
	}
	err = createFlags.MarkHidden("key")
	if err != nil {
		message.Fatal(err, err.Error())
	}
	err = createFlags.MarkHidden("key-pass")
	if err != nil {
		message.Fatal(err, err.Error())
	}
}

func bindDeployFlags(v *viper.Viper) {
	deployFlags := packageDeployCmd.Flags()

	// Always require confirm flag (no viper)
	deployFlags.BoolVar(&config.CommonOptions.Confirm, "confirm", false, lang.CmdPackageDeployFlagConfirm)

	// Always require adopt-existing-resources flag (no viper)
	deployFlags.BoolVar(&pkgConfig.DeployOpts.AdoptExistingResources, "adopt-existing-resources", false, lang.CmdPackageDeployFlagAdoptExistingResources)
	deployFlags.BoolVar(&pkgConfig.DeployOpts.SkipWebhooks, "skip-webhooks", v.GetBool(common.VPkgDeploySkipWebhooks), lang.CmdPackageDeployFlagSkipWebhooks)
	deployFlags.DurationVar(&pkgConfig.DeployOpts.Timeout, "timeout", v.GetDuration(common.VPkgDeployTimeout), lang.CmdPackageDeployFlagTimeout)

	deployFlags.IntVar(&pkgConfig.PkgOpts.Retries, "retries", v.GetInt(common.VPkgRetries), lang.CmdPackageFlagRetries)
	deployFlags.StringToStringVar(&pkgConfig.PkgOpts.SetVariables, "set", v.GetStringMapString(common.VPkgDeploySet), lang.CmdPackageDeployFlagSet)
	deployFlags.StringVar(&pkgConfig.PkgOpts.OptionalComponents, "components", v.GetString(common.VPkgDeployComponents), lang.CmdPackageDeployFlagComponents)
	deployFlags.StringVar(&pkgConfig.PkgOpts.Shasum, "shasum", v.GetString(common.VPkgDeployShasum), lang.CmdPackageDeployFlagShasum)
	deployFlags.StringVar(&pkgConfig.PkgOpts.SGetKeyPath, "sget", v.GetString(common.VPkgDeploySget), lang.CmdPackageDeployFlagSget)

	err := deployFlags.MarkHidden("sget")
	if err != nil {
		message.Fatal(err, err.Error())
	}
}

func bindMirrorFlags(v *viper.Viper) {
	mirrorFlags := packageMirrorCmd.Flags()

	// Init package variable defaults that are non-zero values
	// NOTE: these are not in common.setDefaults so that zarf tools update-creds does not erroneously update values back to the default
	v.SetDefault(common.VInitGitPushUser, types.ZarfGitPushUser)
	v.SetDefault(common.VInitRegistryPushUser, types.ZarfRegistryPushUser)

	// Always require confirm flag (no viper)
	mirrorFlags.BoolVar(&config.CommonOptions.Confirm, "confirm", false, lang.CmdPackageDeployFlagConfirm)

	mirrorFlags.BoolVar(&pkgConfig.MirrorOpts.NoImgChecksum, "no-img-checksum", false, lang.CmdPackageMirrorFlagNoChecksum)

	mirrorFlags.IntVar(&pkgConfig.PkgOpts.Retries, "retries", v.GetInt(common.VPkgRetries), lang.CmdPackageFlagRetries)
	mirrorFlags.StringVar(&pkgConfig.PkgOpts.OptionalComponents, "components", v.GetString(common.VPkgDeployComponents), lang.CmdPackageMirrorFlagComponents)

	// Flags for using an external Git server
	mirrorFlags.StringVar(&pkgConfig.InitOpts.GitServer.Address, "git-url", v.GetString(common.VInitGitURL), lang.CmdInitFlagGitURL)
	mirrorFlags.StringVar(&pkgConfig.InitOpts.GitServer.PushUsername, "git-push-username", v.GetString(common.VInitGitPushUser), lang.CmdInitFlagGitPushUser)
	mirrorFlags.StringVar(&pkgConfig.InitOpts.GitServer.PushPassword, "git-push-password", v.GetString(common.VInitGitPushPass), lang.CmdInitFlagGitPushPass)

	// Flags for using an external registry
	mirrorFlags.StringVar(&pkgConfig.InitOpts.RegistryInfo.Address, "registry-url", v.GetString(common.VInitRegistryURL), lang.CmdInitFlagRegURL)
	mirrorFlags.StringVar(&pkgConfig.InitOpts.RegistryInfo.PushUsername, "registry-push-username", v.GetString(common.VInitRegistryPushUser), lang.CmdInitFlagRegPushUser)
	mirrorFlags.StringVar(&pkgConfig.InitOpts.RegistryInfo.PushPassword, "registry-push-password", v.GetString(common.VInitRegistryPushPass), lang.CmdInitFlagRegPushPass)
}

func bindInspectFlags(_ *viper.Viper) {
	inspectFlags := packageInspectCmd.Flags()
	inspectFlags.BoolVarP(&pkgConfig.InspectOpts.ViewSBOM, "sbom", "s", false, lang.CmdPackageInspectFlagSbom)
	inspectFlags.StringVar(&pkgConfig.InspectOpts.SBOMOutputDir, "sbom-out", "", lang.CmdPackageInspectFlagSbomOut)
	inspectFlags.BoolVar(&pkgConfig.InspectOpts.ListImages, "list-images", false, lang.CmdPackageInspectFlagListImages)
}

func bindRemoveFlags(v *viper.Viper) {
	removeFlags := packageRemoveCmd.Flags()
	removeFlags.BoolVar(&config.CommonOptions.Confirm, "confirm", false, lang.CmdPackageRemoveFlagConfirm)
	removeFlags.StringVar(&pkgConfig.PkgOpts.OptionalComponents, "components", v.GetString(common.VPkgDeployComponents), lang.CmdPackageRemoveFlagComponents)
	_ = packageRemoveCmd.MarkFlagRequired("confirm")
}

func bindPublishFlags(v *viper.Viper) {
	publishFlags := packagePublishCmd.Flags()
	publishFlags.StringVar(&pkgConfig.PublishOpts.SigningKeyPath, "signing-key", v.GetString(common.VPkgPublishSigningKey), lang.CmdPackagePublishFlagSigningKey)
	publishFlags.StringVar(&pkgConfig.PublishOpts.SigningKeyPassword, "signing-key-pass", v.GetString(common.VPkgPublishSigningKeyPassword), lang.CmdPackagePublishFlagSigningKeyPassword)
}

func bindPullFlags(v *viper.Viper) {
	pullFlags := packagePullCmd.Flags()
	pullFlags.StringVarP(&pkgConfig.PullOpts.OutputDirectory, "output-directory", "o", v.GetString(common.VPkgPullOutputDir), lang.CmdPackagePullFlagOutputDirectory)
}
