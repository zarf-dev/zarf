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
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/pkg/helpers/v2"
	goyaml "github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"oras.land/oras-go/v2/registry"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/images"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/packager"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/value"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
)

func newPackageCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "package",
		Aliases: []string{"p"},
		Short:   lang.CmdPackageShort,
	}

	v := getViper()

	cmd.AddCommand(newPackageCreateCommand(v))
	cmd.AddCommand(newPackageDeployCommand(v))
	cmd.AddCommand(newPackageMirrorResourcesCommand(v))
	cmd.AddCommand(newPackageInspectCommand(v))
	cmd.AddCommand(newPackageRemoveCommand(v))
	cmd.AddCommand(newPackageListCommand())
	cmd.AddCommand(newPackagePublishCommand(v))
	cmd.AddCommand(newPackagePullCommand(v))
	cmd.AddCommand(newPackageSignCommand(v))
	cmd.AddCommand(newPackageVerifyCommand(v))

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
	registryOverrides       []string
	signingKeyPath          string
	signingKeyPassword      string
	flavor                  string
	ociConcurrency          int
	skipVersionCheck        bool
	withBuildMachineInfo    bool
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
	cmd.Flags().IntVar(&o.ociConcurrency, "oci-concurrency", v.GetInt(VPkgOCIConcurrency), lang.CmdPackageFlagConcurrency)

	outputDirectory := v.GetString("package.create.output_directory")
	output := v.GetString(VPkgCreateOutput)
	if outputDirectory != "" && output == "" {
		v.Set(VPkgCreateOutput, outputDirectory)
	}
	cmd.Flags().StringVar(&o.output, "output-directory", v.GetString("package.create.output_directory"), lang.CmdPackageCreateFlagOutput)
	cmd.Flags().StringVarP(&o.output, "output", "o", v.GetString(VPkgCreateOutput), lang.CmdPackageCreateFlagOutput)

	cmd.Flags().StringVar(&o.differentialPackagePath, "differential", v.GetString(VPkgCreateDifferential), lang.CmdPackageCreateFlagDifferential)
	cmd.Flags().StringToStringVar(&o.setVariables, "set", v.GetStringMapString(VPkgCreateSet), lang.CmdPackageCreateFlagSetPkgTmpl)
	cmd.Flags().BoolVarP(&o.sbom, "sbom", "s", v.GetBool(VPkgCreateSbom), lang.CmdPackageCreateFlagSbom)
	cmd.Flags().StringVar(&o.sbomOutput, "sbom-out", v.GetString(VPkgCreateSbomOutput), lang.CmdPackageCreateFlagSbomOut)
	cmd.Flags().BoolVar(&o.skipSBOM, "skip-sbom", v.GetBool(VPkgCreateSkipSbom), lang.CmdPackageCreateFlagSkipSbom)
	cmd.Flags().IntVarP(&o.maxPackageSizeMB, "max-package-size", "m", v.GetInt(VPkgCreateMaxPackageSize), lang.CmdPackageCreateFlagMaxPackageSize)
	cmd.Flags().StringSliceVar(&o.registryOverrides, "registry-override", GetStringSlice(v, VPkgCreateRegistryOverride), lang.CmdPackageCreateFlagRegistryOverride)
	cmd.Flags().StringVarP(&o.flavor, "flavor", "f", v.GetString(VPkgCreateFlavor), lang.CmdPackageCreateFlagFlavor)
	cmd.Flags().BoolVar(&o.skipVersionCheck, "skip-version-check", false, "Ignore version requirements when deploying the package")
	_ = cmd.Flags().MarkHidden("skip-version-check")

	cmd.Flags().StringVar(&o.signingKeyPath, "signing-key", v.GetString(VPkgCreateSigningKey), lang.CmdPackageCreateFlagSigningKey)
	cmd.Flags().StringVar(&o.signingKeyPassword, "signing-key-pass", v.GetString(VPkgCreateSigningKeyPassword), lang.CmdPackageCreateFlagSigningKeyPassword)

	cmd.Flags().BoolVar(&o.withBuildMachineInfo, "with-build-machine-info", v.GetBool(VPkgCreateWithBuildMachineInfo), lang.CmdPackageCreateFlagWithBuildMachineInfo)

	cmd.Flags().StringVarP(&o.signingKeyPath, "key", "k", v.GetString(VPkgCreateSigningKey), lang.CmdPackageCreateFlagDeprecatedKey)
	cmd.Flags().StringVar(&o.signingKeyPassword, "key-pass", v.GetString(VPkgCreateSigningKeyPassword), lang.CmdPackageCreateFlagDeprecatedKeyPassword)

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

// Converts registry overrides to a structured type.
// The result will be sorted in descending order.
// Descending order guarantees the longest prefix will be sorted toward the beginning.
//
// Input is of the following form:
// []string{"docker.io/library=docker.example.com", "docker.io=docker.example.com"}
func parseRegistryOverrides(overrides []string) ([]images.RegistryOverride, error) {
	result := make([]images.RegistryOverride, len(overrides))
	for i, mapping := range overrides {
		source, override, found := strings.Cut(mapping, "=")
		if !found {
			return nil, fmt.Errorf("registry override missing '=': %s", mapping)
		}

		if source == "" {
			return nil, fmt.Errorf("registry override missing source: %s", mapping)
		}

		if override == "" {
			return nil, fmt.Errorf("registry override missing value: %s", mapping)
		}

		if index := slices.IndexFunc(result, func(existing images.RegistryOverride) bool {
			return existing.Source == source
		}); index >= 0 {
			return nil, fmt.Errorf("registry override has duplicate source: existing index %d, new index %d, source %s", index, i, source)
		}

		result[i].Source = source
		result[i].Override = override
	}

	// We sort these now at parse time so they are handled correctly throughout execution.
	slices.SortFunc(result, func(a images.RegistryOverride, b images.RegistryOverride) int {
		return -strings.Compare(a.Source, b.Source)
	})

	return result, nil
}

func (o *packageCreateOptions) run(ctx context.Context, args []string) error {
	l := logger.From(ctx)
	basePath := setBaseDirectory(args)

	var isCleanPathRegex = regexp.MustCompile(`^[a-zA-Z0-9\_\-\/\.\~\\:]+$`)
	if !isCleanPathRegex.MatchString(config.CommonOptions.CachePath) {
		l.Warn("invalid characters in Zarf cache path, using default", "cfg", config.ZarfDefaultCachePath, "default", config.ZarfDefaultCachePath)
		config.CommonOptions.CachePath = config.ZarfDefaultCachePath
	}

	v := getViper()
	o.setVariables = helpers.TransformAndMergeMap(v.GetStringMapString(VPkgCreateSet), o.setVariables, strings.ToUpper)
	overrides, err := parseRegistryOverrides(o.registryOverrides)
	if err != nil {
		return fmt.Errorf("error parsing registry override: %w", err)
	}
	l.Debug("parsed registry overrides", "overrides", overrides)

	cachePath, err := getCachePath(ctx)
	if err != nil {
		return err
	}
	opt := packager.CreateOptions{
		Flavor:                  o.flavor,
		RegistryOverrides:       overrides,
		SigningKeyPath:          o.signingKeyPath,
		SigningKeyPassword:      o.signingKeyPassword,
		SetVariables:            o.setVariables,
		MaxPackageSizeMB:        o.maxPackageSizeMB,
		SBOMOut:                 o.sbomOutput,
		SkipSBOM:                o.skipSBOM,
		OCIConcurrency:          o.ociConcurrency,
		DifferentialPackagePath: o.differentialPackagePath,
		RemoteOptions:           defaultRemoteOptions(),
		CachePath:               cachePath,
		IsInteractive:           !o.confirm,
		SkipVersionCheck:        o.skipVersionCheck,
		WithBuildMachineInfo:    o.withBuildMachineInfo,
	}
	pkgPath, err := packager.Create(ctx, basePath, o.output, opt)
	// NOTE(mkcp): LintErrors are rendered with a table
	var lintErr *lint.LintError
	if errors.As(err, &lintErr) {
		PrintFindings(ctx, lintErr)
	}
	if err != nil {
		return fmt.Errorf("failed to create package: %w", err)
	}
	l.Debug("package created", "path", pkgPath)
	return nil
}

type packageDeployOptions struct {
	valuesFiles             []string
	namespaceOverride       string
	confirm                 bool
	adoptExistingResources  bool
	timeout                 time.Duration
	retries                 int
	setVariables            map[string]string
	setValues               map[string]string
	optionalComponents      string
	shasum                  string
	verify                  bool
	skipSignatureValidation bool
	SkipVersionCheck        bool
	ociConcurrency          int
	publicKeyPath           string
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
	cmd.Flags().BoolVarP(&o.confirm, "confirm", "c", false, lang.CmdPackageDeployFlagConfirm)
	cmd.Flags().IntVar(&o.ociConcurrency, "oci-concurrency", v.GetInt(VPkgOCIConcurrency), lang.CmdPackageFlagConcurrency)
	cmd.Flags().StringVarP(&o.publicKeyPath, "key", "k", v.GetString(VPkgPublicKey), lang.CmdPackageFlagFlagPublicKey)

	// Always require adopt-existing-resources flag (no viper)
	cmd.Flags().BoolVar(&o.adoptExistingResources, "adopt-existing-resources", false, lang.CmdPackageDeployFlagAdoptExistingResources)
	cmd.Flags().DurationVar(&o.timeout, "timeout", v.GetDuration(VPkgDeployTimeout), lang.CmdPackageDeployFlagTimeout)

	cmd.Flags().StringSliceVarP(&o.valuesFiles, "values", "v", GetStringSlice(v, VPkgDeployValues), lang.CmdPackageDeployFlagValuesFiles)
	cmd.Flags().IntVar(&o.retries, "retries", v.GetInt(VPkgRetries), lang.CmdPackageFlagRetries)
	cmd.Flags().StringToStringVar(&o.setVariables, "set", v.GetStringMapString(VPkgDeploySet), "Alias for --set-variables")
	_ = cmd.Flags().MarkDeprecated("set", "Use --set-variables instead")
	cmd.Flags().StringToStringVar(&o.setVariables, "set-variables", v.GetStringMapString(VPkgDeploySet), lang.CmdPackageDeployFlagSetVariables)
	cmd.Flags().StringToStringVar(&o.setValues, "set-values", v.GetStringMapString(VPkgDeploySetValues), lang.CmdPackageDeployFlagSetValues)
	cmd.Flags().StringVar(&o.optionalComponents, "components", v.GetString(VPkgDeployComponents), lang.CmdPackageDeployFlagComponents)
	cmd.Flags().StringVar(&o.shasum, "shasum", v.GetString(VPkgDeployShasum), lang.CmdPackageDeployFlagShasum)
	cmd.Flags().StringVarP(&o.namespaceOverride, "namespace", "n", v.GetString(VPkgDeployNamespace), lang.CmdPackageDeployFlagNamespace)
	cmd.Flags().BoolVar(&o.skipSignatureValidation, "skip-signature-validation", false, lang.CmdPackageFlagSkipSignatureValidation)
	cmd.Flags().BoolVar(&o.verify, "verify", v.GetBool(VPkgVerify), lang.CmdPackageFlagVerify)
	cmd.Flags().BoolVar(&o.SkipVersionCheck, "skip-version-check", false, "Ignore version requirements when deploying the package")
	_ = cmd.Flags().MarkHidden("skip-version-check")
	errSig := cmd.Flags().MarkDeprecated("skip-signature-validation", "Signature verification now occurs on every execution, but is not enforced by default. Use --verify to enforce validation. This flag will be removed in Zarf v1.0.0.")
	if errSig != nil {
		logger.Default().Debug("unable to mark flag skip-signature-validation", "error", errSig)
	}
	return cmd
}

func (o *packageDeployOptions) preRun(cmd *cobra.Command, _ []string) {
	// Handle deprecated --skip-signature-validation flag for backwards compatibility
	if cmd.Flags().Changed("skip-signature-validation") {
		logger.Default().Warn("--skip-signature-validation is deprecated and will be removed in v1.0.0. Use --verify to enforce signature validation.")

		if cmd.Flags().Changed("verify") {
			return
		}

		o.verify = !o.skipSignatureValidation
	}
}

func (o *packageDeployOptions) run(cmd *cobra.Command, args []string) (err error) {
	ctx := cmd.Context()
	packageSource, err := choosePackage(ctx, args)
	if err != nil {
		return err
	}

	v := getViper()

	// Merge variables
	o.setVariables = helpers.TransformAndMergeMap(
		v.GetStringMapString(VPkgDeploySet),
		o.setVariables,
		strings.ToUpper,
	)
	// Merge values
	maps.Copy(o.setValues, v.GetStringMapString(VPkgDeploySetValues))

	// Load files supplied by --values / -v or a user's zarf-config.{yaml,toml}
	values, err := value.ParseFiles(ctx, o.valuesFiles, value.ParseFilesOptions{})
	if err != nil {
		return err
	}

	// Apply CLI --set-values overrides last
	for key, val := range o.setValues {
		p := value.Path(key)
		if !strings.HasPrefix(key, ".") {
			p = value.Path("." + key)
		}
		if err := values.Set(p, val); err != nil {
			return fmt.Errorf("unable to set value at path %s: %w", key, err)
		}
	}

	cachePath, err := getCachePath(ctx)
	if err != nil {
		return err
	}

	loadOpt := packager.LoadOptions{
		Shasum:               o.shasum,
		PublicKeyPath:        o.publicKeyPath,
		VerificationStrategy: getVerificationStrategy(o.verify),
		Filter:               filters.Empty(),
		Architecture:         config.GetArch(),
		OCIConcurrency:       o.ociConcurrency,
		RemoteOptions:        defaultRemoteOptions(),
		CachePath:            cachePath,
	}
	pkgLayout, err := packager.LoadPackage(ctx, packageSource, loadOpt)
	if err != nil {
		return fmt.Errorf("unable to load package: %w", err)
	}
	defer func() {
		err = errors.Join(err, pkgLayout.Cleanup())
	}()

	deployOpts := packager.DeployOptions{
		Values:                 values,
		AdoptExistingResources: o.adoptExistingResources,
		Timeout:                o.timeout,
		Retries:                o.retries,
		OCIConcurrency:         o.ociConcurrency,
		SetVariables:           o.setVariables,
		NamespaceOverride:      o.namespaceOverride,
		RemoteOptions:          defaultRemoteOptions(),
		IsInteractive:          !o.confirm,
		SkipVersionCheck:       o.SkipVersionCheck,
	}

	deployedComponents, err := deploy(ctx, pkgLayout, deployOpts, o.setVariables, o.optionalComponents)
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

func deploy(ctx context.Context, pkgLayout *layout.PackageLayout, opts packager.DeployOptions, setVariables map[string]string, optionalComponents string) ([]state.DeployedComponent, error) {
	// Intentionally duplicate the deploy override logic here to allow us to render the updated package in confirm below
	if opts.NamespaceOverride != "" {
		if err := packager.OverridePackageNamespace(pkgLayout.Pkg, opts.NamespaceOverride); err != nil {
			return nil, err
		}
	}
	err := confirmDeploy(ctx, pkgLayout, setVariables, opts.IsInteractive)
	if err != nil {
		return nil, err
	}

	// filter after confirmation to allow users to view the entire package interactively
	filter := filters.Combine(
		filters.ByLocalOS(runtime.GOOS),
		filters.ForDeploy(optionalComponents, opts.IsInteractive),
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

func confirmDeploy(ctx context.Context, pkgLayout *layout.PackageLayout, setVariables map[string]string, isInteractive bool) (err error) {
	l := logger.From(ctx)

	err = utils.ColorPrintYAML(pkgLayout.Pkg, getPackageYAMLHints(pkgLayout.Pkg, setVariables), false)
	if err != nil {
		return fmt.Errorf("unable to print package definition: %w", err)
	}

	if len(pkgLayout.Pkg.Documentation) > 0 {
		l.Info("documentation available for this package - use 'zarf package inspect documentation' to view")
	}

	if pkgLayout.Pkg.IsSBOMAble() && !pkgLayout.ContainsSBOM() {
		l.Warn("this package does NOT contain an SBOM. If you require an SBOM, the package must be built without the --skip-sbom flag")
	}
	if pkgLayout.ContainsSBOM() && isInteractive {
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

	if !isInteractive {
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

	return hints
}

type packageMirrorResourcesOptions struct {
	mirrorImages            bool
	mirrorRepos             bool
	confirm                 bool
	shasum                  string
	noImgChecksum           bool
	verify                  bool
	skipSignatureValidation bool
	retries                 int
	optionalComponents      string
	gitServer               state.GitServerInfo
	registryInfo            state.RegistryInfo
	ociConcurrency          int
	publicKeyPath           string
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
	cmd.Flags().BoolVarP(&o.confirm, "confirm", "c", false, lang.CmdPackageDeployFlagConfirm)
	cmd.Flags().IntVar(&o.ociConcurrency, "oci-concurrency", v.GetInt(VPkgOCIConcurrency), lang.CmdPackageFlagConcurrency)
	cmd.Flags().StringVarP(&o.publicKeyPath, "key", "k", v.GetString(VPkgPublicKey), lang.CmdPackageFlagFlagPublicKey)

	cmd.Flags().StringVar(&o.shasum, "shasum", "", lang.CmdPackagePullFlagShasum)
	cmd.Flags().BoolVar(&o.noImgChecksum, "no-img-checksum", false, lang.CmdPackageMirrorFlagNoChecksum)

	cmd.Flags().BoolVar(&o.verify, "verify", v.GetBool(VPkgVerify), lang.CmdPackageFlagVerify)
	cmd.Flags().BoolVar(&o.skipSignatureValidation, "skip-signature-validation", false, lang.CmdPackageFlagSkipSignatureValidation)
	errSig := cmd.Flags().MarkDeprecated("skip-signature-validation", "Signature verification now occurs on every execution, but is not enforced by default. Use --verify to enforce validation. This flag will be removed in Zarf v1.0.0.")
	if errSig != nil {
		logger.Default().Debug("unable to mark skip-signature-validation", "error", errSig)
	}

	cmd.Flags().IntVar(&o.retries, "retries", v.GetInt(VPkgRetries), lang.CmdPackageFlagRetries)
	cmd.Flags().StringVar(&o.optionalComponents, "components", v.GetString(VPkgDeployComponents), lang.CmdPackageMirrorFlagComponents)

	// Flags for using an external Git server
	cmd.Flags().StringVar(&o.gitServer.Address, "git-url", v.GetString(VInitGitURL), lang.CmdInitFlagGitURL)
	cmd.Flags().StringVar(&o.gitServer.PushUsername, "git-push-username", v.GetString(VInitGitPushUser), lang.CmdInitFlagGitPushUser)
	cmd.Flags().StringVar(&o.gitServer.PushPassword, "git-push-password", v.GetString(VInitGitPushPass), lang.CmdInitFlagGitPushPass)

	// Flags for using an external registry
	cmd.Flags().StringVar(&o.registryInfo.Address, "registry-url", v.GetString(VInitRegistryURL), lang.CmdInitFlagRegURL)
	cmd.Flags().StringVar(&o.registryInfo.PushUsername, "registry-push-username", v.GetString(VInitRegistryPushUser), lang.CmdInitFlagRegPushUser)
	cmd.Flags().StringVar(&o.registryInfo.PushPassword, "registry-push-password", v.GetString(VInitRegistryPushPass), lang.CmdInitFlagRegPushPass)

	// Flags for specifying which resources to mirror
	cmd.Flags().BoolVar(&o.mirrorImages, "images", false, "mirror only the images")
	cmd.Flags().BoolVar(&o.mirrorRepos, "repos", false, "mirror only the git repositories")
	cmd.MarkFlagsMutuallyExclusive("images", "repos")

	return cmd
}

func (o *packageMirrorResourcesOptions) preRun(cmd *cobra.Command, _ []string) {
	// Handle deprecated --skip-signature-validation flag for backwards compatibility
	if cmd.Flags().Changed("skip-signature-validation") {
		logger.Default().Warn("--skip-signature-validation is deprecated and will be removed in v1.0.0. Use --verify to enforce signature validation.")

		if cmd.Flags().Changed("verify") {
			return
		}

		o.verify = !o.skipSignatureValidation
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
		filters.BySelectState(o.optionalComponents),
	)

	cachePath, err := getCachePath(ctx)
	if err != nil {
		return err
	}

	loadOpt := packager.LoadOptions{
		Shasum:               o.shasum,
		PublicKeyPath:        o.publicKeyPath,
		VerificationStrategy: getVerificationStrategy(o.verify),
		Filter:               filter,
		Architecture:         config.GetArch(),
		OCIConcurrency:       o.ociConcurrency,
		RemoteOptions:        defaultRemoteOptions(),
		CachePath:            cachePath,
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
		images += len(component.GetImages())
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
		if o.registryInfo.Address == "" {
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
			o.registryInfo = state.RegistryInfo
		}
		mirrorOpt := packager.ImagePushOptions{
			Cluster:         c,
			NoImageChecksum: o.noImgChecksum,
			Retries:         o.retries,
			OCIConcurrency:  o.ociConcurrency,
			RemoteOptions:   defaultRemoteOptions(),
		}
		err = packager.PushImagesToRegistry(ctx, pkgLayout, o.registryInfo, mirrorOpt)
		if err != nil {
			return err
		}
	}

	if repos == 0 && o.mirrorRepos {
		logger.From(ctx).Warn("no git repositories found in package to mirror")
	}

	if o.mirrorRepos && repos > 0 {
		logger.From(ctx).Info("mirroring repos", "repos", repos)
		if o.gitServer.Address == "" {
			if c == nil {
				return fmt.Errorf("no cluster connection detected - unable to obtain state")
			}
			state, err := c.LoadState(ctx)
			if err != nil {
				return fmt.Errorf("no git URL provided and no zarf state found")
			}
			logger.From(ctx).Debug("no git URL provided, using zarf state", "address", state.GitServer.Address)
			o.gitServer = state.GitServer
		}

		mirrorOpt := packager.RepoPushOptions{
			Cluster: c,
			Retries: o.retries,
		}
		err = packager.PushReposToRepository(ctx, pkgLayout, o.gitServer, mirrorOpt)
		if err != nil {
			return err
		}
	}
	return nil
}

func newPackageInspectCommand(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "inspect [ PACKAGE_SOURCE ]",
		Aliases: []string{"i"},
		Short:   lang.CmdPackageInspectShort,
	}

	cmd.AddCommand(newPackageInspectSBOMCommand(v))
	cmd.AddCommand(newPackageInspectImagesCommand(v))
	cmd.AddCommand(newPackageInspectManifestsCommand(v))
	cmd.AddCommand(newPackageInspectDefinitionCommand(v))
	cmd.AddCommand(newPackageInspectValuesFilesCommand(v))
	cmd.AddCommand(newPackageInspectDocumentationCommand(v))
	return cmd
}

type packageInspectValuesFilesOptions struct {
	verify                  bool
	skipSignatureValidation bool
	components              string
	kubeVersion             string
	setVariables            map[string]string
	outputWriter            io.Writer
	ociConcurrency          int
	publicKeyPath           string
}

func newPackageInspectValuesFilesOptions() *packageInspectValuesFilesOptions {
	return &packageInspectValuesFilesOptions{
		outputWriter: OutputWriter,
	}
}

func newPackageInspectValuesFilesCommand(v *viper.Viper) *cobra.Command {
	o := newPackageInspectValuesFilesOptions()
	cmd := &cobra.Command{
		Use:    "values-files [ PACKAGE ]",
		Short:  "Creates, templates, and outputs the values-files to be sent to each chart",
		Long:   "Creates, templates, and outputs the values-files to be sent to each chart. Does not consider values files builtin to charts",
		Args:   cobra.MaximumNArgs(1),
		PreRun: o.preRun,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return o.run(ctx, args)
		},
	}

	cmd.Flags().IntVar(&o.ociConcurrency, "oci-concurrency", v.GetInt(VPkgOCIConcurrency), lang.CmdPackageFlagConcurrency)
	cmd.Flags().StringVarP(&o.publicKeyPath, "key", "k", v.GetString(VPkgPublicKey), lang.CmdPackageFlagFlagPublicKey)
	cmd.Flags().BoolVar(&o.skipSignatureValidation, "skip-signature-validation", o.skipSignatureValidation, lang.CmdPackageFlagSkipSignatureValidation)
	cmd.Flags().BoolVar(&o.verify, "verify", v.GetBool(VPkgVerify), lang.CmdPackageFlagVerify)
	cmd.Flags().StringVar(&o.components, "components", "", "comma separated list of components to show values files for")
	cmd.Flags().StringVar(&o.kubeVersion, "kube-version", "", lang.CmdDevFlagKubeVersion)
	cmd.Flags().StringToStringVar(&o.setVariables, "set", v.GetStringMapString(VPkgDeploySet), "Alias for --set-variables")
	_ = cmd.Flags().MarkDeprecated("set", "use --set-variables instead")
	cmd.Flags().StringToStringVar(&o.setVariables, "set-variables", v.GetStringMapString(VPkgDeploySet), lang.CmdPackageDeployFlagSetVariables)
	errSig := cmd.Flags().MarkDeprecated("skip-signature-validation", "Signature verification now occurs on every execution, but is not enforced by default. Use --verify to enforce validation. This flag will be removed in Zarf v1.0.0.")
	if errSig != nil {
		logger.Default().Debug("unable to mark skip-signature-validation", "error", errSig)
	}
	return cmd
}

func (o *packageInspectValuesFilesOptions) preRun(cmd *cobra.Command, _ []string) {
	// Handle deprecated --skip-signature-validation flag for backwards compatibility
	if cmd.Flags().Changed("skip-signature-validation") {
		logger.Default().Warn("--skip-signature-validation is deprecated and will be removed in v1.0.0. Use --verify to enforce signature validation.")

		if cmd.Flags().Changed("verify") {
			return
		}

		o.verify = !o.skipSignatureValidation
	}
}

func (o *packageInspectValuesFilesOptions) run(ctx context.Context, args []string) (err error) {
	src, err := choosePackage(ctx, args)
	if err != nil {
		return err
	}
	v := getViper()

	// Merge SetVariables and config variables.
	o.setVariables = helpers.TransformAndMergeMap(v.GetStringMapString(VPkgDeploySet), o.setVariables, strings.ToUpper)

	cachePath, err := getCachePath(ctx)
	if err != nil {
		return err
	}

	loadOpts := packager.LoadOptions{
		Architecture:         config.GetArch(),
		PublicKeyPath:        o.publicKeyPath,
		VerificationStrategy: getVerificationStrategy(o.verify),
		LayersSelector:       zoci.ComponentLayers,
		Filter:               filters.BySelectState(o.components),
		OCIConcurrency:       o.ociConcurrency,
		RemoteOptions:        defaultRemoteOptions(),
		CachePath:            cachePath,
	}
	pkgLayout, err := packager.LoadPackage(ctx, src, loadOpts)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, pkgLayout.Cleanup())
	}()

	resourceOpts := packager.InspectPackageResourcesOptions{
		SetVariables:  o.setVariables,
		KubeVersion:   o.kubeVersion,
		IsInteractive: true,
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
	verify                  bool
	skipSignatureValidation bool
	components              string
	kubeVersion             string
	setVariables            map[string]string
	outputWriter            io.Writer
	ociConcurrency          int
	publicKeyPath           string
}

func newPackageInspectManifestsOptions() *packageInspectManifestsOptions {
	return &packageInspectManifestsOptions{
		outputWriter: OutputWriter,
	}
}

func newPackageInspectManifestsCommand(v *viper.Viper) *cobra.Command {
	o := newPackageInspectManifestsOptions()
	cmd := &cobra.Command{
		Use:    "manifests [ PACKAGE ]",
		Short:  "Template and output all manifests and charts in a package",
		Args:   cobra.MaximumNArgs(1),
		PreRun: o.preRun,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return o.run(ctx, args)
		},
	}

	cmd.Flags().IntVar(&o.ociConcurrency, "oci-concurrency", v.GetInt(VPkgOCIConcurrency), lang.CmdPackageFlagConcurrency)
	cmd.Flags().StringVarP(&o.publicKeyPath, "key", "k", v.GetString(VPkgPublicKey), lang.CmdPackageFlagFlagPublicKey)
	cmd.Flags().BoolVar(&o.skipSignatureValidation, "skip-signature-validation", o.skipSignatureValidation, lang.CmdPackageFlagSkipSignatureValidation)
	cmd.Flags().BoolVar(&o.verify, "verify", v.GetBool(VPkgVerify), lang.CmdPackageFlagVerify)
	cmd.Flags().StringVar(&o.components, "components", "", "comma separated list of components to show manifests for")
	cmd.Flags().StringVar(&o.kubeVersion, "kube-version", "", lang.CmdDevFlagKubeVersion)
	cmd.Flags().StringToStringVar(&o.setVariables, "set", v.GetStringMapString(VPkgDeploySet), "Alias for --set-variables")
	_ = cmd.Flags().MarkDeprecated("set", "use --set-variables instead")
	cmd.Flags().StringToStringVar(&o.setVariables, "set-variables", v.GetStringMapString(VPkgDeploySet), lang.CmdPackageDeployFlagSetVariables)
	errSig := cmd.Flags().MarkDeprecated("skip-signature-validation", "Signature verification now occurs on every execution, but is not enforced by default. Use --verify to enforce validation. This flag will be removed in Zarf v1.0.0.")
	if errSig != nil {
		logger.Default().Debug("unable to mark skip-signature-validation", "error", errSig)
	}
	return cmd
}

func (o *packageInspectManifestsOptions) preRun(cmd *cobra.Command, _ []string) {
	// Handle deprecated --skip-signature-validation flag for backwards compatibility
	if cmd.Flags().Changed("skip-signature-validation") {
		logger.Default().Warn("--skip-signature-validation is deprecated and will be removed in v1.0.0. Use --verify to enforce signature validation.")

		if cmd.Flags().Changed("verify") {
			return
		}

		o.verify = !o.skipSignatureValidation
	}
}

func (o *packageInspectManifestsOptions) run(ctx context.Context, args []string) (err error) {
	src, err := choosePackage(ctx, args)
	if err != nil {
		return err
	}
	v := getViper()

	// Merge SetVariables and config variables.
	o.setVariables = helpers.TransformAndMergeMap(v.GetStringMapString(VPkgDeploySet), o.setVariables, strings.ToUpper)

	cachePath, err := getCachePath(ctx)
	if err != nil {
		return err
	}

	loadOpts := packager.LoadOptions{
		Architecture:         config.GetArch(),
		PublicKeyPath:        o.publicKeyPath,
		VerificationStrategy: getVerificationStrategy(o.verify),
		LayersSelector:       zoci.ComponentLayers,
		Filter:               filters.BySelectState(o.components),
		OCIConcurrency:       o.ociConcurrency,
		RemoteOptions:        defaultRemoteOptions(),
		CachePath:            cachePath,
	}
	pkgLayout, err := packager.LoadPackage(ctx, src, loadOpts)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, pkgLayout.Cleanup())
	}()

	resourceOpts := packager.InspectPackageResourcesOptions{
		SetVariables:  o.setVariables,
		KubeVersion:   o.kubeVersion,
		IsInteractive: true,
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
	verify                  bool
	skipSignatureValidation bool
	outputDir               string
	ociConcurrency          int
	publicKeyPath           string
}

func newPackageInspectSBOMOptions() *packageInspectSBOMOptions {
	return &packageInspectSBOMOptions{
		outputDir: "",
		verify:    false,
	}
}

// newPackageInspectSBOMCommand creates the `package inspect sbom` sub-command.
func newPackageInspectSBOMCommand(v *viper.Viper) *cobra.Command {
	o := newPackageInspectSBOMOptions()
	cmd := &cobra.Command{
		Use:    "sbom [ PACKAGE ]",
		Short:  "Output the package SBOM (Software Bill Of Materials) to the specified directory",
		Args:   cobra.MaximumNArgs(1),
		PreRun: o.preRun,
		RunE:   o.run,
	}

	cmd.Flags().IntVar(&o.ociConcurrency, "oci-concurrency", v.GetInt(VPkgOCIConcurrency), lang.CmdPackageFlagConcurrency)
	cmd.Flags().StringVarP(&o.publicKeyPath, "key", "k", v.GetString(VPkgPublicKey), lang.CmdPackageFlagFlagPublicKey)
	cmd.Flags().BoolVar(&o.skipSignatureValidation, "skip-signature-validation", o.skipSignatureValidation, lang.CmdPackageFlagSkipSignatureValidation)
	cmd.Flags().BoolVar(&o.verify, "verify", v.GetBool(VPkgVerify), lang.CmdPackageFlagVerify)
	cmd.Flags().StringVar(&o.outputDir, "output", o.outputDir, lang.CmdPackageCreateFlagSbomOut)
	errSig := cmd.Flags().MarkDeprecated("skip-signature-validation", "Signature verification now occurs on every execution, but is not enforced by default. Use --verify to enforce validation. This flag will be removed in Zarf v1.0.0.")
	if errSig != nil {
		logger.Default().Debug("unable to mark skip-signature-validation", "error", errSig)
	}
	return cmd
}

func (o *packageInspectSBOMOptions) preRun(cmd *cobra.Command, _ []string) {
	// Handle deprecated --skip-signature-validation flag for backwards compatibility
	if cmd.Flags().Changed("skip-signature-validation") {
		logger.Default().Warn("--skip-signature-validation is deprecated and will be removed in v1.0.0. Use --verify to enforce signature validation.")

		if cmd.Flags().Changed("verify") {
			return
		}

		o.verify = !o.skipSignatureValidation
	}
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
		Architecture:         config.GetArch(),
		PublicKeyPath:        o.publicKeyPath,
		VerificationStrategy: getVerificationStrategy(o.verify),
		LayersSelector:       zoci.SbomLayers,
		Filter:               filters.Empty(),
		OCIConcurrency:       o.ociConcurrency,
		RemoteOptions:        defaultRemoteOptions(),
		CachePath:            cachePath,
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
	verify                  bool
	skipSignatureValidation bool
	ociConcurrency          int
	publicKeyPath           string
}

func newPackageInspectImagesOptions() *packageInspectImagesOptions {
	return &packageInspectImagesOptions{
		verify: false,
	}
}

func newPackageInspectImagesCommand(v *viper.Viper) *cobra.Command {
	o := newPackageInspectImagesOptions()
	cmd := &cobra.Command{
		Use:    "images [ PACKAGE_SOURCE ]",
		Short:  "List all container images contained in the package",
		Args:   cobra.MaximumNArgs(1),
		PreRun: o.preRun,
		RunE:   o.run,
	}

	cmd.Flags().IntVar(&o.ociConcurrency, "oci-concurrency", v.GetInt(VPkgOCIConcurrency), lang.CmdPackageFlagConcurrency)
	cmd.Flags().StringVarP(&o.publicKeyPath, "key", "k", v.GetString(VPkgPublicKey), lang.CmdPackageFlagFlagPublicKey)
	cmd.Flags().StringVarP(&o.namespaceOverride, "namespace", "n", o.namespaceOverride, lang.CmdPackageInspectFlagNamespace)
	cmd.Flags().BoolVar(&o.skipSignatureValidation, "skip-signature-validation", o.skipSignatureValidation, lang.CmdPackageFlagSkipSignatureValidation)
	cmd.Flags().BoolVar(&o.verify, "verify", v.GetBool(VPkgVerify), lang.CmdPackageFlagVerify)
	errSig := cmd.Flags().MarkDeprecated("skip-signature-validation", "Signature verification now occurs on every execution, but is not enforced by default. Use --verify to enforce validation. This flag will be removed in Zarf v1.0.0.")
	if errSig != nil {
		logger.Default().Debug("unable to mark skip-signature-validation", "error", errSig)
	}
	return cmd
}

func (o *packageInspectImagesOptions) preRun(cmd *cobra.Command, _ []string) {
	// Handle deprecated --skip-signature-validation flag for backwards compatibility
	if cmd.Flags().Changed("skip-signature-validation") {
		logger.Default().Warn("--skip-signature-validation is deprecated and will be removed in v1.0.0. Use --verify to enforce signature validation.")

		if cmd.Flags().Changed("verify") {
			return
		}

		o.verify = !o.skipSignatureValidation
	}
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
		VerificationStrategy: getVerificationStrategy(o.verify),
		Architecture:         config.GetArch(),
		Filter:               filters.Empty(),
		PublicKeyPath:        o.publicKeyPath,
		OCIConcurrency:       o.ociConcurrency,
		RemoteOptions:        defaultRemoteOptions(),
		CachePath:            cachePath,
	}
	pkg, err := packager.GetPackageFromSourceOrCluster(ctx, cluster, src, o.namespaceOverride, loadOpts)
	if err != nil {
		return fmt.Errorf("unable to load the package: %w", err)
	}

	images := make([]string, 0)
	for _, component := range pkg.Components {
		images = append(images, component.GetImages()...)
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

type packageInspectDocumentationOptions struct {
	skipSignatureValidation bool
	keys                    []string
	outputDir               string
	ociConcurrency          int
	publicKeyPath           string
	verify                  bool
}

func newPackageInspectDocumentationOptions() *packageInspectDocumentationOptions {
	return &packageInspectDocumentationOptions{}
}

func newPackageInspectDocumentationCommand(v *viper.Viper) *cobra.Command {
	o := newPackageInspectDocumentationOptions()
	cmd := &cobra.Command{
		Use:    "documentation [ PACKAGE_SOURCE ]",
		Short:  "Extract documentation files from the package",
		Args:   cobra.MaximumNArgs(1),
		PreRun: o.preRun,
		RunE:   o.run,
	}

	cmd.Flags().IntVar(&o.ociConcurrency, "oci-concurrency", v.GetInt(VPkgOCIConcurrency), lang.CmdPackageFlagConcurrency)
	cmd.Flags().StringVarP(&o.publicKeyPath, "key", "k", v.GetString(VPkgPublicKey), lang.CmdPackageFlagFlagPublicKey)
	cmd.Flags().BoolVar(&o.skipSignatureValidation, "skip-signature-validation", o.skipSignatureValidation, lang.CmdPackageFlagSkipSignatureValidation)
	cmd.Flags().StringSliceVar(&o.keys, "keys", []string{}, "Comma-separated list of documentation keys to extract (e.g., 'configuration,changelog')")
	cmd.Flags().StringVar(&o.outputDir, "output", o.outputDir, "Directory to extract documentation to (created under '<package-name>-documentation' subdirectory)")
	cmd.Flags().BoolVar(&o.verify, "verify", v.GetBool(VPkgVerify), lang.CmdPackageFlagVerify)
	errSig := cmd.Flags().MarkDeprecated("skip-signature-validation", "Signature verification now occurs on every execution, but is not enforced by default. Use --verify to enforce validation. This flag will be removed in Zarf v1.0.0.")
	if errSig != nil {
		logger.Default().Debug("unable to mark skip-signature-validation", "error", errSig)
	}
	return cmd
}

func (o *packageInspectDocumentationOptions) preRun(cmd *cobra.Command, _ []string) {
	// Handle deprecated --skip-signature-validation flag for backwards compatibility
	if cmd.Flags().Changed("skip-signature-validation") {
		logger.Default().Warn("--skip-signature-validation is deprecated and will be removed in v1.0.0. Use --verify to enforce signature validation.")

		if cmd.Flags().Changed("verify") {
			return
		}

		o.verify = !o.skipSignatureValidation
	}
}

func (o *packageInspectDocumentationOptions) run(cmd *cobra.Command, args []string) (err error) {
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
		VerificationStrategy: getVerificationStrategy(o.verify),
		Architecture:         config.GetArch(),
		Filter:               filters.Empty(),
		PublicKeyPath:        o.publicKeyPath,
		OCIConcurrency:       o.ociConcurrency,
		RemoteOptions:        defaultRemoteOptions(),
		CachePath:            cachePath,
		LayersSelector:       zoci.DocLayers,
	}
	pkgLayout, err := packager.LoadPackage(ctx, src, loadOpts)
	if err != nil {
		return fmt.Errorf("unable to load the package: %w", err)
	}
	defer func() {
		err = errors.Join(err, pkgLayout.Cleanup())
	}()

	outputPath := filepath.Join(o.outputDir, fmt.Sprintf("%s-documentation", pkgLayout.Pkg.Metadata.Name))
	return pkgLayout.GetDocumentation(ctx, outputPath, o.keys)
}

type packageInspectDefinitionOptions struct {
	namespaceOverride       string
	verify                  bool
	skipSignatureValidation bool
	ociConcurrency          int
	publicKeyPath           string
}

func newPackageInspectDefinitionOptions() *packageInspectDefinitionOptions {
	return &packageInspectDefinitionOptions{
		verify: false,
	}
}

func newPackageInspectDefinitionCommand(v *viper.Viper) *cobra.Command {
	o := newPackageInspectDefinitionOptions()
	cmd := &cobra.Command{
		Use:    "definition [ PACKAGE_SOURCE ]",
		Short:  "Displays the 'zarf.yaml' definition for the specified package",
		Args:   cobra.MaximumNArgs(1),
		PreRun: o.preRun,
		RunE:   o.run,
	}

	cmd.Flags().IntVar(&o.ociConcurrency, "oci-concurrency", v.GetInt(VPkgOCIConcurrency), lang.CmdPackageFlagConcurrency)
	cmd.Flags().StringVarP(&o.publicKeyPath, "key", "k", v.GetString(VPkgPublicKey), lang.CmdPackageFlagFlagPublicKey)
	cmd.Flags().StringVarP(&o.namespaceOverride, "namespace", "n", o.namespaceOverride, lang.CmdPackageInspectFlagNamespace)
	cmd.Flags().BoolVar(&o.skipSignatureValidation, "skip-signature-validation", o.skipSignatureValidation, lang.CmdPackageFlagSkipSignatureValidation)
	cmd.Flags().BoolVar(&o.verify, "verify", v.GetBool(VPkgVerify), lang.CmdPackageFlagVerify)
	errSig := cmd.Flags().MarkDeprecated("skip-signature-validation", "Signature verification now occurs on every execution, but is not enforced by default. Use --verify to enforce validation. This flag will be removed in Zarf v1.0.0.")
	if errSig != nil {
		logger.Default().Debug("unable to mark skip-signature-validation", "error", errSig)
	}
	return cmd
}

func (o *packageInspectDefinitionOptions) preRun(cmd *cobra.Command, _ []string) {
	// Handle deprecated --skip-signature-validation flag for backwards compatibility
	if cmd.Flags().Changed("skip-signature-validation") {
		logger.Default().Warn("--skip-signature-validation is deprecated and will be removed in v1.0.0. Use --verify to enforce signature validation.")

		if cmd.Flags().Changed("verify") {
			return
		}

		o.verify = !o.skipSignatureValidation
	}
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
		VerificationStrategy: getVerificationStrategy(o.verify),
		Architecture:         config.GetArch(),
		Filter:               filters.Empty(),
		PublicKeyPath:        o.publicKeyPath,
		OCIConcurrency:       o.ociConcurrency,
		RemoteOptions:        defaultRemoteOptions(),
		CachePath:            cachePath,
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
	namespaceOverride       string
	confirm                 bool
	optionalComponents      string
	verify                  bool
	skipSignatureValidation bool
	skipVersionCheck        bool
	ociConcurrency          int
	publicKeyPath           string
	valuesFiles             []string
	setValues               map[string]string
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

	cmd.Flags().IntVar(&o.ociConcurrency, "oci-concurrency", v.GetInt(VPkgOCIConcurrency), lang.CmdPackageFlagConcurrency)
	cmd.Flags().StringVarP(&o.publicKeyPath, "key", "k", v.GetString(VPkgPublicKey), lang.CmdPackageFlagFlagPublicKey)
	cmd.Flags().BoolVarP(&o.confirm, "confirm", "c", false, lang.CmdPackageRemoveFlagConfirm)
	cmd.Flags().StringVar(&o.optionalComponents, "components", v.GetString(VPkgDeployComponents), lang.CmdPackageRemoveFlagComponents)
	cmd.Flags().StringVarP(&o.namespaceOverride, "namespace", "n", v.GetString(VPkgDeployNamespace), lang.CmdPackageRemoveFlagNamespace)
	cmd.Flags().BoolVar(&o.skipSignatureValidation, "skip-signature-validation", false, lang.CmdPackageFlagSkipSignatureValidation)
	cmd.Flags().BoolVar(&o.verify, "verify", v.GetBool(VPkgVerify), lang.CmdPackageFlagVerify)
	cmd.Flags().BoolVar(&o.skipVersionCheck, "skip-version-check", false, "Ignore version requirements when removing the package")
	_ = cmd.Flags().MarkHidden("skip-version-check")
	cmd.Flags().StringSliceVarP(&o.valuesFiles, "values", "v", []string{}, lang.CmdPackageRemoveFlagValuesFiles)
	cmd.Flags().StringToStringVar(&o.setValues, "set-values", v.GetStringMapString(VPkgRemoveSetValues), lang.CmdPackageDeployFlagSetValues)
	errSig := cmd.Flags().MarkDeprecated("skip-signature-validation", "Signature verification now occurs on every execution, but is not enforced by default. Use --verify to enforce validation. This flag will be removed in Zarf v1.0.0.")
	if errSig != nil {
		logger.Default().Debug("unable to mark skip-signature-validation", "error", errSig)
	}
	return cmd
}

func (o *packageRemoveOptions) preRun(cmd *cobra.Command, _ []string) {
	// Handle deprecated --skip-signature-validation flag for backwards compatibility
	if cmd.Flags().Changed("skip-signature-validation") {
		logger.Default().Warn("--skip-signature-validation is deprecated and will be removed in v1.0.0. Use --verify to enforce signature validation.")

		if cmd.Flags().Changed("verify") {
			return
		}

		o.verify = !o.skipSignatureValidation
	}
}

func (o *packageRemoveOptions) run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	packageSource, err := choosePackage(ctx, args)
	if err != nil {
		return err
	}

	// Parse values from files
	vals, err := value.ParseFiles(ctx, o.valuesFiles, value.ParseFilesOptions{})
	if err != nil {
		return fmt.Errorf("unable to parse values files: %w", err)
	}

	// Apply CLI --set-values overrides
	for key, val := range o.setValues {
		// Convert key to path format (ensure it starts with .)
		path := value.Path(key)
		if !strings.HasPrefix(key, ".") {
			path = value.Path("." + key)
		}
		if err := vals.Set(path, val); err != nil {
			return fmt.Errorf("unable to set value at path %s: %w", key, err)
		}
	}

	filter := filters.Combine(
		filters.ByLocalOS(runtime.GOOS),
		filters.BySelectState(o.optionalComponents),
	)
	cachePath, err := getCachePath(ctx)
	if err != nil {
		return err
	}

	c, _ := cluster.New(ctx) //nolint:errcheck
	loadOpts := packager.LoadOptions{
		VerificationStrategy: getVerificationStrategy(o.verify),
		Architecture:         config.GetArch(),
		Filter:               filter,
		PublicKeyPath:        o.publicKeyPath,
		OCIConcurrency:       o.ociConcurrency,
		RemoteOptions:        defaultRemoteOptions(),
		CachePath:            cachePath,
	}
	pkg, err := packager.GetPackageFromSourceOrCluster(ctx, c, packageSource, o.namespaceOverride, loadOpts)
	if err != nil {
		return fmt.Errorf("unable to load the package: %w", err)
	}
	removeOpt := packager.RemoveOptions{
		Cluster:           c,
		Timeout:           config.ZarfDefaultTimeout,
		NamespaceOverride: o.namespaceOverride,
		SkipVersionCheck:  o.skipVersionCheck,
		Values:            vals,
	}
	logger.From(ctx).Info("loaded package for removal", "name", pkg.Metadata.Name)
	err = utils.ColorPrintYAML(pkg, nil, false)
	if err != nil {
		return fmt.Errorf("unable to print package definition: %w", err)
	}
	if !o.confirm {
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
	flavor                  string
	retries                 int
	signingKeyPath          string
	signingKeyPassword      string
	verify                  bool
	skipSignatureValidation bool
	confirm                 bool
	ociConcurrency          int
	publicKeyPath           string
	skipVersionCheck        bool
	withBuildMachineInfo    bool
}

func newPackagePublishCommand(v *viper.Viper) *cobra.Command {
	o := &packagePublishOptions{}

	cmd := &cobra.Command{
		Use:     "publish { PACKAGE_SOURCE | SKELETON DIRECTORY } REPOSITORY",
		Aliases: []string{"push"},
		Short:   lang.CmdPackagePublishShort,
		Example: lang.CmdPackagePublishExample,
		Args:    cobra.ExactArgs(2),
		PreRun:  o.preRun,
		RunE:    o.run,
	}

	cmd.Flags().IntVar(&o.ociConcurrency, "oci-concurrency", v.GetInt(VPkgOCIConcurrency), lang.CmdPackageFlagConcurrency)
	cmd.Flags().StringVarP(&o.publicKeyPath, "key", "k", v.GetString(VPkgPublicKey), lang.CmdPackageFlagFlagPublicKey)
	cmd.Flags().StringVar(&o.signingKeyPath, "signing-key", v.GetString(VPkgPublishSigningKey), lang.CmdPackagePublishFlagSigningKey)
	cmd.Flags().StringVar(&o.signingKeyPassword, "signing-key-pass", v.GetString(VPkgPublishSigningKeyPassword), lang.CmdPackagePublishFlagSigningKeyPassword)
	cmd.Flags().BoolVar(&o.skipSignatureValidation, "skip-signature-validation", false, lang.CmdPackageFlagSkipSignatureValidation)
	cmd.Flags().BoolVar(&o.verify, "verify", v.GetBool(VPkgVerify), lang.CmdPackageFlagVerify)
	cmd.Flags().StringVarP(&o.flavor, "flavor", "f", v.GetString(VPkgCreateFlavor), lang.CmdPackagePublishFlagFlavor)
	cmd.Flags().IntVar(&o.retries, "retries", v.GetInt(VPkgPublishRetries), lang.CmdPackageFlagRetries)
	cmd.Flags().BoolVarP(&o.confirm, "confirm", "c", false, lang.CmdPackagePublishFlagConfirm)
	cmd.Flags().BoolVar(&o.skipVersionCheck, "skip-version-check", false, "Ignore version requirements when publishing the package")
	_ = cmd.Flags().MarkHidden("skip-version-check")
	cmd.Flags().BoolVar(&o.withBuildMachineInfo, "with-build-machine-info", v.GetBool(VPkgPublishWithBuildMachineInfo), lang.CmdPackageCreateFlagWithBuildMachineInfo)
	errSig := cmd.Flags().MarkDeprecated("skip-signature-validation", "Signature verification now occurs on every execution, but is not enforced by default. Use --verify to enforce validation. This flag will be removed in Zarf v1.0.0.")
	if errSig != nil {
		logger.Default().Debug("unable to mark skip-signature-validation", "error", errSig)
	}
	return cmd
}

func (o *packagePublishOptions) preRun(cmd *cobra.Command, _ []string) {
	// Handle deprecated --skip-signature-validation flag for backwards compatibility
	if cmd.Flags().Changed("skip-signature-validation") {
		logger.Default().Warn("--skip-signature-validation is deprecated and will be removed in v1.0.0. Use --verify to enforce signature validation.")

		if cmd.Flags().Changed("verify") {
			return
		}

		o.verify = !o.skipSignatureValidation
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
			OCIConcurrency:       o.ociConcurrency,
			SigningKeyPath:       o.signingKeyPath,
			SigningKeyPassword:   o.signingKeyPassword,
			Retries:              o.retries,
			RemoteOptions:        defaultRemoteOptions(),
			CachePath:            cachePath,
			Flavor:               o.flavor,
			SkipVersionCheck:     o.skipVersionCheck,
			WithBuildMachineInfo: o.withBuildMachineInfo,
		}
		_, err = packager.PublishSkeleton(ctx, packageSource, dstRef, skeletonOpts)
		return err
	}

	if helpers.IsOCIURL(packageSource) && o.signingKeyPath == "" {
		ociOpts := packager.PublishFromOCIOptions{
			OCIConcurrency: o.ociConcurrency,
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

	// Establish default stance
	verificationStrategy := getVerificationStrategy(o.verify)

	if helpers.IsOCIURL(packageSource) && o.signingKeyPath != "" {
		l.Info("pulling source package locally to sign", "reference", packageSource)
		tmpdir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
		if err != nil {
			return err
		}
		defer func() {
			err = errors.Join(err, os.RemoveAll(tmpdir))
		}()

		packagePath, err := packager.Pull(ctx, packageSource, tmpdir, packager.PullOptions{
			VerificationStrategy: verificationStrategy,
			PublicKeyPath:        o.publicKeyPath,
			Architecture:         config.GetArch(),
			OCIConcurrency:       o.ociConcurrency,
			RemoteOptions:        defaultRemoteOptions(),
			CachePath:            cachePath,
		})
		if err != nil {
			return fmt.Errorf("failed to pull package: %w", err)
		}
		packageSource = packagePath
	}

	loadOpt := packager.LoadOptions{
		PublicKeyPath:        o.publicKeyPath,
		VerificationStrategy: verificationStrategy,
		Filter:               filters.Empty(),
		Architecture:         config.GetArch(),
		OCIConcurrency:       o.ociConcurrency,
		RemoteOptions:        defaultRemoteOptions(),
		CachePath:            cachePath,
	}
	pkgLayout, err := packager.LoadPackage(ctx, packageSource, loadOpt)
	if err != nil {
		return fmt.Errorf("unable to load package: %w", err)
	}
	defer func() {
		err = errors.Join(err, pkgLayout.Cleanup())
	}()

	publishPackageOpts := packager.PublishPackageOptions{
		OCIConcurrency:     o.ociConcurrency,
		SigningKeyPath:     o.signingKeyPath,
		SigningKeyPassword: o.signingKeyPassword,
		Retries:            o.retries,
		RemoteOptions:      defaultRemoteOptions(),
	}

	_, err = packager.PublishPackage(ctx, pkgLayout, dstRef, publishPackageOpts)
	return err
}

type packagePullOptions struct {
	shasum                  string
	outputDirectory         string
	skipSignatureValidation bool
	verify                  bool
	ociConcurrency          int
	publicKeyPath           string
}

func newPackagePullCommand(v *viper.Viper) *cobra.Command {
	o := &packagePullOptions{}

	cmd := &cobra.Command{
		Use:     "pull PACKAGE_SOURCE",
		Short:   lang.CmdPackagePullShort,
		Example: lang.CmdPackagePullExample,
		Args:    cobra.ExactArgs(1),
		PreRun:  o.preRun,
		RunE:    o.run,
	}

	cmd.Flags().IntVar(&o.ociConcurrency, "oci-concurrency", v.GetInt(VPkgOCIConcurrency), lang.CmdPackageFlagConcurrency)
	cmd.Flags().StringVarP(&o.publicKeyPath, "key", "k", v.GetString(VPkgPublicKey), lang.CmdPackageFlagFlagPublicKey)
	cmd.Flags().StringVar(&o.shasum, "shasum", "", lang.CmdPackagePullFlagShasum)
	cmd.Flags().StringVarP(&o.outputDirectory, "output-directory", "o", v.GetString(VPkgPullOutputDir), lang.CmdPackagePullFlagOutputDirectory)
	cmd.Flags().BoolVar(&o.skipSignatureValidation, "skip-signature-validation", false, lang.CmdPackageFlagSkipSignatureValidation)
	cmd.Flags().BoolVar(&o.verify, "verify", v.GetBool(VPkgVerify), lang.CmdPackageFlagVerify)
	errSig := cmd.Flags().MarkDeprecated("skip-signature-validation", "Signature verification now occurs on every execution, but is not enforced by default. Use --verify to enforce validation. This flag will be removed in Zarf v1.0.0.")
	if errSig != nil {
		logger.Default().Debug("unable to mark skip-signature-validation", "error", errSig)
	}

	return cmd
}

func (o *packagePullOptions) preRun(cmd *cobra.Command, _ []string) {
	// Handle deprecated --skip-signature-validation flag for backwards compatibility
	if cmd.Flags().Changed("skip-signature-validation") {
		logger.Default().Warn("--skip-signature-validation is deprecated and will be removed in v1.0.0. Use --verify to enforce signature validation.")

		if cmd.Flags().Changed("verify") {
			return
		}

		o.verify = !o.skipSignatureValidation
	}
}

func (o *packagePullOptions) run(cmd *cobra.Command, args []string) error {
	srcURL := args[0]
	outputDir := o.outputDirectory
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
		SHASum:               o.shasum,
		VerificationStrategy: getVerificationStrategy(o.verify),
		PublicKeyPath:        o.publicKeyPath,
		Architecture:         config.GetArch(),
		OCIConcurrency:       o.ociConcurrency,
		RemoteOptions:        defaultRemoteOptions(),
		CachePath:            cachePath,
	})
	if err != nil {
		return err
	}
	logger.From(cmd.Context()).Info("package downloaded successful", "path", packagePath)
	return nil
}

type packageSignOptions struct {
	signingKeyPath     string
	signingKeyPassword string
	publicKeyPath      string
	overwrite          bool
	output             string
	ociConcurrency     int
	retries            int
	verify             bool
}

func newPackageSignCommand(v *viper.Viper) *cobra.Command {
	o := &packageSignOptions{}

	cmd := &cobra.Command{
		Use:     "sign PACKAGE_SOURCE",
		Aliases: []string{"s"},
		Args:    cobra.ExactArgs(1),
		Short:   lang.CmdPackageSignShort,
		Long:    lang.CmdPackageSignLong,
		Example: lang.CmdPackageSignExample,
		RunE:    o.run,
	}

	cmd.Flags().StringVar(&o.signingKeyPath, "signing-key", v.GetString(VPkgSignSigningKey), lang.CmdPackageSignFlagSigningKey)
	cmd.Flags().StringVar(&o.signingKeyPassword, "signing-key-pass", v.GetString(VPkgSignSigningKeyPassword), lang.CmdPackageSignFlagSigningKeyPass)
	cmd.Flags().StringVarP(&o.output, "output", "o", v.GetString(VPkgSignOutput), lang.CmdPackageSignFlagOutput)
	cmd.Flags().BoolVar(&o.overwrite, "overwrite", v.GetBool(VPkgSignOverwrite), lang.CmdPackageSignFlagOverwrite)
	cmd.Flags().StringVarP(&o.publicKeyPath, "key", "k", v.GetString(VPkgPublicKey), lang.CmdPackageSignFlagKey)
	cmd.Flags().IntVar(&o.ociConcurrency, "oci-concurrency", v.GetInt(VPkgOCIConcurrency), lang.CmdPackageFlagConcurrency)
	cmd.Flags().IntVar(&o.retries, "retries", v.GetInt(VPkgRetries), lang.CmdPackageFlagRetries)
	cmd.Flags().BoolVar(&o.verify, "verify", v.GetBool(VPkgVerify), lang.CmdPackageFlagVerify)

	return cmd
}

func (o *packageSignOptions) run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	l := logger.From(ctx)
	packageSource := args[0]

	if o.signingKeyPath == "" {
		return errors.New("--signing-key is required")
	}

	// Determine output destination
	outputDest := o.output
	if outputDest == "" {
		if helpers.IsOCIURL(packageSource) {
			// For OCI sources, default to publishing back to the same OCI location
			// Extract the repository portion (without package name and tag) from source
			trimmed := strings.TrimPrefix(packageSource, helpers.OCIURLPrefix)
			srcRef, err := registry.ParseReference(trimmed)
			if err != nil {
				return fmt.Errorf("failed to parse source OCI reference: %w", err)
			}

			// Extract repository path without the package name
			// e.g., "registry.com/namespace/package:tag" -> "registry.com/namespace"
			repoParts := strings.Split(srcRef.Repository, "/")
			if len(repoParts) > 1 {
				// Remove the last part (package name)
				repoPath := strings.Join(repoParts[:len(repoParts)-1], "/")
				outputDest = helpers.OCIURLPrefix + srcRef.Registry + "/" + repoPath
			} else {
				// Package is directly under registry (no namespace)
				outputDest = helpers.OCIURLPrefix + srcRef.Registry
			}
		} else {
			// For file sources, use the same directory as the source
			outputDest = filepath.Dir(packageSource)
		}
	}

	// If output is OCI (either default or user-specified), delegate to publish workflow
	if helpers.IsOCIURL(outputDest) {
		l.Info("signing and publishing package to OCI registry", "source", packageSource, "destination", outputDest)

		// Create publish options from sign options
		publishOpts := &packagePublishOptions{
			signingKeyPath:     o.signingKeyPath,
			signingKeyPassword: o.signingKeyPassword,
			ociConcurrency:     o.ociConcurrency,
			retries:            o.retries,
			publicKeyPath:      o.publicKeyPath,
			verify:             o.verify,
		}

		// Call publish with source and destination repository
		return publishOpts.run(cmd, []string{packageSource, outputDest})
	}

	// For local file output, use existing sign logic
	cachePath, err := getCachePath(ctx)
	if err != nil {
		return err
	}

	// Load the package
	loadOpts := packager.LoadOptions{
		PublicKeyPath:        o.publicKeyPath,
		Filter:               filters.Empty(),
		Architecture:         config.GetArch(),
		OCIConcurrency:       o.ociConcurrency,
		RemoteOptions:        defaultRemoteOptions(),
		CachePath:            cachePath,
		VerificationStrategy: getVerificationStrategy(o.verify),
	}

	l.Info("loading package", "source", packageSource)
	pkgLayout, err := packager.LoadPackage(ctx, packageSource, loadOpts)
	if err != nil {
		return fmt.Errorf("unable to load package: %w", err)
	}
	defer func() {
		if cleanupErr := pkgLayout.Cleanup(); cleanupErr != nil {
			l.Warn("failed to cleanup package layout", "error", cleanupErr)
		}
	}()

	signed := pkgLayout.IsSigned()

	if signed && !o.overwrite {
		return errors.New("package is already signed, use --overwrite to re-sign")
	}

	// Sign the package
	l.Info("signing package with provided key")

	signOpts := utils.DefaultSignBlobOptions()
	signOpts.KeyRef = o.signingKeyPath
	signOpts.Password = o.signingKeyPassword

	err = pkgLayout.SignPackage(ctx, signOpts)
	if err != nil {
		return fmt.Errorf("failed to sign package: %w", err)
	}

	// Archive to local directory
	l.Info("archiving signed package to local directory", "directory", outputDest)
	signedPath, err := pkgLayout.Archive(ctx, outputDest, 0)
	if err != nil {
		return fmt.Errorf("failed to archive signed package: %w", err)
	}

	l.Info("package signed successfully", "path", signedPath)
	return nil
}

type packageVerifyOptions struct {
	publicKeyPath  string
	ociConcurrency int
}

func newPackageVerifyCommand(v *viper.Viper) *cobra.Command {
	o := &packageVerifyOptions{}

	cmd := &cobra.Command{
		Use:     "verify PACKAGE_SOURCE",
		Aliases: []string{"v"},
		Args:    cobra.ExactArgs(1),
		Short:   lang.CmdPackageVerifyShort,
		Long:    lang.CmdPackageVerifyLong,
		Example: lang.CmdPackageVerifyExample,
		RunE:    o.run,
	}

	cmd.Flags().StringVarP(&o.publicKeyPath, "key", "k", v.GetString(VPkgPublicKey), lang.CmdPackageVerifyFlagKey)
	cmd.Flags().IntVar(&o.ociConcurrency, "oci-concurrency", v.GetInt(VPkgOCIConcurrency), lang.CmdPackageFlagConcurrency)

	return cmd
}

func (o *packageVerifyOptions) run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	l := logger.From(ctx)
	packageSource := args[0]

	l.Info("verifying package", "source", packageSource)

	cachePath, err := getCachePath(ctx)
	if err != nil {
		return err
	}

	// Load the package with verification enabled
	// The verify command always uses strict verification (VerifyAlways)
	// This will error if: signed package without key, or unsigned package with key
	loadOpts := packager.LoadOptions{
		PublicKeyPath:        o.publicKeyPath,
		VerificationStrategy: layout.VerifyAlways, // Always enforce strict verification
		Filter:               filters.Empty(),
		Architecture:         config.GetArch(),
		OCIConcurrency:       o.ociConcurrency,
		RemoteOptions:        defaultRemoteOptions(),
		CachePath:            cachePath,
		LayersSelector:       zoci.MetadataLayers,
	}

	pkgLayout, err := packager.LoadPackage(ctx, packageSource, loadOpts)
	if err != nil {
		return fmt.Errorf("package verification failed: %w", err)
	}
	defer func() {
		if cleanupErr := pkgLayout.Cleanup(); cleanupErr != nil {
			l.Warn("failed to cleanup package", "error", cleanupErr)
		}
	}()

	// If we got here, all verification passed
	l.Info("checksum verification", "status", "PASSED")

	// Log signature verification status
	if pkgLayout.IsSigned() {
		// If signed and we got here, signature verification passed
		l.Info("signature verification", "status", "PASSED")
	}

	if !pkgLayout.IsSigned() {
		// Package is unsigned (allowed when no key provided)
		l.Warn("package is unsigned", "signed", false)
	}

	l.Info("verification complete", "status", "SUCCESS")
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

func getVerificationStrategy(verify bool) layout.VerificationStrategy {
	if verify {
		return layout.VerifyAlways
	}
	return layout.VerifyIfPossible
}
