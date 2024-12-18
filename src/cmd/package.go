// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"oras.land/oras-go/v2/registry"

	"github.com/zarf-dev/zarf/src/cmd/common"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/internal/dns"
	"github.com/zarf-dev/zarf/src/internal/packager2"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/packager"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/types"
)

// NewPackageCommand creates the `package` sub-command and its nested children.
func NewPackageCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "package",
		Aliases: []string{"p"},
		Short:   lang.CmdPackageShort,
	}

	v := common.GetViper()

	persistentFlags := cmd.PersistentFlags()
	persistentFlags.IntVar(&config.CommonOptions.OCIConcurrency, "oci-concurrency", v.GetInt(common.VPkgOCIConcurrency), lang.CmdPackageFlagConcurrency)
	persistentFlags.StringVarP(&pkgConfig.PkgOpts.PublicKeyPath, "key", "k", v.GetString(common.VPkgPublicKey), lang.CmdPackageFlagFlagPublicKey)

	cmd.AddCommand(NewPackageCreateCommand(v))
	cmd.AddCommand(NewPackageDeployCommand(v))
	cmd.AddCommand(NewPackageMirrorResourcesCommand(v))
	cmd.AddCommand(NewPackageInspectCommand())
	cmd.AddCommand(NewPackageRemoveCommand(v))
	cmd.AddCommand(NewPackageListCommand())
	cmd.AddCommand(NewPackagePublishCommand(v))
	cmd.AddCommand(NewPackagePullCommand(v))

	return cmd
}

// PackageCreateOptions holds the command-line options for 'package create' sub-command.
type PackageCreateOptions struct{}

// NewPackageCreateCommand creates the `package create` sub-command.
func NewPackageCreateCommand(v *viper.Viper) *cobra.Command {
	o := &PackageCreateOptions{}

	cmd := &cobra.Command{
		Use:     "create [ DIRECTORY ]",
		Aliases: []string{"c"},
		Args:    cobra.MaximumNArgs(1),
		Short:   lang.CmdPackageCreateShort,
		Long:    lang.CmdPackageCreateLong,
		RunE:    o.Run,
	}

	// Always require confirm flag (no viper)
	cmd.Flags().BoolVar(&config.CommonOptions.Confirm, "confirm", false, lang.CmdPackageCreateFlagConfirm)

	outputDirectory := v.GetString("package.create.output_directory")
	output := v.GetString(common.VPkgCreateOutput)
	if outputDirectory != "" && output == "" {
		v.Set(common.VPkgCreateOutput, outputDirectory)
	}
	cmd.Flags().StringVar(&pkgConfig.CreateOpts.Output, "output-directory", v.GetString("package.create.output_directory"), lang.CmdPackageCreateFlagOutput)
	cmd.Flags().StringVarP(&pkgConfig.CreateOpts.Output, "output", "o", v.GetString(common.VPkgCreateOutput), lang.CmdPackageCreateFlagOutput)

	cmd.Flags().StringVar(&pkgConfig.CreateOpts.DifferentialPackagePath, "differential", v.GetString(common.VPkgCreateDifferential), lang.CmdPackageCreateFlagDifferential)
	cmd.Flags().StringToStringVar(&pkgConfig.CreateOpts.SetVariables, "set", v.GetStringMapString(common.VPkgCreateSet), lang.CmdPackageCreateFlagSet)
	cmd.Flags().BoolVarP(&pkgConfig.CreateOpts.ViewSBOM, "sbom", "s", v.GetBool(common.VPkgCreateSbom), lang.CmdPackageCreateFlagSbom)
	cmd.Flags().StringVar(&pkgConfig.CreateOpts.SBOMOutputDir, "sbom-out", v.GetString(common.VPkgCreateSbomOutput), lang.CmdPackageCreateFlagSbomOut)
	cmd.Flags().BoolVar(&pkgConfig.CreateOpts.SkipSBOM, "skip-sbom", v.GetBool(common.VPkgCreateSkipSbom), lang.CmdPackageCreateFlagSkipSbom)
	cmd.Flags().IntVarP(&pkgConfig.CreateOpts.MaxPackageSizeMB, "max-package-size", "m", v.GetInt(common.VPkgCreateMaxPackageSize), lang.CmdPackageCreateFlagMaxPackageSize)
	cmd.Flags().StringToStringVar(&pkgConfig.CreateOpts.RegistryOverrides, "registry-override", v.GetStringMapString(common.VPkgCreateRegistryOverride), lang.CmdPackageCreateFlagRegistryOverride)
	cmd.Flags().StringVarP(&pkgConfig.CreateOpts.Flavor, "flavor", "f", v.GetString(common.VPkgCreateFlavor), lang.CmdPackageCreateFlagFlavor)

	cmd.Flags().StringVar(&pkgConfig.CreateOpts.SigningKeyPath, "signing-key", v.GetString(common.VPkgCreateSigningKey), lang.CmdPackageCreateFlagSigningKey)
	cmd.Flags().StringVar(&pkgConfig.CreateOpts.SigningKeyPassword, "signing-key-pass", v.GetString(common.VPkgCreateSigningKeyPassword), lang.CmdPackageCreateFlagSigningKeyPassword)

	cmd.Flags().StringVarP(&pkgConfig.CreateOpts.SigningKeyPath, "key", "k", v.GetString(common.VPkgCreateSigningKey), lang.CmdPackageCreateFlagDeprecatedKey)
	cmd.Flags().StringVar(&pkgConfig.CreateOpts.SigningKeyPassword, "key-pass", v.GetString(common.VPkgCreateSigningKeyPassword), lang.CmdPackageCreateFlagDeprecatedKeyPassword)

	cmd.Flags().IntVar(&pkgConfig.PkgOpts.Retries, "retries", v.GetInt(common.VPkgRetries), lang.CmdPackageFlagRetries)

	errOD := cmd.Flags().MarkHidden("output-directory")
	if errOD != nil {
		logger.Default().Debug("unable to mark flag output-directory", "error", errOD)
	}
	errKey := cmd.Flags().MarkHidden("key")
	if errKey != nil {
		logger.Default().Debug("unable to mark flag key", "error", errKey)
	}
	errKP := cmd.Flags().MarkHidden("key-pass")
	if errKP != nil {
		logger.Default().Debug("unable to mark flag key-pass", "error", errKP)
	}

	return cmd
}

// Run performs the execution of 'package create' sub-command.
func (o *PackageCreateOptions) Run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	l := logger.From(ctx)
	pkgConfig.CreateOpts.BaseDir = setBaseDirectory(args)

	var isCleanPathRegex = regexp.MustCompile(`^[a-zA-Z0-9\_\-\/\.\~\\:]+$`)
	if !isCleanPathRegex.MatchString(config.CommonOptions.CachePath) {
		// TODO(mkcp): Remove message on logger release
		message.Warnf(lang.CmdPackageCreateCleanPathErr, config.ZarfDefaultCachePath)
		l.Warn("invalid characters in Zarf cache path, using default", "cfg", config.ZarfDefaultCachePath, "default", config.ZarfDefaultCachePath)
		config.CommonOptions.CachePath = config.ZarfDefaultCachePath
	}

	v := common.GetViper()
	pkgConfig.CreateOpts.SetVariables = helpers.TransformAndMergeMap(
		v.GetStringMapString(common.VPkgCreateSet), pkgConfig.CreateOpts.SetVariables, strings.ToUpper)

	pkgClient, err := packager.New(&pkgConfig,
		packager.WithContext(ctx),
	)
	if err != nil {
		return err
	}
	defer pkgClient.ClearTempPaths()

	err = pkgClient.Create(ctx)

	// NOTE(mkcp): LintErrors are rendered with a table
	var lintErr *lint.LintError
	if errors.As(err, &lintErr) {
		common.PrintFindings(ctx, lintErr)
	}
	if err != nil {
		return fmt.Errorf("failed to create package: %w", err)
	}
	return nil
}

// PackageDeployOptions holds the command-line options for 'package deploy' sub-command.
type PackageDeployOptions struct{}

// NewPackageDeployCommand creates the `package deploy` sub-command.
func NewPackageDeployCommand(v *viper.Viper) *cobra.Command {
	o := &PackageDeployOptions{}

	cmd := &cobra.Command{
		Use:     "deploy [ PACKAGE_SOURCE ]",
		Aliases: []string{"d"},
		Short:   lang.CmdPackageDeployShort,
		Long:    lang.CmdPackageDeployLong,
		Args:    cobra.MaximumNArgs(1),
		PreRun:  o.PreRun,
		RunE:    o.Run,
	}

	// Always require confirm flag (no viper)
	cmd.Flags().BoolVar(&config.CommonOptions.Confirm, "confirm", false, lang.CmdPackageDeployFlagConfirm)

	// Always require adopt-existing-resources flag (no viper)
	cmd.Flags().BoolVar(&pkgConfig.DeployOpts.AdoptExistingResources, "adopt-existing-resources", false, lang.CmdPackageDeployFlagAdoptExistingResources)
	cmd.Flags().DurationVar(&pkgConfig.DeployOpts.Timeout, "timeout", v.GetDuration(common.VPkgDeployTimeout), lang.CmdPackageDeployFlagTimeout)

	cmd.Flags().IntVar(&pkgConfig.PkgOpts.Retries, "retries", v.GetInt(common.VPkgRetries), lang.CmdPackageFlagRetries)
	cmd.Flags().StringToStringVar(&pkgConfig.PkgOpts.SetVariables, "set", v.GetStringMapString(common.VPkgDeploySet), lang.CmdPackageDeployFlagSet)
	cmd.Flags().StringVar(&pkgConfig.PkgOpts.OptionalComponents, "components", v.GetString(common.VPkgDeployComponents), lang.CmdPackageDeployFlagComponents)
	cmd.Flags().StringVar(&pkgConfig.PkgOpts.Shasum, "shasum", v.GetString(common.VPkgDeployShasum), lang.CmdPackageDeployFlagShasum)
	cmd.Flags().StringVar(&pkgConfig.PkgOpts.SGetKeyPath, "sget", v.GetString(common.VPkgDeploySget), lang.CmdPackageDeployFlagSget)
	cmd.Flags().BoolVar(&pkgConfig.PkgOpts.SkipSignatureValidation, "skip-signature-validation", false, lang.CmdPackageFlagSkipSignatureValidation)

	err := cmd.Flags().MarkHidden("sget")
	if err != nil {
		logger.Default().Debug("unable to mark flag sget", "error", err)
	}

	return cmd
}

// PreRun performs the pre-run checks for 'package deploy' sub-command.
func (o *PackageDeployOptions) PreRun(_ *cobra.Command, _ []string) {
	// If --insecure was provided, set --skip-signature-validation to match
	if config.CommonOptions.Insecure {
		pkgConfig.PkgOpts.SkipSignatureValidation = true
	}
}

// Run performs the execution of 'package deploy' sub-command.
func (o *PackageDeployOptions) Run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	packageSource, err := choosePackage(ctx, args)
	if err != nil {
		return err
	}
	pkgConfig.PkgOpts.PackageSource = packageSource

	v := common.GetViper()
	pkgConfig.PkgOpts.SetVariables = helpers.TransformAndMergeMap(
		v.GetStringMapString(common.VPkgDeploySet), pkgConfig.PkgOpts.SetVariables, strings.ToUpper)

	pkgClient, err := packager.New(&pkgConfig, packager.WithContext(cmd.Context()))
	if err != nil {
		return err
	}
	defer pkgClient.ClearTempPaths()

	if err := pkgClient.Deploy(ctx); err != nil {
		return fmt.Errorf("failed to deploy package: %w", err)
	}
	return nil
}

// PackageMirrorResourcesOptions holds the command-line options for 'package mirror-resources' sub-command.
type PackageMirrorResourcesOptions struct{}

// NewPackageMirrorResourcesCommand creates the `package mirror-resources` sub-command.
func NewPackageMirrorResourcesCommand(v *viper.Viper) *cobra.Command {
	o := &PackageMirrorResourcesOptions{}

	cmd := &cobra.Command{
		Use:     "mirror-resources [ PACKAGE_SOURCE ]",
		Aliases: []string{"mr"},
		Short:   lang.CmdPackageMirrorShort,
		Long:    lang.CmdPackageMirrorLong,
		Example: lang.CmdPackageMirrorExample,
		Args:    cobra.MaximumNArgs(1),
		PreRun:  o.PreRun,
		RunE:    o.Run,
	}

	// Init package variable defaults that are non-zero values
	// NOTE: these are not in common.setDefaults so that zarf tools update-creds does not erroneously update values back to the default
	v.SetDefault(common.VInitGitPushUser, types.ZarfGitPushUser)
	v.SetDefault(common.VInitRegistryPushUser, types.ZarfRegistryPushUser)

	// Always require confirm flag (no viper)
	cmd.Flags().BoolVar(&config.CommonOptions.Confirm, "confirm", false, lang.CmdPackageDeployFlagConfirm)

	cmd.Flags().StringVar(&pkgConfig.PkgOpts.Shasum, "shasum", "", lang.CmdPackagePullFlagShasum)
	cmd.Flags().BoolVar(&pkgConfig.MirrorOpts.NoImgChecksum, "no-img-checksum", false, lang.CmdPackageMirrorFlagNoChecksum)
	cmd.Flags().BoolVar(&pkgConfig.PkgOpts.SkipSignatureValidation, "skip-signature-validation", false, lang.CmdPackageFlagSkipSignatureValidation)

	cmd.Flags().IntVar(&pkgConfig.PkgOpts.Retries, "retries", v.GetInt(common.VPkgRetries), lang.CmdPackageFlagRetries)
	cmd.Flags().StringVar(&pkgConfig.PkgOpts.OptionalComponents, "components", v.GetString(common.VPkgDeployComponents), lang.CmdPackageMirrorFlagComponents)

	// Flags for using an external Git server
	cmd.Flags().StringVar(&pkgConfig.InitOpts.GitServer.Address, "git-url", v.GetString(common.VInitGitURL), lang.CmdInitFlagGitURL)
	cmd.Flags().StringVar(&pkgConfig.InitOpts.GitServer.PushUsername, "git-push-username", v.GetString(common.VInitGitPushUser), lang.CmdInitFlagGitPushUser)
	cmd.Flags().StringVar(&pkgConfig.InitOpts.GitServer.PushPassword, "git-push-password", v.GetString(common.VInitGitPushPass), lang.CmdInitFlagGitPushPass)

	// Flags for using an external registry
	cmd.Flags().StringVar(&pkgConfig.InitOpts.RegistryInfo.Address, "registry-url", v.GetString(common.VInitRegistryURL), lang.CmdInitFlagRegURL)
	cmd.Flags().StringVar(&pkgConfig.InitOpts.RegistryInfo.PushUsername, "registry-push-username", v.GetString(common.VInitRegistryPushUser), lang.CmdInitFlagRegPushUser)
	cmd.Flags().StringVar(&pkgConfig.InitOpts.RegistryInfo.PushPassword, "registry-push-password", v.GetString(common.VInitRegistryPushPass), lang.CmdInitFlagRegPushPass)

	return cmd
}

// PreRun performs the pre-run checks for 'package mirror-resources' sub-command.
func (o *PackageMirrorResourcesOptions) PreRun(_ *cobra.Command, _ []string) {
	// If --insecure was provided, set --skip-signature-validation to match
	if config.CommonOptions.Insecure {
		pkgConfig.PkgOpts.SkipSignatureValidation = true
	}
}

// Run performs the execution of 'package mirror-resources' sub-command.
func (o *PackageMirrorResourcesOptions) Run(cmd *cobra.Command, args []string) (err error) {
	ctx := cmd.Context()
	var c *cluster.Cluster
	if dns.IsServiceURL(pkgConfig.InitOpts.RegistryInfo.Address) || dns.IsServiceURL(pkgConfig.InitOpts.GitServer.Address) {
		var err error
		c, err = cluster.NewCluster()
		if err != nil {
			return err
		}
	}
	src, err := choosePackage(ctx, args)
	if err != nil {
		return err
	}
	filter := filters.Combine(
		filters.ByLocalOS(runtime.GOOS),
		filters.BySelectState(pkgConfig.PkgOpts.OptionalComponents),
	)

	loadOpt := packager2.LoadOptions{
		Source:                  src,
		Shasum:                  pkgConfig.PkgOpts.Shasum,
		PublicKeyPath:           pkgConfig.PkgOpts.PublicKeyPath,
		SkipSignatureValidation: pkgConfig.PkgOpts.SkipSignatureValidation,
		Filter:                  filter,
	}
	pkgLayout, err := packager2.LoadPackage(cmd.Context(), loadOpt)
	if err != nil {
		return err
	}
	defer func() {
		// Cleanup package files
		err = errors.Join(err, pkgLayout.Cleanup())
	}()

	mirrorOpt := packager2.MirrorOptions{
		Cluster:         c,
		PkgLayout:       pkgLayout,
		Filter:          filter,
		RegistryInfo:    pkgConfig.InitOpts.RegistryInfo,
		GitInfo:         pkgConfig.InitOpts.GitServer,
		NoImageChecksum: pkgConfig.MirrorOpts.NoImgChecksum,
		Retries:         pkgConfig.PkgOpts.Retries,
	}
	err = packager2.Mirror(ctx, mirrorOpt)
	if err != nil {
		return err
	}
	return nil
}

// PackageInspectOptions holds the command-line options for 'package inspect' sub-command.
type PackageInspectOptions struct{}

// NewPackageInspectCommand creates the `package inspect` sub-command.
func NewPackageInspectCommand() *cobra.Command {
	o := &PackageInspectOptions{}
	cmd := &cobra.Command{
		Use:     "inspect [ PACKAGE_SOURCE ]",
		Aliases: []string{"i"},
		Short:   lang.CmdPackageInspectShort,
		Long:    lang.CmdPackageInspectLong,
		Args:    cobra.MaximumNArgs(1),
		PreRun:  o.PreRun,
		RunE:    o.Run,
	}

	cmd.Flags().BoolVarP(&pkgConfig.InspectOpts.ViewSBOM, "sbom", "s", false, lang.CmdPackageInspectFlagSbom)
	cmd.Flags().StringVar(&pkgConfig.InspectOpts.SBOMOutputDir, "sbom-out", "", lang.CmdPackageInspectFlagSbomOut)
	cmd.Flags().BoolVar(&pkgConfig.InspectOpts.ListImages, "list-images", false, lang.CmdPackageInspectFlagListImages)
	cmd.Flags().BoolVar(&pkgConfig.PkgOpts.SkipSignatureValidation, "skip-signature-validation", false, lang.CmdPackageFlagSkipSignatureValidation)

	return cmd
}

// PreRun performs the pre-run checks for 'package inspect' sub-command.
func (o *PackageInspectOptions) PreRun(_ *cobra.Command, _ []string) {
	// If --insecure was provided, set --skip-signature-validation to match
	if config.CommonOptions.Insecure {
		pkgConfig.PkgOpts.SkipSignatureValidation = true
	}
}

// Run performs the execution of 'package inspect' sub-command.
func (o *PackageInspectOptions) Run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	// NOTE(mkcp): Gets user input with message
	src, err := choosePackage(ctx, args)
	if err != nil {
		return err
	}

	cluster, _ := cluster.NewCluster() //nolint:errcheck
	inspectOpt := packager2.ZarfInspectOptions{
		Source:                  src,
		SkipSignatureValidation: pkgConfig.PkgOpts.SkipSignatureValidation,
		Cluster:                 cluster,
		ListImages:              pkgConfig.InspectOpts.ListImages,
		ViewSBOM:                pkgConfig.InspectOpts.ViewSBOM,
		SBOMOutputDir:           pkgConfig.InspectOpts.SBOMOutputDir,
		PublicKeyPath:           pkgConfig.PkgOpts.PublicKeyPath,
	}

	if pkgConfig.InspectOpts.ListImages {
		output, err := packager2.InspectList(ctx, inspectOpt)
		if err != nil {
			return fmt.Errorf("failed to inspect package: %w", err)
		}
		for _, image := range output {
			_, err := fmt.Fprintln(os.Stdout, "-", image)
			if err != nil {
				return err
			}
		}
	}

	output, err := packager2.Inspect(ctx, inspectOpt)
	if err != nil {
		return fmt.Errorf("failed to inspect package: %w", err)
	}
	err = utils.ColorPrintYAML(output, nil, false)
	if err != nil {
		return err
	}
	return nil
}

// PackageListOptions holds the command-line options for 'package list' sub-command.
type PackageListOptions struct{}

// NewPackageListCommand creates the `package list` sub-command.
func NewPackageListCommand() *cobra.Command {
	o := &PackageListOptions{}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"l", "ls"},
		Short:   lang.CmdPackageListShort,
		RunE:    o.Run,
	}

	return cmd
}

// Run performs the execution of 'package list' sub-command.
func (o *PackageListOptions) Run(cmd *cobra.Command, _ []string) error {
	timeoutCtx, cancel := context.WithTimeout(cmd.Context(), cluster.DefaultTimeout)
	defer cancel()
	c, err := cluster.NewClusterWithWait(timeoutCtx)
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	deployedZarfPackages, err := c.GetDeployedZarfPackages(ctx)
	if err != nil && len(deployedZarfPackages) == 0 {
		return fmt.Errorf("unable to get the packages deployed to the cluster: %w", err)
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
	message.TableWithWriter(message.OutputWriter, header, packageData)

	// Print out any unmarshalling errors
	if err != nil {
		return fmt.Errorf("unable to read all of the packages deployed to the cluster: %w", err)
	}
	return nil
}

// PackageRemoveOptions holds the command-line options for 'package remove' sub-command.
type PackageRemoveOptions struct{}

// NewPackageRemoveCommand creates the `package remove` sub-command.
func NewPackageRemoveCommand(v *viper.Viper) *cobra.Command {
	o := &PackageRemoveOptions{}

	cmd := &cobra.Command{
		Use:               "remove { PACKAGE_SOURCE | PACKAGE_NAME } --confirm",
		Aliases:           []string{"u", "rm"},
		Args:              cobra.MaximumNArgs(1),
		Short:             lang.CmdPackageRemoveShort,
		PreRun:            o.PreRun,
		RunE:              o.Run,
		ValidArgsFunction: getPackageCompletionArgs,
	}

	cmd.Flags().BoolVar(&config.CommonOptions.Confirm, "confirm", false, lang.CmdPackageRemoveFlagConfirm)
	_ = cmd.MarkFlagRequired("confirm")
	cmd.Flags().StringVar(&pkgConfig.PkgOpts.OptionalComponents, "components", v.GetString(common.VPkgDeployComponents), lang.CmdPackageRemoveFlagComponents)
	cmd.Flags().BoolVar(&pkgConfig.PkgOpts.SkipSignatureValidation, "skip-signature-validation", false, lang.CmdPackageFlagSkipSignatureValidation)

	return cmd
}

// PreRun performs the pre-run checks for 'package remove' sub-command.
func (o *PackageRemoveOptions) PreRun(_ *cobra.Command, _ []string) {
	// If --insecure was provided, set --skip-signature-validation to match
	if config.CommonOptions.Insecure {
		pkgConfig.PkgOpts.SkipSignatureValidation = true
	}
}

// Run performs the execution of 'package remove' sub-command.
func (o *PackageRemoveOptions) Run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	packageSource, err := choosePackage(ctx, args)
	if err != nil {
		return err
	}
	filter := filters.Combine(
		filters.ByLocalOS(runtime.GOOS),
		filters.BySelectState(pkgConfig.PkgOpts.OptionalComponents),
	)
	cluster, _ := cluster.NewCluster() //nolint:errcheck
	removeOpt := packager2.RemoveOptions{
		Source:                  packageSource,
		Cluster:                 cluster,
		Filter:                  filter,
		SkipSignatureValidation: pkgConfig.PkgOpts.SkipSignatureValidation,
		PublicKeyPath:           pkgConfig.PkgOpts.PublicKeyPath,
	}
	err = packager2.Remove(ctx, removeOpt)
	if err != nil {
		return err
	}
	return nil
}

// PackagePublishOptions holds the command-line options for 'package publish' sub-command.
type PackagePublishOptions struct{}

// NewPackagePublishCommand creates the `package publish` sub-command.
func NewPackagePublishCommand(v *viper.Viper) *cobra.Command {
	o := &PackagePublishOptions{}

	cmd := &cobra.Command{
		Use:     "publish { PACKAGE_SOURCE | SKELETON DIRECTORY } REPOSITORY",
		Short:   lang.CmdPackagePublishShort,
		Example: lang.CmdPackagePublishExample,
		Args:    cobra.ExactArgs(2),
		PreRun:  o.PreRun,
		RunE:    o.Run,
	}

	cmd.Flags().StringVar(&pkgConfig.PublishOpts.SigningKeyPath, "signing-key", v.GetString(common.VPkgPublishSigningKey), lang.CmdPackagePublishFlagSigningKey)
	cmd.Flags().StringVar(&pkgConfig.PublishOpts.SigningKeyPassword, "signing-key-pass", v.GetString(common.VPkgPublishSigningKeyPassword), lang.CmdPackagePublishFlagSigningKeyPassword)
	cmd.Flags().BoolVar(&pkgConfig.PkgOpts.SkipSignatureValidation, "skip-signature-validation", false, lang.CmdPackageFlagSkipSignatureValidation)

	return cmd
}

// PreRun performs the pre-run checks for 'package publish' sub-command.
func (o *PackagePublishOptions) PreRun(_ *cobra.Command, _ []string) {
	// If --insecure was provided, set --skip-signature-validation to match
	if config.CommonOptions.Insecure {
		pkgConfig.PkgOpts.SkipSignatureValidation = true
	}
}

// Run performs the execution of 'package publish' sub-command.
func (o *PackagePublishOptions) Run(cmd *cobra.Command, args []string) error {
	pkgConfig.PkgOpts.PackageSource = args[0]

	if !helpers.IsOCIURL(args[1]) {
		return errors.New("Registry must be prefixed with 'oci://'")
	}
	parts := strings.Split(strings.TrimPrefix(args[1], helpers.OCIURLPrefix), "/")
	ref := registry.Reference{
		Registry:   parts[0],
		Repository: strings.Join(parts[1:], "/"),
	}
	err := ref.ValidateRegistry()
	if err != nil {
		return err
	}

	if helpers.IsDir(pkgConfig.PkgOpts.PackageSource) {
		pkgConfig.CreateOpts.BaseDir = pkgConfig.PkgOpts.PackageSource
		pkgConfig.CreateOpts.IsSkeleton = true
	}

	pkgConfig.PublishOpts.PackageDestination = ref.String()

	pkgClient, err := packager.New(&pkgConfig, packager.WithContext(cmd.Context()))
	if err != nil {
		return err
	}
	defer pkgClient.ClearTempPaths()

	if err := pkgClient.Publish(cmd.Context()); err != nil {
		return fmt.Errorf("failed to publish package: %w", err)
	}
	return nil
}

// PackagePullOptions holds the command-line options for 'package pull' sub-command.
type PackagePullOptions struct{}

// NewPackagePullCommand creates the `package pull` sub-command.
func NewPackagePullCommand(v *viper.Viper) *cobra.Command {
	o := &PackagePullOptions{}

	cmd := &cobra.Command{
		Use:     "pull PACKAGE_SOURCE",
		Short:   lang.CmdPackagePullShort,
		Example: lang.CmdPackagePullExample,
		Args:    cobra.ExactArgs(1),
		RunE:    o.Run,
	}

	cmd.Flags().StringVar(&pkgConfig.PkgOpts.Shasum, "shasum", "", lang.CmdPackagePullFlagShasum)
	cmd.Flags().StringVarP(&pkgConfig.PullOpts.OutputDirectory, "output-directory", "o", v.GetString(common.VPkgPullOutputDir), lang.CmdPackagePullFlagOutputDirectory)

	return cmd
}

// Run performs the execution of 'package pull' sub-command.
func (o *PackagePullOptions) Run(cmd *cobra.Command, args []string) error {
	outputDir := pkgConfig.PullOpts.OutputDirectory
	if outputDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		outputDir = wd
	}
	err := packager2.Pull(cmd.Context(), args[0], outputDir, pkgConfig.PkgOpts.Shasum, filters.Empty())
	if err != nil {
		return err
	}
	return nil
}

func choosePackage(ctx context.Context, args []string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}
	l := logger.From(ctx)
	var path string
	prompt := &survey.Input{
		Message: lang.CmdPackageChoose,
		Suggest: func(toComplete string) []string {
			tarPath := config.ZarfPackagePrefix + toComplete + "*.tar"
			files, err := filepath.Glob(tarPath)
			if err != nil {
				l.Debug("unable to glob", "tarPath", tarPath, "error", err)
			}

			zstPath := config.ZarfPackagePrefix + toComplete + "*.tar.zst"
			zstFiles, err := filepath.Glob(zstPath)
			if err != nil {
				l.Debug("unable to glob", "zstPath", zstPath, "error", err)
			}

			splitPath := config.ZarfPackagePrefix + toComplete + "*.part000"
			splitFiles, err := filepath.Glob(splitPath)
			if err != nil {
				l.Debug("unable to glob", "splitPath", splitPath, "error", err)
			}

			files = append(files, zstFiles...)
			files = append(files, splitFiles...)
			return files
		},
	}

	if err := survey.AskOne(prompt, &path, survey.WithValidator(survey.Required)); err != nil {
		return "", fmt.Errorf("package path selection canceled: %w", err)
	}

	return path, nil
}

func getPackageCompletionArgs(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	var pkgCandidates []string

	c, err := cluster.NewCluster()
	if err != nil {
		return pkgCandidates, cobra.ShellCompDirectiveDefault
	}

	ctx := cmd.Context()

	deployedZarfPackages, err := c.GetDeployedZarfPackages(ctx)
	if err != nil {
		message.Debug("Unable to get deployed zarf packages for package completion args", "error", err)
		logger.From(cmd.Context()).Debug("unable to get deployed zarf packages for package completion args", "error", err)
	}
	// Populate list of package names
	for _, pkg := range deployedZarfPackages {
		pkgCandidates = append(pkgCandidates, pkg.Name)
	}

	return pkgCandidates, cobra.ShellCompDirectiveDefault
}
