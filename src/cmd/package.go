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
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/pkg/helpers/v2"
	goyaml "github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/zarf-dev/zarf/src/pkg/packager"
	"oras.land/oras-go/v2/registry"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
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

type packageCreateOptions struct {
	confirm                 bool
	output                  string
	differentialPackagePath string
	setVariables            map[string]string
	sbom                    bool
	sbomOutput              string
	skipSBOM                bool
	maxPackageSizeMB        int
	registryOverrides       map[string]string
	signingKeyPath          string
	signingKeyPassword      string
	flavor                  string
}

func newPackageCreateCommand(v *viper.Viper) *cobra.Command {
	o := &packageCreateOptions{}

	cmd := &cobra.Command{
		Use:     "create [ DIRECTORY ]",
		Aliases: []string{"c"},
		Args:    cobra.MaximumNArgs(1),
		Short:   lang.CmdPackageCreateShort,
		Long:    lang.CmdPackageCreateLong,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return o.run(ctx, args)
		},
	}

	// Always require confirm flag (no viper)
	cmd.Flags().BoolVarP(&o.confirm, "confirm", "c", false, lang.CmdPackageCreateFlagConfirm)

	outputDirectory := v.GetString("package.create.output_directory")
	output := v.GetString(VPkgCreateOutput)
	if outputDirectory != "" && output == "" {
		v.Set(VPkgCreateOutput, outputDirectory)
	}
	cmd.Flags().StringVar(&o.output, "output-directory", v.GetString("package.create.output_directory"), lang.CmdPackageCreateFlagOutput)
	cmd.Flags().StringVarP(&o.output, "output", "o", v.GetString(VPkgCreateOutput), lang.CmdPackageCreateFlagOutput)

	cmd.Flags().StringVar(&o.differentialPackagePath, "differential", v.GetString(VPkgCreateDifferential), lang.CmdPackageCreateFlagDifferential)
	cmd.Flags().StringToStringVar(&o.setVariables, "set", v.GetStringMapString(VPkgCreateSet), lang.CmdPackageCreateFlagSet)
	cmd.Flags().BoolVarP(&o.sbom, "sbom", "s", v.GetBool(VPkgCreateSbom), lang.CmdPackageCreateFlagSbom)
	cmd.Flags().StringVar(&o.sbomOutput, "sbom-out", v.GetString(VPkgCreateSbomOutput), lang.CmdPackageCreateFlagSbomOut)
	cmd.Flags().BoolVar(&o.skipSBOM, "skip-sbom", v.GetBool(VPkgCreateSkipSbom), lang.CmdPackageCreateFlagSkipSbom)
	cmd.Flags().IntVarP(&o.maxPackageSizeMB, "max-package-size", "m", v.GetInt(VPkgCreateMaxPackageSize), lang.CmdPackageCreateFlagMaxPackageSize)
	cmd.Flags().StringToStringVar(&o.registryOverrides, "registry-override", v.GetStringMapString(VPkgCreateRegistryOverride), lang.CmdPackageCreateFlagRegistryOverride)
	cmd.Flags().StringVarP(&o.flavor, "flavor", "f", v.GetString(VPkgCreateFlavor), lang.CmdPackageCreateFlagFlavor)

	cmd.Flags().StringVar(&o.signingKeyPath, "signing-key", v.GetString(VPkgCreateSigningKey), lang.CmdPackageCreateFlagSigningKey)
	cmd.Flags().StringVar(&o.signingKeyPassword, "signing-key-pass", v.GetString(VPkgCreateSigningKeyPassword), lang.CmdPackageCreateFlagSigningKeyPassword)

	cmd.Flags().StringVarP(&o.signingKeyPath, "key", "k", v.GetString(VPkgCreateSigningKey), lang.CmdPackageCreateFlagDeprecatedKey)
	cmd.Flags().StringVar(&o.signingKeyPassword, "key-pass", v.GetString(VPkgCreateSigningKeyPassword), lang.CmdPackageCreateFlagDeprecatedKeyPassword)

	cmd.Flags().IntVar(&pkgConfig.PkgOpts.Retries, "retries", v.GetInt(VPkgRetries), lang.CmdPackageFlagRetries)
	err := cmd.Flags().MarkDeprecated("retries", "retries does not have any impact on package creation")
	if err != nil {
		logger.Default().Debug("unable to mark flag retries as deprecated", "error", err)
	}

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

func (o *packageCreateOptions) run(ctx context.Context, args []string) error {
	// TODO pass confirm through the system rather than keeping it as a global
	config.CommonOptions.Confirm = o.confirm
	l := logger.From(ctx)
	baseDir := setBaseDirectory(args)

	var isCleanPathRegex = regexp.MustCompile(`^[a-zA-Z0-9\_\-\/\.\~\\:]+$`)
	if !isCleanPathRegex.MatchString(config.CommonOptions.CachePath) {
		l.Warn("invalid characters in Zarf cache path, using default", "cfg", config.ZarfDefaultCachePath, "default", config.ZarfDefaultCachePath)
		config.CommonOptions.CachePath = config.ZarfDefaultCachePath
	}

	v := getViper()
	o.setVariables = helpers.TransformAndMergeMap(v.GetStringMapString(VPkgCreateSet), o.setVariables, strings.ToUpper)

	cachePath, err := getCachePath(ctx)
	if err != nil {
		return err
	}
	opt := packager.CreateOptions{
		Flavor:                  o.flavor,
		RegistryOverrides:       o.registryOverrides,
		SigningKeyPath:          o.signingKeyPath,
		SigningKeyPassword:      o.signingKeyPassword,
		SetVariables:            o.setVariables,
		MaxPackageSizeMB:        o.maxPackageSizeMB,
		SBOMOut:                 o.sbomOutput,
		SkipSBOM:                o.skipSBOM,
		OCIConcurrency:          config.CommonOptions.OCIConcurrency,
		DifferentialPackagePath: o.differentialPackagePath,
		RemoteOptions:           defaultRemoteOptions(),
		CachePath:               cachePath,
	}
	_, err = packager.Create(ctx, baseDir, o.output, opt)
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

type packageDeployOptions struct {
	namespaceOverride string
}

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
	cmd.Flags().BoolVarP(&config.CommonOptions.Confirm, "confirm", "c", false, lang.CmdPackageDeployFlagConfirm)

	// Always require adopt-existing-resources flag (no viper)
	cmd.Flags().BoolVar(&pkgConfig.DeployOpts.AdoptExistingResources, "adopt-existing-resources", false, lang.CmdPackageDeployFlagAdoptExistingResources)
	cmd.Flags().DurationVar(&pkgConfig.DeployOpts.Timeout, "timeout", v.GetDuration(VPkgDeployTimeout), lang.CmdPackageDeployFlagTimeout)

	cmd.Flags().IntVar(&pkgConfig.PkgOpts.Retries, "retries", v.GetInt(VPkgRetries), lang.CmdPackageFlagRetries)
	cmd.Flags().StringToStringVar(&pkgConfig.PkgOpts.SetVariables, "set", v.GetStringMapString(VPkgDeploySet), lang.CmdPackageDeployFlagSet)
	cmd.Flags().StringVar(&pkgConfig.PkgOpts.OptionalComponents, "components", v.GetString(VPkgDeployComponents), lang.CmdPackageDeployFlagComponents)
	cmd.Flags().StringVar(&pkgConfig.PkgOpts.Shasum, "shasum", v.GetString(VPkgDeployShasum), lang.CmdPackageDeployFlagShasum)
	cmd.Flags().StringVarP(&o.namespaceOverride, "namespace", "n", v.GetString(VPkgDeployNamespace), lang.CmdPackageDeployFlagNamespace)
	cmd.Flags().BoolVar(&pkgConfig.PkgOpts.SkipSignatureValidation, "skip-signature-validation", false, lang.CmdPackageFlagSkipSignatureValidation)

	return cmd
}

func (o *packageDeployOptions) preRun(_ *cobra.Command, _ []string) {
	// If --insecure was provided, set --skip-signature-validation to match
	if config.CommonOptions.Insecure {
		pkgConfig.PkgOpts.SkipSignatureValidation = true
	}
}

func (o *packageDeployOptions) run(cmd *cobra.Command, args []string) (err error) {
	ctx := cmd.Context()
	packageSource, err := choosePackage(ctx, args)
	if err != nil {
		return err
	}

	v := getViper()
	pkgConfig.PkgOpts.SetVariables = helpers.TransformAndMergeMap(
		v.GetStringMapString(VPkgDeploySet), pkgConfig.PkgOpts.SetVariables, strings.ToUpper)

	cachePath, err := getCachePath(ctx)
	if err != nil {
		return err
	}

	loadOpt := packager.LoadOptions{
		Shasum:                  pkgConfig.PkgOpts.Shasum,
		PublicKeyPath:           pkgConfig.PkgOpts.PublicKeyPath,
		SkipSignatureValidation: pkgConfig.PkgOpts.SkipSignatureValidation,
		Filter:                  filters.Empty(),
		Architecture:            config.GetArch(),
		OCIConcurrency:          config.CommonOptions.OCIConcurrency,
		RemoteOptions:           defaultRemoteOptions(),
		CachePath:               cachePath,
	}
	pkgLayout, err := packager.LoadPackage(ctx, packageSource, loadOpt)
	if err != nil {
		return fmt.Errorf("unable to load package: %w", err)
	}
	defer func() {
		err = errors.Join(err, pkgLayout.Cleanup())
	}()

	deployOpts := packager.DeployOptions{
		AdoptExistingResources: pkgConfig.DeployOpts.AdoptExistingResources,
		Timeout:                pkgConfig.DeployOpts.Timeout,
		Retries:                pkgConfig.PkgOpts.Retries,
		OCIConcurrency:         config.CommonOptions.OCIConcurrency,
		SetVariables:           pkgConfig.PkgOpts.SetVariables,
		NamespaceOverride:      o.namespaceOverride,
		RemoteOptions:          defaultRemoteOptions(),
	}

	deployedComponents, err := deploy(ctx, pkgLayout, deployOpts)
	if err != nil {
		return err
	}

	if pkgLayout.Pkg.IsInitConfig() {
		return nil
	}
	connectStrings := state.ConnectStrings{}
	for _, comp := range deployedComponents {
		for _, chart := range comp.InstalledCharts {
			for k, v := range chart.ConnectStrings {
				connectStrings[k] = v
			}
		}
	}
	printConnectStringTable(connectStrings)
	return nil
}

func deploy(ctx context.Context, pkgLayout *layout.PackageLayout, opts packager.DeployOptions) ([]state.DeployedComponent, error) {
	// Intentionally duplicate the deploy override logic here to allow us to render the updated package in confirm below
	if opts.NamespaceOverride != "" {
		if err := packager.OverridePackageNamespace(pkgLayout.Pkg, opts.NamespaceOverride); err != nil {
			return nil, err
		}
	}
	err := confirmDeploy(ctx, pkgLayout, pkgConfig.PkgOpts.SetVariables)
	if err != nil {
		return nil, err
	}

	// filter after confirmation to allow users to view the entire package interactively
	filter := filters.Combine(
		filters.ByLocalOS(runtime.GOOS),
		filters.ForDeploy(pkgConfig.PkgOpts.OptionalComponents, !config.CommonOptions.Confirm),
	)

	pkgLayout.Pkg.Components, err = filter.Apply(pkgLayout.Pkg)
	if err != nil {
		return nil, err
	}

	result, err := packager.Deploy(ctx, pkgLayout, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to deploy package: %w", err)
	}

	return result.DeployedComponents, nil
}

func confirmDeploy(ctx context.Context, pkgLayout *layout.PackageLayout, setVariables map[string]string) (err error) {
	l := logger.From(ctx)

	err = utils.ColorPrintYAML(pkgLayout.Pkg, getPackageYAMLHints(pkgLayout.Pkg, setVariables), true)
	if err != nil {
		return fmt.Errorf("unable to print package definition: %w", err)
	}

	if pkgLayout.Pkg.IsSBOMAble() && !pkgLayout.ContainsSBOM() {
		l.Warn("this package does NOT contain an SBOM. If you require an SBOM, the package must be built without the --skip-sbom flag")
	}
	if pkgLayout.ContainsSBOM() && !config.CommonOptions.Confirm {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		SBOMPath := filepath.Join(cwd, "zarf-sbom")
		err = pkgLayout.GetSBOM(ctx, SBOMPath)
		if err != nil {
			return err
		}
		defer func() {
			err = errors.Join(err, os.RemoveAll(SBOMPath))
		}()
		l.Info("this package has SBOMs available for review in a temporary directory", "directory", SBOMPath)
	}

	if config.CommonOptions.Confirm {
		return nil
	}

	prompt := &survey.Confirm{
		Message: "Deploy this Zarf package?",
	}
	var confirm bool
	if err := survey.AskOne(prompt, &confirm); err != nil || !confirm {
		return fmt.Errorf("deployment cancelled")
	}

	return nil
}

func getPackageYAMLHints(pkg v1alpha1.ZarfPackage, setVariables map[string]string) map[string]string {
	hints := map[string]string{}

	for _, variable := range pkg.Variables {
		value, present := setVariables[variable.Name]
		if !present {
			value = fmt.Sprintf("'%s' (default)", helpers.Truncate(variable.Default, 20, false))
		} else {
			value = fmt.Sprintf("'%s'", helpers.Truncate(value, 20, false))
		}
		if variable.Sensitive {
			value = "'**sanitized**'"
		}
		hints = utils.AddRootListHint(hints, "name", variable.Name, fmt.Sprintf("currently set to %s", value))
	}

	hints = utils.AddRootHint(hints, "metadata", "information about this package\n")
	hints = utils.AddRootHint(hints, "build", "info about the machine, zarf version, and user that created this package\n")
	hints = utils.AddRootHint(hints, "components", "components selected for this operation")
	hints = utils.AddRootHint(hints, "constants", "static values set by the package author")
	hints = utils.AddRootHint(hints, "variables", "deployment-specific values that are set on each package deployment")

	return hints
}

type packageMirrorResourcesOptions struct {
	mirrorImages bool
	mirrorRepos  bool
}

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
	v.SetDefault(VInitGitPushUser, state.ZarfGitPushUser)
	v.SetDefault(VInitRegistryPushUser, state.ZarfRegistryPushUser)

	// Always require confirm flag (no viper)
	cmd.Flags().BoolVarP(&config.CommonOptions.Confirm, "confirm", "c", false, lang.CmdPackageDeployFlagConfirm)

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

	// Flags for specifying which resources to mirror
	cmd.Flags().BoolVar(&o.mirrorImages, "images", false, "mirror only the images")
	cmd.Flags().BoolVar(&o.mirrorRepos, "repos", false, "mirror only the git repositories")
	cmd.MarkFlagsMutuallyExclusive("images", "repos")

	return cmd
}

func (o *packageMirrorResourcesOptions) preRun(_ *cobra.Command, _ []string) {
	// If --insecure was provided, set --skip-signature-validation to match
	if config.CommonOptions.Insecure {
		pkgConfig.PkgOpts.SkipSignatureValidation = true
	}

	// post flag validation - perform both if neither were set
	if !o.mirrorImages && !o.mirrorRepos {
		o.mirrorImages = true
		o.mirrorRepos = true
	}
}

func (o *packageMirrorResourcesOptions) run(cmd *cobra.Command, args []string) (err error) {
	ctx := cmd.Context()

	src, err := choosePackage(ctx, args)
	if err != nil {
		return err
	}
	filter := filters.Combine(
		filters.ByLocalOS(runtime.GOOS),
		filters.BySelectState(pkgConfig.PkgOpts.OptionalComponents),
	)

	cachePath, err := getCachePath(ctx)
	if err != nil {
		return err
	}

	loadOpt := packager.LoadOptions{
		Shasum:                  pkgConfig.PkgOpts.Shasum,
		PublicKeyPath:           pkgConfig.PkgOpts.PublicKeyPath,
		SkipSignatureValidation: pkgConfig.PkgOpts.SkipSignatureValidation,
		Filter:                  filter,
		Architecture:            config.GetArch(),
		OCIConcurrency:          config.CommonOptions.OCIConcurrency,
		RemoteOptions:           defaultRemoteOptions(),
		CachePath:               cachePath,
	}
	pkgLayout, err := packager.LoadPackage(ctx, src, loadOpt)
	if err != nil {
		return err
	}
	defer func() {
		// Cleanup package files
		err = errors.Join(err, pkgLayout.Cleanup())
	}()

	images, repos := 0, 0
	// Let's count the images and repos in the package
	for _, component := range pkgLayout.Pkg.Components {
		images += len(component.Images)
		repos += len(component.Repos)
	}
	logger.From(ctx).Debug("package contains images and repos", "images", images, "repos", repos)

	// We don't yet know if the targets are internal or external
	c, _ := cluster.New(ctx) //nolint:errcheck

	if images == 0 && o.mirrorImages {
		logger.From(ctx).Warn("no images found in package to mirror")
	}

	if o.mirrorImages && images > 0 {
		logger.From(ctx).Info("mirroring images", "images", images)
		if pkgConfig.InitOpts.RegistryInfo.Address == "" {
			// if empty flag & zarf state available - execute
			// otherwise return error
			if c == nil {
				return fmt.Errorf("no cluster connection detected - unable to obtain state")
			}
			state, err := c.LoadState(ctx)
			if err != nil {
				return fmt.Errorf("no registry URL provided and no zarf state found")
			}
			logger.From(ctx).Debug("no registry URL provided, using zarf state", "address", state.RegistryInfo.Address)
			pkgConfig.InitOpts.RegistryInfo = state.RegistryInfo
		}
		mirrorOpt := packager.ImagePushOptions{
			Cluster:         c,
			NoImageChecksum: pkgConfig.MirrorOpts.NoImgChecksum,
			Retries:         pkgConfig.PkgOpts.Retries,
			OCIConcurrency:  config.CommonOptions.OCIConcurrency,
			RemoteOptions:   defaultRemoteOptions(),
		}
		err = packager.PushImagesToRegistry(ctx, pkgLayout, pkgConfig.InitOpts.RegistryInfo, mirrorOpt)
		if err != nil {
			return err
		}
	}

	if repos == 0 && o.mirrorRepos {
		logger.From(ctx).Warn("no git repositories found in package to mirror")
	}

	if o.mirrorRepos && repos > 0 {
		logger.From(ctx).Info("mirroring repos", "repos", repos)
		if pkgConfig.InitOpts.GitServer.Address == "" {
			if c == nil {
				return fmt.Errorf("no cluster connection detected - unable to obtain state")
			}
			state, err := c.LoadState(ctx)
			if err != nil {
				return fmt.Errorf("no git URL provided and no zarf state found")
			}
			logger.From(ctx).Debug("no git URL provided, using zarf state", "address", state.GitServer.Address)
			pkgConfig.InitOpts.GitServer = state.GitServer
		}

		mirrorOpt := packager.RepoPushOptions{
			Cluster: c,
			Retries: pkgConfig.PkgOpts.Retries,
		}
		err = packager.PushReposToRepository(ctx, pkgLayout, pkgConfig.InitOpts.GitServer, mirrorOpt)
		if err != nil {
			return err
		}
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
	cmd.AddCommand(newPackageInspectShowManifestsCommand())
	cmd.AddCommand(newPackageInspectDefinitionCommand())
	cmd.AddCommand(newPackageInspectValuesFilesCommand())

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
		sbomOpts := packageInspectSBOMOptions{
			skipSignatureValidation: pkgConfig.PkgOpts.SkipSignatureValidation,
			outputDir:               pkgConfig.InspectOpts.SBOMOutputDir,
		}
		return sbomOpts.run(cmd, args)
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

type packageInspectValuesFilesOptions struct {
	skipSignatureValidation bool
	components              string
	kubeVersion             string
	setVariables            map[string]string
	outputWriter            io.Writer
}

func newPackageInspectValuesFilesOptions() *packageInspectValuesFilesOptions {
	return &packageInspectValuesFilesOptions{
		outputWriter: OutputWriter,
	}
}

func newPackageInspectValuesFilesCommand() *cobra.Command {
	o := newPackageInspectValuesFilesOptions()
	cmd := &cobra.Command{
		Use:   "values-files [ PACKAGE ]",
		Short: "Creates, templates, and outputs the values-files to be sent to each chart",
		Long:  "Creates, templates, and outputs the values-files to be sent to each chart. Does not consider values files builtin to charts",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return o.run(ctx, args)
		},
	}

	cmd.Flags().BoolVar(&o.skipSignatureValidation, "skip-signature-validation", o.skipSignatureValidation, lang.CmdPackageFlagSkipSignatureValidation)
	cmd.Flags().StringVar(&o.components, "components", "", "comma separated list of components to show values files for")
	cmd.Flags().StringVar(&o.kubeVersion, "kube-version", "", lang.CmdDevFlagKubeVersion)
	cmd.Flags().StringToStringVar(&o.setVariables, "set", v.GetStringMapString(VPkgDeploySet), lang.CmdPackageDeployFlagSet)

	return cmd
}

func (o *packageInspectValuesFilesOptions) run(ctx context.Context, args []string) (err error) {
	src, err := choosePackage(ctx, args)
	if err != nil {
		return err
	}
	v := getViper()
	o.setVariables = helpers.TransformAndMergeMap(v.GetStringMapString(VPkgDeploySet), o.setVariables, strings.ToUpper)

	cachePath, err := getCachePath(ctx)
	if err != nil {
		return err
	}

	loadOpts := packager.LoadOptions{
		Architecture:            config.GetArch(),
		PublicKeyPath:           pkgConfig.PkgOpts.PublicKeyPath,
		SkipSignatureValidation: o.skipSignatureValidation,
		LayersSelector:          zoci.ComponentLayers,
		Filter:                  filters.BySelectState(o.components),
		OCIConcurrency:          config.CommonOptions.OCIConcurrency,
		RemoteOptions:           defaultRemoteOptions(),
		CachePath:               cachePath,
	}
	pkgLayout, err := packager.LoadPackage(ctx, src, loadOpts)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, pkgLayout.Cleanup())
	}()

	resourceOpts := packager.InspectPackageResourcesOptions{
		SetVariables: o.setVariables,
		KubeVersion:  o.kubeVersion,
	}
	resources, err := packager.InspectPackageResources(ctx, pkgLayout, resourceOpts)
	if err != nil {
		return err
	}
	resources = slices.DeleteFunc(resources, func(r packager.Resource) bool {
		return r.ResourceType != packager.ValuesFileResource
	})
	if len(resources) == 0 {
		return fmt.Errorf("0 values files found")
	}
	for _, resource := range resources {
		fmt.Fprintf(o.outputWriter, "# associated chart: %s\n", resource.Name)
		fmt.Fprintf(o.outputWriter, "%s---\n", resource.Content)
	}
	return nil
}

type packageInspectManifestsOptions struct {
	skipSignatureValidation bool
	components              string
	kubeVersion             string
	setVariables            map[string]string
	outputWriter            io.Writer
}

func newPackageInspectManifestsOptions() *packageInspectManifestsOptions {
	return &packageInspectManifestsOptions{
		outputWriter: OutputWriter,
	}
}

func newPackageInspectShowManifestsCommand() *cobra.Command {
	o := newPackageInspectManifestsOptions()
	cmd := &cobra.Command{
		Use:   "manifests [ PACKAGE ]",
		Short: "Template and output all manifests and charts in a package",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return o.run(ctx, args)
		},
	}

	cmd.Flags().BoolVar(&o.skipSignatureValidation, "skip-signature-validation", o.skipSignatureValidation, lang.CmdPackageFlagSkipSignatureValidation)
	cmd.Flags().StringVar(&o.components, "components", "", "comma separated list of components to show manifests for")
	cmd.Flags().StringVar(&o.kubeVersion, "kube-version", "", lang.CmdDevFlagKubeVersion)
	cmd.Flags().StringToStringVar(&o.setVariables, "set", v.GetStringMapString(VPkgDeploySet), lang.CmdPackageDeployFlagSet)

	return cmd
}

func (o *packageInspectManifestsOptions) run(ctx context.Context, args []string) (err error) {
	src, err := choosePackage(ctx, args)
	if err != nil {
		return err
	}
	v := getViper()
	o.setVariables = helpers.TransformAndMergeMap(v.GetStringMapString(VPkgDeploySet), o.setVariables, strings.ToUpper)

	cachePath, err := getCachePath(ctx)
	if err != nil {
		return err
	}

	loadOpts := packager.LoadOptions{
		Architecture:            config.GetArch(),
		PublicKeyPath:           pkgConfig.PkgOpts.PublicKeyPath,
		SkipSignatureValidation: o.skipSignatureValidation,
		LayersSelector:          zoci.ComponentLayers,
		Filter:                  filters.BySelectState(o.components),
		OCIConcurrency:          config.CommonOptions.OCIConcurrency,
		RemoteOptions:           defaultRemoteOptions(),
		CachePath:               cachePath,
	}
	pkgLayout, err := packager.LoadPackage(ctx, src, loadOpts)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, pkgLayout.Cleanup())
	}()

	resourceOpts := packager.InspectPackageResourcesOptions{
		SetVariables: o.setVariables,
		KubeVersion:  o.kubeVersion,
	}

	resources, err := packager.InspectPackageResources(ctx, pkgLayout, resourceOpts)
	if err != nil {
		return err
	}
	resources = slices.DeleteFunc(resources, func(r packager.Resource) bool {
		return r.ResourceType == packager.ValuesFileResource
	})
	if len(resources) == 0 {
		return fmt.Errorf("0 manifests found")
	}
	for _, resource := range resources {
		fmt.Fprintf(o.outputWriter, "#type: %s\n", resource.ResourceType)
		// Helm charts already provide a comment on the source when templated
		if resource.ResourceType == packager.ManifestResource {
			fmt.Fprintf(o.outputWriter, "#source: %s\n", resource.Name)
		}
		fmt.Fprintf(o.outputWriter, "%s---\n", resource.Content)
	}
	return nil
}

// packageInspectSBOMOptions holds the command-line options for 'package inspect sbom' sub-command.
type packageInspectSBOMOptions struct {
	skipSignatureValidation bool
	outputDir               string
}

func newPackageInspectSBOMOptions() *packageInspectSBOMOptions {
	return &packageInspectSBOMOptions{
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
		RunE:  o.run,
	}

	cmd.Flags().BoolVar(&o.skipSignatureValidation, "skip-signature-validation", o.skipSignatureValidation, lang.CmdPackageFlagSkipSignatureValidation)
	cmd.Flags().StringVar(&o.outputDir, "output", o.outputDir, lang.CmdPackageCreateFlagSbomOut)

	return cmd
}

// run performs the execution of 'package inspect sbom' sub-command.
func (o *packageInspectSBOMOptions) run(cmd *cobra.Command, args []string) (err error) {
	ctx := cmd.Context()
	src, err := choosePackage(ctx, args)
	if err != nil {
		return err
	}

	cachePath, err := getCachePath(ctx)
	if err != nil {
		return err
	}

	loadOpts := packager.LoadOptions{
		Architecture:            config.GetArch(),
		PublicKeyPath:           pkgConfig.PkgOpts.PublicKeyPath,
		SkipSignatureValidation: o.skipSignatureValidation,
		LayersSelector:          zoci.SbomLayers,
		Filter:                  filters.Empty(),
		OCIConcurrency:          config.CommonOptions.OCIConcurrency,
		RemoteOptions:           defaultRemoteOptions(),
		CachePath:               cachePath,
	}
	pkgLayout, err := packager.LoadPackage(ctx, src, loadOpts)
	if err != nil {
		return fmt.Errorf("unable to load the package: %w", err)
	}

	defer func() {
		err = errors.Join(err, pkgLayout.Cleanup())
	}()
	outputPath := filepath.Join(o.outputDir, pkgLayout.Pkg.Metadata.Name)
	err = pkgLayout.GetSBOM(ctx, outputPath)
	if err != nil {
		return fmt.Errorf("could not get SBOM: %w", err)
	}
	sbomPath, err := filepath.Abs(outputPath)
	if err != nil {
		logger.From(ctx).Warn("SBOM successfully extracted, couldn't get output path", "error", err)
		return nil
	}
	logger.From(ctx).Info("SBOM successfully extracted", "path", sbomPath)
	return nil
}

type packageInspectImagesOptions struct {
	namespaceOverride       string
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

	cmd.Flags().StringVarP(&o.namespaceOverride, "namespace", "n", o.namespaceOverride, lang.CmdPackageInspectFlagNamespace)
	cmd.Flags().BoolVar(&o.skipSignatureValidation, "skip-signature-validation", o.skipSignatureValidation, lang.CmdPackageFlagSkipSignatureValidation)

	return cmd
}

func (o *packageInspectImagesOptions) run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	src, err := choosePackage(ctx, args)
	if err != nil {
		return err
	}

	cachePath, err := getCachePath(ctx)
	if err != nil {
		return err
	}

	cluster, _ := cluster.New(ctx) //nolint: errcheck // package source may or may not be a cluster
	loadOpts := packager.LoadOptions{
		SkipSignatureValidation: o.skipSignatureValidation,
		Architecture:            config.GetArch(),
		Filter:                  filters.Empty(),
		PublicKeyPath:           pkgConfig.PkgOpts.PublicKeyPath,
		OCIConcurrency:          config.CommonOptions.OCIConcurrency,
		RemoteOptions:           defaultRemoteOptions(),
		CachePath:               cachePath,
	}
	pkg, err := packager.GetPackageFromSourceOrCluster(ctx, cluster, src, o.namespaceOverride, loadOpts)
	if err != nil {
		return fmt.Errorf("unable to load the package: %w", err)
	}

	images := make([]string, 0)
	for _, component := range pkg.Components {
		images = append(images, component.Images...)
	}
	images = helpers.Unique(images)
	if len(images) == 0 {
		return fmt.Errorf("no images found in package")
	}

	for _, image := range images {
		fmt.Println("-", image)
	}
	return nil
}

type packageInspectDefinitionOptions struct {
	namespaceOverride       string
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

	cmd.Flags().StringVarP(&o.namespaceOverride, "namespace", "n", o.namespaceOverride, lang.CmdPackageInspectFlagNamespace)
	cmd.Flags().BoolVar(&o.skipSignatureValidation, "skip-signature-validation", o.skipSignatureValidation, lang.CmdPackageFlagSkipSignatureValidation)

	return cmd
}

func (o *packageInspectDefinitionOptions) run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	src, err := choosePackage(ctx, args)
	if err != nil {
		return err
	}

	cachePath, err := getCachePath(ctx)
	if err != nil {
		return err
	}

	cluster, _ := cluster.New(ctx) //nolint: errcheck // package source may or may not be a cluster
	loadOpts := packager.LoadOptions{
		SkipSignatureValidation: o.skipSignatureValidation,
		Architecture:            config.GetArch(),
		Filter:                  filters.Empty(),
		PublicKeyPath:           pkgConfig.PkgOpts.PublicKeyPath,
		OCIConcurrency:          config.CommonOptions.OCIConcurrency,
		RemoteOptions:           defaultRemoteOptions(),
		CachePath:               cachePath,
	}
	pkg, err := packager.GetPackageFromSourceOrCluster(ctx, cluster, src, o.namespaceOverride, loadOpts)
	if err != nil {
		return fmt.Errorf("unable to load the package: %w", err)
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
		outputWriter: OutputWriter,
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
	c, err := cluster.NewWithWait(timeoutCtx)
	if err != nil {
		return err
	}
	o.cluster = c
	return nil
}

// packageListInfo represents the package information for output.
type packageListInfo struct {
	Package           string   `json:"package"`
	NamespaceOverride string   `json:"namespaceOverride"`
	Version           string   `json:"version"`
	Components        []string `json:"components"`
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
			Package:           pkg.Name,
			NamespaceOverride: pkg.NamespaceOverride,
			Version:           pkg.Data.Metadata.Version,
			Components:        components,
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
		header := []string{"Package", "Namespace Override", "Version", "Components"}
		var packageData [][]string
		for _, info := range packageList {
			packageData = append(packageData, []string{
				info.Package, info.NamespaceOverride, info.Version, fmt.Sprintf("%v", info.Components),
			})
		}
		message.TableWithWriter(o.outputWriter, header, packageData)
	default:
		return fmt.Errorf("unsupported output format: %s", o.outputFormat)
	}
	return nil
}

type packageRemoveOptions struct {
	namespaceOverride string
}

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

	cmd.Flags().BoolVarP(&config.CommonOptions.Confirm, "confirm", "c", false, lang.CmdPackageRemoveFlagConfirm)
	cmd.Flags().StringVar(&pkgConfig.PkgOpts.OptionalComponents, "components", v.GetString(VPkgDeployComponents), lang.CmdPackageRemoveFlagComponents)
	cmd.Flags().StringVarP(&o.namespaceOverride, "namespace", "n", v.GetString(VPkgDeployNamespace), lang.CmdPackageRemoveFlagNamespace)
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
	cachePath, err := getCachePath(ctx)
	if err != nil {
		return err
	}
	c, _ := cluster.New(ctx) //nolint:errcheck
	loadOpts := packager.LoadOptions{
		SkipSignatureValidation: pkgConfig.PkgOpts.SkipSignatureValidation,
		Architecture:            config.GetArch(),
		Filter:                  filter,
		PublicKeyPath:           pkgConfig.PkgOpts.PublicKeyPath,
		OCIConcurrency:          config.CommonOptions.OCIConcurrency,
		RemoteOptions:           defaultRemoteOptions(),
		CachePath:               cachePath,
	}
	pkg, err := packager.GetPackageFromSourceOrCluster(ctx, c, packageSource, o.namespaceOverride, loadOpts)
	if err != nil {
		return fmt.Errorf("unable to load the package: %w", err)
	}
	removeOpt := packager.RemoveOptions{
		Cluster:           c,
		Timeout:           config.ZarfDefaultTimeout,
		NamespaceOverride: o.namespaceOverride,
	}
	logger.From(ctx).Info("loaded package for removal", "name", pkg.Metadata.Name)
	err = utils.ColorPrintYAML(pkg, nil, false)
	if err != nil {
		return fmt.Errorf("unable to print package definition: %w", err)
	}
	if !config.CommonOptions.Confirm {
		prompt := &survey.Confirm{
			Message: "Remove this Zarf package?",
		}
		var confirm bool
		if err := survey.AskOne(prompt, &confirm); err != nil || !confirm {
			return fmt.Errorf("package remove cancelled")
		}
	}

	err = packager.Remove(ctx, pkg, removeOpt)
	if err != nil {
		return err
	}
	return nil
}

type packagePublishOptions struct {
	flavor  string
	retries int
}

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
	cmd.Flags().StringVarP(&o.flavor, "flavor", "f", v.GetString(VPkgCreateFlavor), lang.CmdPackagePublishFlagFlavor)
	cmd.Flags().IntVar(&o.retries, "retries", v.GetInt(VPkgPublishRetries), lang.CmdPackageFlagRetries)
	cmd.Flags().BoolVarP(&config.CommonOptions.Confirm, "confirm", "c", false, lang.CmdPackagePublishFlagConfirm)

	return cmd
}

func (o *packagePublishOptions) preRun(_ *cobra.Command, _ []string) {
	// If --insecure was provided, set --skip-signature-validation to match
	if config.CommonOptions.Insecure {
		pkgConfig.PkgOpts.SkipSignatureValidation = true
	}
}

func (o *packagePublishOptions) run(cmd *cobra.Command, args []string) error {
	packageSource := args[0]
	ctx := cmd.Context()
	l := logger.From(ctx)

	if !helpers.IsOCIURL(args[1]) {
		return errors.New("registry must be prefixed with 'oci://'")
	}

	// Destination Repository
	parts := strings.Split(strings.TrimPrefix(args[1], helpers.OCIURLPrefix), "/")
	dstRef := registry.Reference{
		Registry:   parts[0],
		Repository: strings.Join(parts[1:], "/"),
	}
	err := dstRef.ValidateRegistry()
	if err != nil {
		return err
	}

	cachePath, err := getCachePath(ctx)
	if err != nil {
		return err
	}

	// Skeleton package - call PublishSkeleton
	if helpers.IsDir(packageSource) {
		skeletonOpts := packager.PublishSkeletonOptions{
			OCIConcurrency:     config.CommonOptions.OCIConcurrency,
			SigningKeyPath:     pkgConfig.PublishOpts.SigningKeyPath,
			SigningKeyPassword: pkgConfig.PublishOpts.SigningKeyPassword,
			Retries:            o.retries,
			RemoteOptions:      defaultRemoteOptions(),
			CachePath:          cachePath,
			Flavor:             o.flavor,
		}
		_, err = packager.PublishSkeleton(ctx, packageSource, dstRef, skeletonOpts)
		return err
	}

	if helpers.IsOCIURL(packageSource) && pkgConfig.PublishOpts.SigningKeyPath == "" {
		ociOpts := packager.PublishFromOCIOptions{
			OCIConcurrency: config.CommonOptions.OCIConcurrency,
			Architecture:   config.GetArch(),
			RemoteOptions:  defaultRemoteOptions(),
			Retries:        o.retries,
		}

		// source registry reference
		trimmed := strings.TrimPrefix(packageSource, helpers.OCIURLPrefix)
		srcRef, err := registry.ParseReference(trimmed)
		if err != nil {
			return err
		}

		// Grab the package name and append it to the ref.repository to ensure package name and tag/digest match
		srcRepoParts := strings.Split(srcRef.Repository, "/")
		srcPackageName := srcRepoParts[len(srcRepoParts)-1]

		dstRef.Repository = path.Join(dstRef.Repository, srcPackageName)
		dstRef.Reference = srcRef.Reference

		return packager.PublishFromOCI(ctx, srcRef, dstRef, ociOpts)
	}

	if helpers.IsOCIURL(packageSource) && pkgConfig.PublishOpts.SigningKeyPath != "" {
		l.Info("pulling source package locally to sign", "reference", packageSource)
		tmpdir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
		if err != nil {
			return err
		}
		defer func() {
			err = errors.Join(err, os.RemoveAll(tmpdir))
		}()

		packagePath, err := packager.Pull(ctx, packageSource, tmpdir, packager.PullOptions{
			SkipSignatureValidation: pkgConfig.PkgOpts.SkipSignatureValidation,
			PublicKeyPath:           pkgConfig.PkgOpts.PublicKeyPath,
			Architecture:            config.GetArch(),
			OCIConcurrency:          config.CommonOptions.OCIConcurrency,
			RemoteOptions:           defaultRemoteOptions(),
			CachePath:               cachePath,
		})
		if err != nil {
			return fmt.Errorf("failed to pull package: %w", err)
		}
		packageSource = packagePath
	}

	loadOpt := packager.LoadOptions{
		Shasum:                  pkgConfig.PkgOpts.Shasum,
		PublicKeyPath:           pkgConfig.PkgOpts.PublicKeyPath,
		SkipSignatureValidation: pkgConfig.PkgOpts.SkipSignatureValidation,
		Filter:                  filters.Empty(),
		Architecture:            config.GetArch(),
		OCIConcurrency:          config.CommonOptions.OCIConcurrency,
		RemoteOptions:           defaultRemoteOptions(),
		CachePath:               cachePath,
	}
	pkgLayout, err := packager.LoadPackage(ctx, packageSource, loadOpt)
	if err != nil {
		return fmt.Errorf("unable to load package: %w", err)
	}
	defer func() {
		err = errors.Join(err, pkgLayout.Cleanup())
	}()

	publishPackageOpts := packager.PublishPackageOptions{
		OCIConcurrency:     config.CommonOptions.OCIConcurrency,
		SigningKeyPath:     pkgConfig.PublishOpts.SigningKeyPath,
		SigningKeyPassword: pkgConfig.PublishOpts.SigningKeyPassword,
		Retries:            o.retries,
		RemoteOptions:      defaultRemoteOptions(),
	}

	_, err = packager.PublishPackage(ctx, pkgLayout, dstRef, publishPackageOpts)
	return err
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
	srcURL := args[0]
	outputDir := pkgConfig.PullOpts.OutputDirectory
	ctx := cmd.Context()
	if outputDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		outputDir = wd
	}
	cachePath, err := getCachePath(ctx)
	if err != nil {
		return err
	}
	packagePath, err := packager.Pull(ctx, srcURL, outputDir, packager.PullOptions{
		SHASum:                  pkgConfig.PkgOpts.Shasum,
		SkipSignatureValidation: pkgConfig.PkgOpts.SkipSignatureValidation,
		PublicKeyPath:           pkgConfig.PkgOpts.PublicKeyPath,
		Architecture:            config.GetArch(),
		OCIConcurrency:          config.CommonOptions.OCIConcurrency,
		RemoteOptions:           defaultRemoteOptions(),
		CachePath:               cachePath,
	})
	if err != nil {
		return err
	}
	logger.From(cmd.Context()).Info("package downloaded successful", "path", packagePath)
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

	c, err := cluster.New(cmd.Context())
	if err != nil {
		return pkgCandidates, cobra.ShellCompDirectiveDefault
	}

	ctx := cmd.Context()

	deployedZarfPackages, err := c.GetDeployedZarfPackages(ctx)
	if err != nil {
		logger.From(cmd.Context()).Debug("unable to get deployed zarf packages for package completion args", "error", err)
	}
	// Populate list of package names
	for _, pkg := range deployedZarfPackages {
		pkgCandidates = append(pkgCandidates, pkg.Name)
	}

	return pkgCandidates, cobra.ShellCompDirectiveDefault
}
