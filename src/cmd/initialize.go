// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
	kerrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/spf13/cobra"
)

type initOptions struct {
	setVariables            map[string]string
	optionalComponents      string
	storageClass            string
	gitServer               state.GitServerInfo
	registryInfo            state.RegistryInfo
	artifactServer          state.ArtifactServerInfo
	injectorPort            int
	adoptExistingResources  bool
	timeout                 time.Duration
	retries                 int
	publicKeyPath           string
	verify                  bool
	skipSignatureValidation bool
	confirm                 bool
	ociConcurrency          int
}

func newInitCommand() *cobra.Command {
	o := &initOptions{}

	cmd := &cobra.Command{
		Use:     "init [ PACKAGE_SOURCE ]",
		Aliases: []string{"i"},
		Short:   lang.CmdInitShort,
		Long:    lang.CmdInitLong,
		Example: lang.CmdInitExample,
		Args:    cobra.MaximumNArgs(1),
		PreRun:  o.preRun,
		RunE:    o.run,
	}

	v := getViper()

	// Init package set variable flags
	cmd.Flags().StringToStringVar(&o.setVariables, "set", v.GetStringMapString(VPkgDeploySet), "Alias for --set-variables")
	_ = cmd.Flags().MarkDeprecated("set", "Use --set-variables instead")
	cmd.Flags().StringToStringVar(&o.setVariables, "set-variables", v.GetStringMapString(VPkgDeploySet), lang.CmdInitFlagSetVariables)

	// Continue to require --confirm flag for init command to avoid accidental deployments
	cmd.Flags().BoolVarP(&o.confirm, "confirm", "c", false, lang.CmdInitFlagConfirm)
	cmd.Flags().StringVar(&o.optionalComponents, "components", v.GetString(VInitComponents), lang.CmdInitFlagComponents)
	cmd.Flags().StringVar(&o.storageClass, "storage-class", v.GetString(VInitStorageClass), lang.CmdInitFlagStorageClass)

	cmd.Flags().StringVar((*string)(&o.registryInfo.RegistryMode), "registry-mode", "",
		fmt.Sprintf("how to access the registry (valid values: %s, %s, %s). Proxy mode is an alpha feature", state.RegistryModeNodePort, state.RegistryModeProxy, state.RegistryModeExternal))
	cmd.Flags().IntVar(&o.injectorPort, "injector-port", v.GetInt(InjectorPort),
		"the port that the injector will be exposed through. Affects the service nodeport in nodeport mode and pod hostport in proxy mode")
	// While this feature is in early alpha we will hide the flags
	cmd.Flags().MarkHidden("registry-mode")

	// Flags for using an external Git server
	cmd.Flags().StringVar(&o.gitServer.Address, "git-url", v.GetString(VInitGitURL), lang.CmdInitFlagGitURL)
	cmd.Flags().StringVar(&o.gitServer.PushUsername, "git-push-username", v.GetString(VInitGitPushUser), lang.CmdInitFlagGitPushUser)
	cmd.Flags().StringVar(&o.gitServer.PushPassword, "git-push-password", v.GetString(VInitGitPushPass), lang.CmdInitFlagGitPushPass)
	cmd.Flags().StringVar(&o.gitServer.PullUsername, "git-pull-username", v.GetString(VInitGitPullUser), lang.CmdInitFlagGitPullUser)
	cmd.Flags().StringVar(&o.gitServer.PullPassword, "git-pull-password", v.GetString(VInitGitPullPass), lang.CmdInitFlagGitPullPass)

	// Flags for using an external registry
	cmd.Flags().StringVar(&o.registryInfo.Address, "registry-url", v.GetString(VInitRegistryURL), lang.CmdInitFlagRegURL)
	cmd.Flags().IntVar(&o.registryInfo.NodePort, "nodeport", v.GetInt(VInitRegistryNodeport), lang.CmdInitFlagRegNodePort)
	cmd.Flags().StringVar(&o.registryInfo.PushUsername, "registry-push-username", v.GetString(VInitRegistryPushUser), lang.CmdInitFlagRegPushUser)
	cmd.Flags().StringVar(&o.registryInfo.PushPassword, "registry-push-password", v.GetString(VInitRegistryPushPass), lang.CmdInitFlagRegPushPass)
	cmd.Flags().StringVar(&o.registryInfo.PullUsername, "registry-pull-username", v.GetString(VInitRegistryPullUser), lang.CmdInitFlagRegPullUser)
	cmd.Flags().StringVar(&o.registryInfo.PullPassword, "registry-pull-password", v.GetString(VInitRegistryPullPass), lang.CmdInitFlagRegPullPass)
	cmd.Flags().StringVar(&o.registryInfo.Secret, "registry-secret", v.GetString(VInitRegistrySecret), lang.CmdInitFlagRegSecret)

	// Flags for using an external artifact server
	cmd.Flags().StringVar(&o.artifactServer.Address, "artifact-url", v.GetString(VInitArtifactURL), lang.CmdInitFlagArtifactURL)
	cmd.Flags().StringVar(&o.artifactServer.PushUsername, "artifact-push-username", v.GetString(VInitArtifactPushUser), lang.CmdInitFlagArtifactPushUser)
	cmd.Flags().StringVar(&o.artifactServer.PushToken, "artifact-push-token", v.GetString(VInitArtifactPushToken), lang.CmdInitFlagArtifactPushToken)

	// Flags that control how a deployment proceeds
	// Always require adopt-existing-resources flag (no viper)
	cmd.Flags().BoolVar(&o.adoptExistingResources, "adopt-existing-resources", false, lang.CmdPackageDeployFlagAdoptExistingResources)
	cmd.Flags().DurationVar(&o.timeout, "timeout", v.GetDuration(VPkgDeployTimeout), lang.CmdPackageDeployFlagTimeout)

	cmd.Flags().IntVar(&o.retries, "retries", v.GetInt(VPkgRetries), lang.CmdPackageFlagRetries)
	cmd.Flags().StringVarP(&o.publicKeyPath, "key", "k", v.GetString(VPkgPublicKey), lang.CmdPackageFlagFlagPublicKey)
	cmd.Flags().BoolVar(&o.verify, "verify", v.GetBool(VPkgVerify), lang.CmdPackageFlagVerify)
	cmd.Flags().IntVar(&o.ociConcurrency, "oci-concurrency", v.GetInt(VPkgOCIConcurrency), lang.CmdPackageFlagConcurrency)
	cmd.Flags().BoolVar(&o.skipSignatureValidation, "skip-signature-validation", false, lang.CmdPackageFlagSkipSignatureValidation)
	errSig := cmd.Flags().MarkDeprecated("skip-signature-validation", "Signature verification now occurs on every execution, but is not enforced by default. Use --verify to enforce validation. This flag will be removed in Zarf v1.0.0.")
	if errSig != nil {
		logger.Default().Debug("unable to mark skip-signature-validation", "error", errSig)
	}

	// If an external registry is used then don't allow users to configure the internal registry / injector
	cmd.MarkFlagsMutuallyExclusive("registry-url", "registry-mode")
	cmd.MarkFlagsMutuallyExclusive("registry-url", "injector-port")
	cmd.MarkFlagsMutuallyExclusive("registry-url", "nodeport")

	cmd.Flags().SortFlags = true

	return cmd
}

func (o *initOptions) preRun(cmd *cobra.Command, _ []string) {
	// Handle deprecated --skip-signature-validation flag for backwards compatibility
	if cmd.Flags().Changed("skip-signature-validation") {
		logger.Default().Warn("--skip-signature-validation is deprecated and will be removed in v1.0.0. Use --verify to enforce signature validation.")

		if cmd.Flags().Changed("verify") {
			return
		}

		o.verify = !o.skipSignatureValidation
	}
}

func (o *initOptions) run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	err := o.validateInitFlags()
	if err != nil {
		return fmt.Errorf("invalid command flags were provided: %w", err)
	}

	err = validateExistingStateMatchesInput(cmd.Context(), o.registryInfo, o.gitServer, o.artifactServer)
	if err != nil {
		return err
	}

	if o.registryInfo.RegistryMode == "" {
		if o.registryInfo.Address == "" {
			o.registryInfo.RegistryMode = state.RegistryModeNodePort
		} else {
			o.registryInfo.RegistryMode = state.RegistryModeExternal
		}
	}

	packageSource := ""

	if len(args) > 0 {
		packageSource = args[0]
	} else {
		initPackageName := config.GetInitPackageName()

		// Try to use an init-package in the executable directory if none exist in current working directory
		packageSource, err = o.findInitPackage(cmd.Context(), initPackageName)
		if err != nil {
			return err
		}
	}

	v := getViper()
	o.setVariables = helpers.TransformAndMergeMap(
		v.GetStringMapString(VPkgDeploySet), o.setVariables, strings.ToUpper)

	cachePath, err := getCachePath(ctx)
	if err != nil {
		return err
	}

	loadOpt := packager.LoadOptions{
		PublicKeyPath:        o.publicKeyPath,
		VerificationStrategy: getVerificationStrategy(o.verify),
		Filter:               filters.Empty(),
		Architecture:         config.GetArch(),
		CachePath:            cachePath,
	}
	pkgLayout, err := packager.LoadPackage(ctx, packageSource, loadOpt)
	if err != nil {
		return fmt.Errorf("unable to load package: %w", err)
	}
	if pkgLayout.Pkg.Kind != v1alpha1.ZarfInitConfig {
		return fmt.Errorf("not a zarf init package")
	}
	defer func() {
		err = errors.Join(err, pkgLayout.Cleanup())
	}()

	opts := packager.DeployOptions{
		GitServer:              o.gitServer,
		RegistryInfo:           o.registryInfo,
		ArtifactServer:         o.artifactServer,
		AdoptExistingResources: o.adoptExistingResources,
		Timeout:                o.timeout,
		Retries:                o.retries,
		OCIConcurrency:         o.ociConcurrency,
		SetVariables:           o.setVariables,
		StorageClass:           o.storageClass,
		InjectorPort:           o.injectorPort,
		RemoteOptions:          defaultRemoteOptions(),
		IsInteractive:          !o.confirm,
	}
	_, err = deploy(ctx, pkgLayout, opts, o.setVariables, o.optionalComponents)
	if err != nil {
		return err
	}

	logger.From(ctx).Info("init complete. To get credentials for Zarf deployed services run `zarf tools get-creds`")
	return nil
}

func (o *initOptions) findInitPackage(ctx context.Context, initPackageName string) (string, error) {
	// First, look for the init package in the current working directory
	if !helpers.InvalidPath(initPackageName) {
		return initPackageName, nil
	}

	// Next, look for the init package in the executable directory
	binaryPath, err := utils.GetFinalExecutablePath()
	if err != nil {
		return "", err
	}
	executableDir := path.Dir(binaryPath)
	if !helpers.InvalidPath(filepath.Join(executableDir, initPackageName)) {
		return filepath.Join(executableDir, initPackageName), nil
	}

	// Create the cache directory if it doesn't exist
	absCachePath, err := getCachePath(ctx)
	if err != nil {
		return "", err
	}
	// Verify that we can write to the path
	if helpers.InvalidPath(absCachePath) {
		// Create the directory if the path is invalid
		err = helpers.CreateDirectory(absCachePath, helpers.ReadExecuteAllWriteUser)
		if err != nil {
			return "", fmt.Errorf("unable to create the cache directory %s: %w", absCachePath, err)
		}
	}

	// Next, look in the cache directory
	if !helpers.InvalidPath(filepath.Join(absCachePath, initPackageName)) {
		// join and return
		return filepath.Join(absCachePath, initPackageName), nil
	}

	if o.confirm {
		return "", lang.ErrInitNotFound
	}

	// Finally, if the init-package doesn't exist in the cache directory, suggest downloading it
	err = o.downloadInitPackage(ctx, absCachePath)
	if err != nil {
		return "", fmt.Errorf("failed to download the init package: %w", err)
	}
	return filepath.Join(absCachePath, initPackageName), nil
}

func (o *initOptions) downloadInitPackage(ctx context.Context, cacheDirectory string) error {
	l := logger.From(ctx)
	url := zoci.GetInitPackageURL(config.CLIVersion)

	// Give the user the choice to download the init-package and note that this does require an internet connection
	l.Info("the init package was not found locally, but can be pulled in connected environments", "url", fmt.Sprintf("oci://%s", url))

	var confirmDownload bool
	prompt := &survey.Confirm{
		Message: lang.CmdInitPullConfirm,
	}
	if err := survey.AskOne(prompt, &confirmDownload); err != nil {
		return fmt.Errorf("confirm download canceled: %w", err)
	}

	// If the user wants to download the init-package, download it
	if confirmDownload {
		// Add the oci:// prefix
		url = fmt.Sprintf("oci://%s", url)

		cachePath, err := getCachePath(ctx)
		if err != nil {
			return err
		}

		pullOptions := packager.PullOptions{
			Architecture:   config.GetArch(),
			OCIConcurrency: o.ociConcurrency,
			CachePath:      cachePath,
		}

		_, err = packager.Pull(ctx, url, cacheDirectory, pullOptions)
		if err != nil {
			return fmt.Errorf("unable to download the init package: %w", err)
		}

		return nil
	}
	// Otherwise, exit and tell the user to manually download the init-package
	return errors.New(lang.CmdInitPullErrManual)
}

// Checks if an init has already happened and if so check that none of the Zarf service information has changed
func validateExistingStateMatchesInput(ctx context.Context, registryInfo state.RegistryInfo, gitServer state.GitServerInfo, artifactServer state.ArtifactServerInfo) error {
	c, err := cluster.New(ctx)
	// If there's no cluster available an init has not happened yet, or this is a custom init
	if err != nil {
		return nil
	}
	s, err := c.LoadState(ctx)
	// If there is no state found this is the first init on this cluster
	if kerrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	if helpers.IsNotZeroAndNotEqual(gitServer, s.GitServer) {
		return fmt.Errorf("cannot change git server information after initial init, to update run `zarf tools update-creds git`")
	}
	if helpers.IsNotZeroAndNotEqual(registryInfo, s.RegistryInfo) {
		return fmt.Errorf("cannot change registry information after initial init, to update run `zarf tools update-creds registry`")
	}
	if helpers.IsNotZeroAndNotEqual(artifactServer, s.ArtifactServer) {
		return fmt.Errorf("cannot change artifact server information after initial init, to update run `zarf tools update-creds artifact`")
	}
	return nil
}

func (o *initOptions) validateInitFlags() error {
	// If 'git-url' is provided, make sure they provided values for the username and password of the push user
	if o.gitServer.Address != "" {
		if o.gitServer.PushUsername == "" || o.gitServer.PushPassword == "" {
			return fmt.Errorf(lang.CmdInitErrValidateGit)
		}
	}

	// If 'registry-url' is provided, make sure they provided values for the username and password of the push user
	if o.registryInfo.Address != "" {
		if o.registryInfo.PushUsername == "" || o.registryInfo.PushPassword == "" {
			return fmt.Errorf(lang.CmdInitErrValidateRegistry)
		}
	}

	// If 'artifact-url' is provided, make sure they provided values for the username and password of the push user
	if o.artifactServer.Address != "" {
		if o.artifactServer.PushUsername == "" || o.artifactServer.PushToken == "" {
			return fmt.Errorf(lang.CmdInitErrValidateArtifact)
		}
	}

	if o.registryInfo.RegistryMode != "" {
		if o.registryInfo.RegistryMode != state.RegistryModeNodePort &&
			o.registryInfo.RegistryMode != state.RegistryModeProxy && o.registryInfo.RegistryMode != state.RegistryModeExternal {
			return fmt.Errorf("invalid registry mode %q, must be %q, %q, or %q", o.registryInfo.RegistryMode,
				state.RegistryModeNodePort, state.RegistryModeProxy, state.RegistryModeExternal)
		}
	}

	return nil
}
