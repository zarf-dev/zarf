// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/pkg/helpers/v2"
	goyaml "github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"oras.land/oras-go/v2/registry"

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

func newPackageCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "package",
		Aliases: []string{"p"},
		Short:   lang.CmdPackageShort,
	}

	v := getViper()

	persistentFlags := cmd.PersistentFlags()
	persistentFlags.IntVar(&config.CommonOptions.OCIConcurrency, "oci-concurrency", v.GetInt(VPkgOCIConcurrency), lang.CmdPackageFlagConcurrency)
	persistentFlags.StringVarP(&pkgConfig.PkgOpts.PublicKeyPath, "key", "k", v.GetString(VPkgPublicKey), lang.CmdPackageFlagFlagPublicKey)

	cmd.AddCommand(newPackageCreateCommand(v))
	cmd.AddCommand(newPackageDeployCommand(v))
	cmd.AddCommand(newPackageMirrorResourcesCommand(v))
	cmd.AddCommand(newPackageInspectCommand())
	cmd.AddCommand(newPackageRemoveCommand(v))
	cmd.AddCommand(newPackageListCommand())
	cmd.AddCommand(newPackagePublishCommand(v))
	cmd.AddCommand(newPackagePullCommand(v))

	return cmd
}

type packageCreateOptions struct{}

func newPackageCreateCommand(v *viper.Viper) *cobra.Command {
	o := &packageCreateOptions{}

	cmd := &cobra.Command{
		Use:     "create [ DIRECTORY ]",
		Aliases: []string{"c"},
		Args:    cobra.MaximumNArgs(1),
		Short:   lang.CmdPackageCreateShort,
		Long:    lang.CmdPackageCreateLong,
		RunE:    o.run,
	}

	// Always require confirm flag (no viper)
	cmd.Flags().BoolVar(&config.CommonOptions.Confirm, "confirm", false, lang.CmdPackageCreateFlagConfirm)

	outputDirectory := v.GetString("package.create.output_directory")
	output := v.GetString(VPkgCreateOutput)
	if outputDirectory != "" && output == "" {
		v.Set(VPkgCreateOutput, outputDirectory)
	}
	cmd.Flags().StringVar(&pkgConfig.CreateOpts.Output, "output-directory", v.GetString("package.create.output_directory"), lang.CmdPackageCreateFlagOutput)
	cmd.Flags().StringVarP(&pkgConfig.CreateOpts.Output, "output", "o", v.GetString(VPkgCreateOutput), lang.CmdPackageCreateFlagOutput)

	cmd.Flags().StringVar(&pkgConfig.CreateOpts.DifferentialPackagePath, "differential", v.GetString(VPkgCreateDifferential), lang.CmdPackageCreateFlagDifferential)
	cmd.Flags().StringToStringVar(&pkgConfig.CreateOpts.SetVariables, "set", v.GetStringMapString(VPkgCreateSet), lang.CmdPackageCreateFlagSet)
	cmd.Flags().BoolVarP(&pkgConfig.CreateOpts.ViewSBOM, "sbom", "s", v.GetBool(VPkgCreateSbom), lang.CmdPackageCreateFlagSbom)
	cmd.Flags().StringVar(&pkgConfig.CreateOpts.SBOMOutputDir, "sbom-out", v.GetString(VPkgCreateSbomOutput), lang.CmdPackageCreateFlagSbomOut)
	cmd.Flags().BoolVar(&pkgConfig.CreateOpts.SkipSBOM, "skip-sbom", v.GetBool(VPkgCreateSkipSbom), lang.CmdPackageCreateFlagSkipSbom)
	cmd.Flags().IntVarP(&pkgConfig.CreateOpts.MaxPackageSizeMB, "max-package-size", "m", v.GetInt(VPkgCreateMaxPackageSize), lang.CmdPackageCreateFlagMaxPackageSize)
	cmd.Flags().StringToStringVar(&pkgConfig.CreateOpts.RegistryOverrides, "registry-override", v.GetStringMapString(VPkgCreateRegistryOverride), lang.CmdPackageCreateFlagRegistryOverride)
	cmd.Flags().StringVarP(&pkgConfig.CreateOpts.Flavor, "flavor", "f", v.GetString(VPkgCreateFlavor), lang.CmdPackageCreateFlagFlavor)

	cmd.Flags().StringVar(&pkgConfig.CreateOpts.SigningKeyPath, "signing-key", v.GetString(VPkgCreateSigningKey), lang.CmdPackageCreateFlagSigningKey)
	cmd.Flags().StringVar(&pkgConfig.CreateOpts.SigningKeyPassword, "signing-key-pass", v.GetString(VPkgCreateSigningKeyPassword), lang.CmdPackageCreateFlagSigningKeyPassword)

	cmd.Flags().StringVarP(&pkgConfig.CreateOpts.SigningKeyPath, "key", "k", v.GetString(VPkgCreateSigningKey), lang.CmdPackageCreateFlagDeprecatedKey)
	cmd.Flags().StringVar(&pkgConfig.CreateOpts.SigningKeyPassword, "key-pass", v.GetString(VPkgCreateSigningKeyPassword), lang.CmdPackageCreateFlagDeprecatedKeyPassword)

	cmd.Flags().IntVar(&pkgConfig.PkgOpts.Retries, "retries", v.GetInt(VPkgRetries), lang.CmdPackageFlagRetries)

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

func (o *packageCreateOptions) run(cmd *cobra.Command, args []string) error {
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

	v := getViper()
	pkgConfig.CreateOpts.SetVariables = helpers.TransformAndMergeMap(
		v.GetStringMapString(VPkgCreateSet), pkgConfig.CreateOpts.SetVariables, strings.ToUpper)

	opt := packager2.CreateOptions{
		Flavor:                  pkgConfig.CreateOpts.Flavor,
		RegistryOverrides:       pkgConfig.CreateOpts.RegistryOverrides,
		SigningKeyPath:          pkgConfig.CreateOpts.SigningKeyPath,
		SigningKeyPassword:      pkgConfig.CreateOpts.SigningKeyPassword,
		SetVariables:            pkgConfig.CreateOpts.SetVariables,
		MaxPackageSizeMB:        pkgConfig.CreateOpts.MaxPackageSizeMB,
		SBOMOut:                 pkgConfig.CreateOpts.SBOMOutputDir,
		SkipSBOM:                pkgConfig.CreateOpts.SkipSBOM,
		Output:                  pkgConfig.CreateOpts.Output,
		DifferentialPackagePath: pkgConfig.CreateOpts.DifferentialPackagePath,
	}
	err := packager2.Create(cmd.Context(), pkgConfig.CreateOpts.BaseDir, opt)
	// NOTE(mkcp): LintErrors are rendered with a table
	var lintErr *lint.LintError
	if errors.As(err, &lintErr) {
		PrintFindings(ctx, lintErr)
	}
	if err != nil {
		return fmt.Errorf("failed to create package: %w", err)
	}
	return nil
}

type packageDeployOptions struct{}

func newPackageDeployCommand(v *viper.Viper) *cobra.Command {
	o := &packageDeployOptions{}

	cmd := &cobra.Command{
		Use:     "deploy [ PACKAGE_SOURCE ]",
		Aliases: []string{"d"},
		Short:   lang.CmdPackageDeployShort,
		Long:    lang.CmdPackageDeployLong,
		Args:    cobra.MaximumNArgs(1),
		PreRun:  o.preRun,
		RunE:    o.run,
	}

	// Always require confirm flag (no viper)
	cmd.Flags().BoolVar(&config.CommonOptions.Confirm, "confirm", false, lang.CmdPackageDeployFlagConfirm)

	// Always require adopt-existing-resources flag (no viper)
	cmd.Flags().BoolVar(&pkgConfig.DeployOpts.AdoptExistingResources, "adopt-existing-resources", false, lang.CmdPackageDeployFlagAdoptExistingResources)
	cmd.Flags().DurationVar(&pkgConfig.DeployOpts.Timeout, "timeout", v.GetDuration(VPkgDeployTimeout), lang.CmdPackageDeployFlagTimeout)

	cmd.Flags().IntVar(&pkgConfig.PkgOpts.Retries, "retries", v.GetInt(VPkgRetries), lang.CmdPackageFlagRetries)
	cmd.Flags().StringToStringVar(&pkgConfig.PkgOpts.SetVariables, "set", v.GetStringMapString(VPkgDeploySet), lang.CmdPackageDeployFlagSet)
	cmd.Flags().StringVar(&pkgConfig.PkgOpts.OptionalComponents, "components", v.GetString(VPkgDeployComponents), lang.CmdPackageDeployFlagComponents)
	cmd.Flags().StringVar(&pkgConfig.PkgOpts.Shasum, "shasum", v.GetString(VPkgDeployShasum), lang.CmdPackageDeployFlagShasum)
	cmd.Flags().StringVar(&pkgConfig.PkgOpts.SGetKeyPath, "sget", v.GetString(VPkgDeploySget), lang.CmdPackageDeployFlagSget)
	cmd.Flags().BoolVar(&pkgConfig.PkgOpts.SkipSignatureValidation, "skip-signature-validation", false, lang.CmdPackageFlagSkipSignatureValidation)

	err := cmd.Flags().MarkHidden("sget")
	if err != nil {
		logger.Default().Debug("unable to mark flag sget", "error", err)
	}

	return cmd
}

func (o *packageDeployOptions) preRun(_ *cobra.Command, _ []string) {
	// If --insecure was provided, set --skip-signature-validation to match
	if config.CommonOptions.Insecure {
		pkgConfig.PkgOpts.SkipSignatureValidation = true
	}
}

func (o *packageDeployOptions) run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	packageSource, err := choosePackage(ctx, args)
	if err != nil {
		return err
	}
	pkgConfig.PkgOpts.PackageSource = packageSource

	v := getViper()
	pkgConfig.PkgOpts.SetVariables = helpers.TransformAndMergeMap(
		v.GetStringMapString(VPkgDeploySet), pkgConfig.PkgOpts.SetVariables, strings.ToUpper)

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

type packageMirrorResourcesOptions struct{}

func newPackageMirrorResourcesCommand(v *viper.Viper) *cobra.Command {
	o := &packageMirrorResourcesOptions{}

	cmd := &cobra.Command{
		Use:     "mirror-resources [ PACKAGE_SOURCE ]",
		Aliases: []string{"mr"},
		Short:   lang.CmdPackageMirrorShort,
		Long:    lang.CmdPackageMirrorLong,
		Example: lang.CmdPackageMirrorExample,
		Args:    cobra.MaximumNArgs(1),
		PreRun:  o.preRun,
		RunE:    o.run,
	}

	// Init package variable defaults that are non-zero values
	// NOTE: these are not in setDefaults so that zarf tools update-creds does not erroneously update values back to the default
	v.SetDefault(VInitGitPushUser, types.ZarfGitPushUser)
	v.SetDefault(VInitRegistryPushUser, types.ZarfRegistryPushUser)

	// Always require confirm flag (no viper)
	cmd.Flags().BoolVar(&config.CommonOptions.Confirm, "confirm", false, lang.CmdPackageDeployFlagConfirm)

	cmd.Flags().StringVar(&pkgConfig.PkgOpts.Shasum, "shasum", "", lang.CmdPackagePullFlagShasum)
	cmd.Flags().BoolVar(&pkgConfig.MirrorOpts.NoImgChecksum, "no-img-checksum", false, lang.CmdPackageMirrorFlagNoChecksum)
	cmd.Flags().BoolVar(&pkgConfig.PkgOpts.SkipSignatureValidation, "skip-signature-validation", false, lang.CmdPackageFlagSkipSignatureValidation)

	cmd.Flags().IntVar(&pkgConfig.PkgOpts.Retries, "retries", v.GetInt(VPkgRetries), lang.CmdPackageFlagRetries)
	cmd.Flags().StringVar(&pkgConfig.PkgOpts.OptionalComponents, "components", v.GetString(VPkgDeployComponents), lang.CmdPackageMirrorFlagComponents)

	// Flags for using an external Git server
	cmd.Flags().StringVar(&pkgConfig.InitOpts.GitServer.Address, "git-url", v.GetString(VInitGitURL), lang.CmdInitFlagGitURL)
	cmd.Flags().StringVar(&pkgConfig.InitOpts.GitServer.PushUsername, "git-push-username", v.GetString(VInitGitPushUser), lang.CmdInitFlagGitPushUser)
	cmd.Flags().StringVar(&pkgConfig.InitOpts.GitServer.PushPassword, "git-push-password", v.GetString(VInitGitPushPass), lang.CmdInitFlagGitPushPass)

	// Flags for using an external registry
	cmd.Flags().StringVar(&pkgConfig.InitOpts.RegistryInfo.Address, "registry-url", v.GetString(VInitRegistryURL), lang.CmdInitFlagRegURL)
	cmd.Flags().StringVar(&pkgConfig.InitOpts.RegistryInfo.PushUsername, "registry-push-username", v.GetString(VInitRegistryPushUser), lang.CmdInitFlagRegPushUser)
	cmd.Flags().StringVar(&pkgConfig.InitOpts.RegistryInfo.PushPassword, "registry-push-password", v.GetString(VInitRegistryPushPass), lang.CmdInitFlagRegPushPass)

	return cmd
}

func (o *packageMirrorResourcesOptions) preRun(_ *cobra.Command, _ []string) {
	// If --insecure was provided, set --skip-signature-validation to match
	if config.CommonOptions.Insecure {
		pkgConfig.PkgOpts.SkipSignatureValidation = true
	}
}

func (o *packageMirrorResourcesOptions) run(cmd *cobra.Command, args []string) (err error) {
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

type packageInspectOptions struct{}

func newPackageInspectCommand() *cobra.Command {
	o := &packageInspectOptions{}
	cmd := &cobra.Command{
		Use:     "inspect [ PACKAGE_SOURCE ]",
		Aliases: []string{"i"},
		Short:   lang.CmdPackageInspectShort,
		Long:    lang.CmdPackageInspectLong,
		Args:    cobra.MaximumNArgs(1),
		PreRun:  o.preRun,
		RunE:    o.run,
	}

	cmd.AddCommand(newPackageInspectSBOMCommand())
	cmd.AddCommand(newPackageInspectImagesCommand())
	cmd.AddCommand(newPackageInspectDefinitionCommand())

	cmd.Flags().StringVar(&pkgConfig.InspectOpts.SBOMOutputDir, "sbom-out", "", lang.CmdPackageInspectFlagSbomOut)
	cmd.Flags().BoolVar(&pkgConfig.InspectOpts.ListImages, "list-images", false, lang.CmdPackageInspectFlagListImages)
	cmd.Flags().BoolVar(&pkgConfig.PkgOpts.SkipSignatureValidation, "skip-signature-validation", false, lang.CmdPackageFlagSkipSignatureValidation)

	return cmd
}

func (o *packageInspectOptions) preRun(_ *cobra.Command, _ []string) {
	// If --insecure was provided, set --skip-signature-validation to match
	if config.CommonOptions.Insecure {
		pkgConfig.PkgOpts.SkipSignatureValidation = true
	}
}

func (o *packageInspectOptions) run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	logger.From(ctx).Warn("Direct usage of inspect is deprecated and will be removed in a future release. Inspect is now a parent command. Use 'zarf package inspect definition|sbom|images' instead.")

	if pkgConfig.InspectOpts.ListImages && pkgConfig.InspectOpts.SBOMOutputDir != "" {
		return fmt.Errorf("cannot use --sbom-out and --list-images at the same time")
	}

	if pkgConfig.InspectOpts.SBOMOutputDir != "" {
		sbomOpts := PackageInspectSBOMOptions{
			skipSignatureValidation: pkgConfig.PkgOpts.SkipSignatureValidation,
			outputDir:               pkgConfig.InspectOpts.SBOMOutputDir,
		}
		return sbomOpts.Run(cmd, args)
	}

	if pkgConfig.InspectOpts.ListImages {
		imagesOpts := packageInspectImagesOptions{
			skipSignatureValidation: pkgConfig.PkgOpts.SkipSignatureValidation,
		}
		return imagesOpts.run(cmd, args)
	}

	definitionOpts := packageInspectDefinitionOptions{
		skipSignatureValidation: pkgConfig.PkgOpts.SkipSignatureValidation,
	}
	return definitionOpts.run(cmd, args)
}

// PackageInspectSBOMOptions holds the command-line options for 'package inspect sbom' sub-command.
type PackageInspectSBOMOptions struct {
	skipSignatureValidation bool
	outputDir               string
}

func newPackageInspectSBOMOptions() *PackageInspectSBOMOptions {
	return &PackageInspectSBOMOptions{
		outputDir:               "",
		skipSignatureValidation: false,
	}
}

// newPackageInspectSBOMCommand creates the `package inspect sbom` sub-command.
func newPackageInspectSBOMCommand() *cobra.Command {
	o := newPackageInspectSBOMOptions()
	cmd := &cobra.Command{
		Use:   "sbom [ PACKAGE ]",
		Short: "Output the package SBOM (Software Bill Of Materials) to the specified directory",
		Args:  cobra.MaximumNArgs(1),
		RunE:  o.Run,
	}

	cmd.Flags().BoolVar(&o.skipSignatureValidation, "skip-signature-validation", o.skipSignatureValidation, lang.CmdPackageFlagSkipSignatureValidation)
	cmd.Flags().StringVar(&o.outputDir, "output", o.outputDir, lang.CmdPackageCreateFlagSbomOut)

	return cmd
}

// Run performs the execution of 'package inspect sbom' sub-command.
func (o *PackageInspectSBOMOptions) Run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	src, err := choosePackage(ctx, args)
	if err != nil {
		return err
	}
	loadOpt := packager2.LoadOptions{
		Source:                  src,
		SkipSignatureValidation: o.skipSignatureValidation,
		Filter:                  filters.Empty(),
		PublicKeyPath:           pkgConfig.PkgOpts.PublicKeyPath,
	}
	layout, err := packager2.LoadPackage(ctx, loadOpt)
	if err != nil {
		return err
	}
	outputPath, err := layout.GetSBOM(o.outputDir)
	if err != nil {
		return fmt.Errorf("could not get SBOM: %w", err)
	}
	outputPath, err = filepath.Abs(outputPath)
	if err != nil {
		logger.From(ctx).Warn("SBOM successfully extracted, couldn't get output path", "error", err)
		return nil
	}
	logger.From(ctx).Info("SBOM successfully extracted", "path", outputPath)
	return nil
}

type packageInspectImagesOptions struct {
	skipSignatureValidation bool
}

func newPackageInspectImagesOptions() *packageInspectImagesOptions {
	return &packageInspectImagesOptions{
		skipSignatureValidation: false,
	}
}

func newPackageInspectImagesCommand() *cobra.Command {
	o := newPackageInspectImagesOptions()
	cmd := &cobra.Command{
		Use:   "images [ PACKAGE_SOURCE ]",
		Short: "List all container images contained in the package",
		Args:  cobra.MaximumNArgs(1),
		RunE:  o.run,
	}

	cmd.Flags().BoolVar(&o.skipSignatureValidation, "skip-signature-validation", o.skipSignatureValidation, lang.CmdPackageFlagSkipSignatureValidation)

	return cmd
}

func (o *packageInspectImagesOptions) run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	src, err := choosePackage(ctx, args)
	if err != nil {
		return err
	}

	// The user may be pulling the package from the cluster or using a built package
	// since we don't know we don't check this error
	cluster, _ := cluster.NewCluster() //nolint:errcheck

	pkg, err := packager2.GetPackageFromSourceOrCluster(ctx, cluster, src, o.skipSignatureValidation, pkgConfig.PkgOpts.PublicKeyPath)
	if err != nil {
		return err
	}
	var imageList []string
	for _, component := range pkg.Components {
		imageList = append(imageList, component.Images...)
	}
	if imageList == nil {
		return fmt.Errorf("failed listing images: 0 images found in package")
	}
	imageList = helpers.Unique(imageList)
	for _, image := range imageList {
		fmt.Println("-", image)
	}
	return nil
}

type packageInspectDefinitionOptions struct {
	skipSignatureValidation bool
}

func newPackageInspectDefinitionOptions() *packageInspectDefinitionOptions {
	return &packageInspectDefinitionOptions{
		skipSignatureValidation: false,
	}
}

func newPackageInspectDefinitionCommand() *cobra.Command {
	o := newPackageInspectDefinitionOptions()
	cmd := &cobra.Command{
		Use:   "definition [ PACKAGE_SOURCE ]",
		Short: "Displays the 'zarf.yaml' definition for the specified package",
		Args:  cobra.MaximumNArgs(1),
		RunE:  o.run,
	}

	cmd.Flags().BoolVar(&o.skipSignatureValidation, "skip-signature-validation", o.skipSignatureValidation, lang.CmdPackageFlagSkipSignatureValidation)

	return cmd
}

func (o *packageInspectDefinitionOptions) run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	src, err := choosePackage(ctx, args)
	if err != nil {
		return err
	}

	// The user may be pulling the package from the cluster or using a built package
	// since we don't know we don't check this error
	cluster, _ := cluster.NewCluster() //nolint:errcheck

	pkg, err := packager2.GetPackageFromSourceOrCluster(ctx, cluster, src, o.skipSignatureValidation, pkgConfig.PkgOpts.PublicKeyPath)
	if err != nil {
		return err
	}
	err = utils.ColorPrintYAML(pkg, nil, false)
	if err != nil {
		return err
	}
	return nil
}

type packageListOptions struct {
	outputFormat outputFormat
	outputWriter io.Writer
	cluster      *cluster.Cluster
}

func newPackageListOptions() *packageListOptions {
	return &packageListOptions{
		outputFormat: outputTable,
		// TODO accept output writer as a parameter to the root Zarf command and pass it through here
		outputWriter: message.OutputWriter,
	}
}

func newPackageListCommand() *cobra.Command {
	o := newPackageListOptions()

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"l", "ls"},
		Short:   lang.CmdPackageListShort,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			err := o.complete(ctx)
			if err != nil {
				return err
			}
			return o.run(ctx)
		},
	}

	cmd.Flags().VarP(&o.outputFormat, "output-format", "o", "Prints the output in the specified format. Valid options: table, json, yaml")

	return cmd
}

func (o *packageListOptions) complete(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, cluster.DefaultTimeout)
	defer cancel()
	c, err := cluster.NewClusterWithWait(timeoutCtx)
	if err != nil {
		return err
	}
	o.cluster = c
	return nil
}

// packageListInfo represents the package information for output.
type packageListInfo struct {
	Package    string   `json:"package"`
	Version    string   `json:"version"`
	Components []string `json:"components"`
}

func (o *packageListOptions) run(ctx context.Context) error {
	deployedZarfPackages, err := o.cluster.GetDeployedZarfPackages(ctx)
	if err != nil && len(deployedZarfPackages) == 0 {
		return fmt.Errorf("unable to get the packages deployed to the cluster: %w", err)
	}

	var packageList []packageListInfo
	for _, pkg := range deployedZarfPackages {
		var components []string
		for _, component := range pkg.DeployedComponents {
			components = append(components, component.Name)
		}
		packageList = append(packageList, packageListInfo{
			Package:    pkg.Name,
			Version:    pkg.Data.Metadata.Version,
			Components: components,
		})
	}

	switch o.outputFormat {
	case outputJSON:
		output, err := json.MarshalIndent(packageList, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(o.outputWriter, string(output))
	case outputYAML:
		output, err := goyaml.Marshal(packageList)
		if err != nil {
			return err
		}
		fmt.Fprint(o.outputWriter, string(output))
	case outputTable:
		header := []string{"Package", "Version", "Components"}
		var packageData [][]string
		for _, info := range packageList {
			packageData = append(packageData, []string{
				info.Package, info.Version, fmt.Sprintf("%v", info.Components),
			})
		}
		message.TableWithWriter(o.outputWriter, header, packageData)
	default:
		return fmt.Errorf("unsupported output format: %s", o.outputFormat)
	}
	return nil
}

type packageRemoveOptions struct{}

func newPackageRemoveCommand(v *viper.Viper) *cobra.Command {
	o := &packageRemoveOptions{}

	cmd := &cobra.Command{
		Use:               "remove { PACKAGE_SOURCE | PACKAGE_NAME } --confirm",
		Aliases:           []string{"u", "rm"},
		Args:              cobra.MaximumNArgs(1),
		Short:             lang.CmdPackageRemoveShort,
		Long:              lang.CmdPackageRemoveLong,
		PreRun:            o.preRun,
		RunE:              o.run,
		ValidArgsFunction: getPackageCompletionArgs,
	}

	cmd.Flags().BoolVar(&config.CommonOptions.Confirm, "confirm", false, lang.CmdPackageRemoveFlagConfirm)
	_ = cmd.MarkFlagRequired("confirm")
	cmd.Flags().StringVar(&pkgConfig.PkgOpts.OptionalComponents, "components", v.GetString(VPkgDeployComponents), lang.CmdPackageRemoveFlagComponents)
	cmd.Flags().BoolVar(&pkgConfig.PkgOpts.SkipSignatureValidation, "skip-signature-validation", false, lang.CmdPackageFlagSkipSignatureValidation)

	return cmd
}

func (o *packageRemoveOptions) preRun(_ *cobra.Command, _ []string) {
	// If --insecure was provided, set --skip-signature-validation to match
	if config.CommonOptions.Insecure {
		pkgConfig.PkgOpts.SkipSignatureValidation = true
	}
}

func (o *packageRemoveOptions) run(cmd *cobra.Command, args []string) error {
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

type packagePublishOptions struct{}

func newPackagePublishCommand(v *viper.Viper) *cobra.Command {
	o := &packagePublishOptions{}

	cmd := &cobra.Command{
		Use:     "publish { PACKAGE_SOURCE | SKELETON DIRECTORY } REPOSITORY",
		Short:   lang.CmdPackagePublishShort,
		Example: lang.CmdPackagePublishExample,
		Args:    cobra.ExactArgs(2),
		PreRun:  o.preRun,
		RunE:    o.run,
	}

	cmd.Flags().StringVar(&pkgConfig.PublishOpts.SigningKeyPath, "signing-key", v.GetString(VPkgPublishSigningKey), lang.CmdPackagePublishFlagSigningKey)
	cmd.Flags().StringVar(&pkgConfig.PublishOpts.SigningKeyPassword, "signing-key-pass", v.GetString(VPkgPublishSigningKeyPassword), lang.CmdPackagePublishFlagSigningKeyPassword)
	cmd.Flags().BoolVar(&pkgConfig.PkgOpts.SkipSignatureValidation, "skip-signature-validation", false, lang.CmdPackageFlagSkipSignatureValidation)
	cmd.Flags().BoolVar(&config.CommonOptions.Confirm, "confirm", false, lang.CmdPackagePublishFlagConfirm)

	return cmd
}

func (o *packagePublishOptions) preRun(_ *cobra.Command, _ []string) {
	// If --insecure was provided, set --skip-signature-validation to match
	if config.CommonOptions.Insecure {
		pkgConfig.PkgOpts.SkipSignatureValidation = true
	}
}

func (o *packagePublishOptions) run(cmd *cobra.Command, args []string) error {
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

type packagePullOptions struct{}

func newPackagePullCommand(v *viper.Viper) *cobra.Command {
	o := &packagePullOptions{}

	cmd := &cobra.Command{
		Use:     "pull PACKAGE_SOURCE",
		Short:   lang.CmdPackagePullShort,
		Example: lang.CmdPackagePullExample,
		Args:    cobra.ExactArgs(1),
		RunE:    o.run,
	}

	cmd.Flags().StringVar(&pkgConfig.PkgOpts.Shasum, "shasum", "", lang.CmdPackagePullFlagShasum)
	cmd.Flags().StringVarP(&pkgConfig.PullOpts.OutputDirectory, "output-directory", "o", v.GetString(VPkgPullOutputDir), lang.CmdPackagePullFlagOutputDirectory)
	cmd.Flags().BoolVar(&pkgConfig.PkgOpts.SkipSignatureValidation, "skip-signature-validation", false, lang.CmdPackageFlagSkipSignatureValidation)

	return cmd
}

func (o *packagePullOptions) run(cmd *cobra.Command, args []string) error {
	outputDir := pkgConfig.PullOpts.OutputDirectory
	if outputDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		outputDir = wd
	}
	err := packager2.Pull(cmd.Context(), args[0], outputDir, pkgConfig.PkgOpts.Shasum, filters.Empty(), pkgConfig.PkgOpts.PublicKeyPath, pkgConfig.PkgOpts.SkipSignatureValidation)
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
