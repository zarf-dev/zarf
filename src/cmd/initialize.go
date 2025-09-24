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

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/pkg/helpers/v2"
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

type initOptions struct{}

func newInitCommand() *cobra.Command {
	o := initOptions{}

	cmd := &cobra.Command{
		Use:     "init",
		Aliases: []string{"i"},
		Short:   lang.CmdInitShort,
		Long:    lang.CmdInitLong,
		Example: lang.CmdInitExample,
		RunE:    o.run,
	}

	v := getViper()

	// Init package variable defaults that are non-zero values
	// NOTE: these are not in setDefaults so that zarf tools update-creds does not erroneously update values back to the default
	v.SetDefault(VInitGitPushUser, state.ZarfGitPushUser)
	v.SetDefault(VInitRegistryPushUser, state.ZarfRegistryPushUser)

	// Init package set variable flags
	cmd.Flags().StringToStringVar(&pkgConfig.PkgOpts.SetVariables, "set", v.GetStringMapString(VPkgDeploySet), lang.CmdInitFlagSet)

	// Continue to require --confirm flag for init command to avoid accidental deployments
	cmd.Flags().BoolVarP(&config.CommonOptions.Confirm, "confirm", "c", false, lang.CmdInitFlagConfirm)
	cmd.Flags().StringVar(&pkgConfig.PkgOpts.OptionalComponents, "components", v.GetString(VInitComponents), lang.CmdInitFlagComponents)
	cmd.Flags().StringVar(&pkgConfig.InitOpts.StorageClass, "storage-class", v.GetString(VInitStorageClass), lang.CmdInitFlagStorageClass)

	cmd.Flags().StringVar((*string)(&pkgConfig.InitOpts.RegistryInfo.RegistryMode), "registry-mode", string(state.RegistryModeNodePort),
		fmt.Sprintf("how to access the registry (valid values: %s, %s). Proxy mode is an alpha feature", state.RegistryModeNodePort, state.RegistryModeProxy))
	cmd.Flags().IntVar(&pkgConfig.InitOpts.InjectorHostPort, "injector-hostport", v.GetInt(InjectorHostPort),
		"the hostport that the long lived DaemonSet injector will use when the registry is running in proxy mode")
	// While this feature is in early alpha we will hide the flags
	cmd.Flags().MarkHidden("registry-mode")
	cmd.Flags().MarkHidden("injector-hostport")

	// Flags for using an external Git server
	cmd.Flags().StringVar(&pkgConfig.InitOpts.GitServer.Address, "git-url", v.GetString(VInitGitURL), lang.CmdInitFlagGitURL)
	cmd.Flags().StringVar(&pkgConfig.InitOpts.GitServer.PushUsername, "git-push-username", v.GetString(VInitGitPushUser), lang.CmdInitFlagGitPushUser)
	cmd.Flags().StringVar(&pkgConfig.InitOpts.GitServer.PushPassword, "git-push-password", v.GetString(VInitGitPushPass), lang.CmdInitFlagGitPushPass)
	cmd.Flags().StringVar(&pkgConfig.InitOpts.GitServer.PullUsername, "git-pull-username", v.GetString(VInitGitPullUser), lang.CmdInitFlagGitPullUser)
	cmd.Flags().StringVar(&pkgConfig.InitOpts.GitServer.PullPassword, "git-pull-password", v.GetString(VInitGitPullPass), lang.CmdInitFlagGitPullPass)

	// Flags for using an external registry
	cmd.Flags().StringVar(&pkgConfig.InitOpts.RegistryInfo.Address, "registry-url", v.GetString(VInitRegistryURL), lang.CmdInitFlagRegURL)
	cmd.Flags().IntVar(&pkgConfig.InitOpts.RegistryInfo.NodePort, "nodeport", v.GetInt(VInitRegistryNodeport), lang.CmdInitFlagRegNodePort)
	cmd.Flags().StringVar(&pkgConfig.InitOpts.RegistryInfo.PushUsername, "registry-push-username", v.GetString(VInitRegistryPushUser), lang.CmdInitFlagRegPushUser)
	cmd.Flags().StringVar(&pkgConfig.InitOpts.RegistryInfo.PushPassword, "registry-push-password", v.GetString(VInitRegistryPushPass), lang.CmdInitFlagRegPushPass)
	cmd.Flags().StringVar(&pkgConfig.InitOpts.RegistryInfo.PullUsername, "registry-pull-username", v.GetString(VInitRegistryPullUser), lang.CmdInitFlagRegPullUser)
	cmd.Flags().StringVar(&pkgConfig.InitOpts.RegistryInfo.PullPassword, "registry-pull-password", v.GetString(VInitRegistryPullPass), lang.CmdInitFlagRegPullPass)
	cmd.Flags().StringVar(&pkgConfig.InitOpts.RegistryInfo.Secret, "registry-secret", v.GetString(VInitRegistrySecret), lang.CmdInitFlagRegSecret)

	// Flags for using an external artifact server
	cmd.Flags().StringVar(&pkgConfig.InitOpts.ArtifactServer.Address, "artifact-url", v.GetString(VInitArtifactURL), lang.CmdInitFlagArtifactURL)
	cmd.Flags().StringVar(&pkgConfig.InitOpts.ArtifactServer.PushUsername, "artifact-push-username", v.GetString(VInitArtifactPushUser), lang.CmdInitFlagArtifactPushUser)
	cmd.Flags().StringVar(&pkgConfig.InitOpts.ArtifactServer.PushToken, "artifact-push-token", v.GetString(VInitArtifactPushToken), lang.CmdInitFlagArtifactPushToken)

	// Flags that control how a deployment proceeds
	// Always require adopt-existing-resources flag (no viper)
	cmd.Flags().BoolVar(&pkgConfig.DeployOpts.AdoptExistingResources, "adopt-existing-resources", false, lang.CmdPackageDeployFlagAdoptExistingResources)
	cmd.Flags().DurationVar(&pkgConfig.DeployOpts.Timeout, "timeout", v.GetDuration(VPkgDeployTimeout), lang.CmdPackageDeployFlagTimeout)

	cmd.Flags().IntVar(&pkgConfig.PkgOpts.Retries, "retries", v.GetInt(VPkgRetries), lang.CmdPackageFlagRetries)
	cmd.Flags().StringVarP(&pkgConfig.PkgOpts.PublicKeyPath, "key", "k", v.GetString(VPkgPublicKey), lang.CmdPackageFlagFlagPublicKey)
	cmd.Flags().BoolVar(&pkgConfig.PkgOpts.SkipSignatureValidation, "skip-signature-validation", false, lang.CmdPackageFlagSkipSignatureValidation)
	cmd.Flags().IntVar(&config.CommonOptions.OCIConcurrency, "oci-concurrency", v.GetInt(VPkgOCIConcurrency), lang.CmdPackageFlagConcurrency)

	// If an external registry is used then don't allow users to configure the internal registry / injector
	cmd.MarkFlagsMutuallyExclusive("registry-url", "registry-mode")
	cmd.MarkFlagsMutuallyExclusive("registry-url", "injector-hostport")
	cmd.MarkFlagsMutuallyExclusive("registry-url", "nodeport")

	cmd.Flags().SortFlags = true

	return cmd
}

func (o *initOptions) run(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	if err := validateInitFlags(); err != nil {
		return fmt.Errorf("invalid command flags were provided: %w", err)
	}

	if err := validateExistingStateMatchesInput(cmd.Context(), pkgConfig.InitOpts.RegistryInfo, pkgConfig.InitOpts.GitServer, pkgConfig.InitOpts.ArtifactServer); err != nil {
		return err
	}

	initPackageName := config.GetInitPackageName()

	// Try to use an init-package in the executable directory if none exist in current working directory
	packageSource, err := findInitPackage(cmd.Context(), initPackageName)
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
		CachePath:               cachePath,
	}
	pkgLayout, err := packager.LoadPackage(ctx, packageSource, loadOpt)
	if err != nil {
		return fmt.Errorf("unable to load package: %w", err)
	}
	defer func() {
		err = errors.Join(err, pkgLayout.Cleanup())
	}()

	opts := packager.DeployOptions{
		GitServer:              pkgConfig.InitOpts.GitServer,
		RegistryInfo:           pkgConfig.InitOpts.RegistryInfo,
		ArtifactServer:         pkgConfig.InitOpts.ArtifactServer,
		AdoptExistingResources: pkgConfig.DeployOpts.AdoptExistingResources,
		Timeout:                pkgConfig.DeployOpts.Timeout,
		Retries:                pkgConfig.PkgOpts.Retries,
		OCIConcurrency:         config.CommonOptions.OCIConcurrency,
		SetVariables:           pkgConfig.PkgOpts.SetVariables,
		StorageClass:           pkgConfig.InitOpts.StorageClass,
		InjectorHostPort:       pkgConfig.InitOpts.InjectorHostPort,
		RemoteOptions:          defaultRemoteOptions(),
	}
	_, err = deploy(ctx, pkgLayout, opts)
	if err != nil {
		return err
	}

	logger.From(ctx).Info("init complete. To get credentials for Zarf deployed services run `zarf tools get-creds`")
	return nil
}

func findInitPackage(ctx context.Context, initPackageName string) (string, error) {
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

	if config.CommonOptions.Confirm {
		return "", lang.ErrInitNotFound
	}

	// Finally, if the init-package doesn't exist in the cache directory, suggest downloading it
	err = downloadInitPackage(ctx, absCachePath)
	if err != nil {
		return "", fmt.Errorf("failed to download the init package: %w", err)
	}
	return filepath.Join(absCachePath, initPackageName), nil
}

func downloadInitPackage(ctx context.Context, cacheDirectory string) error {
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
			OCIConcurrency: config.CommonOptions.OCIConcurrency,
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
		return fmt.Errorf("cannot change artifact server information after initial init, to update run `zarf tools update-creds registry`")
	}
	return nil
}

func validateInitFlags() error {
	// If 'git-url' is provided, make sure they provided values for the username and password of the push user
	if pkgConfig.InitOpts.GitServer.Address != "" {
		if pkgConfig.InitOpts.GitServer.PushUsername == "" || pkgConfig.InitOpts.GitServer.PushPassword == "" {
			return fmt.Errorf(lang.CmdInitErrValidateGit)
		}
	}

	// If 'registry-url' is provided, make sure they provided values for the username and password of the push user
	if pkgConfig.InitOpts.RegistryInfo.Address != "" {
		if pkgConfig.InitOpts.RegistryInfo.PushUsername == "" || pkgConfig.InitOpts.RegistryInfo.PushPassword == "" {
			return fmt.Errorf(lang.CmdInitErrValidateRegistry)
		}
	}

	// If 'artifact-url' is provided, make sure they provided values for the username and password of the push user
	if pkgConfig.InitOpts.ArtifactServer.Address != "" {
		if pkgConfig.InitOpts.ArtifactServer.PushUsername == "" || pkgConfig.InitOpts.ArtifactServer.PushToken == "" {
			return fmt.Errorf(lang.CmdInitErrValidateArtifact)
		}
	}

	if pkgConfig.InitOpts.RegistryInfo.RegistryMode != "" {
		if pkgConfig.InitOpts.RegistryInfo.RegistryMode != state.RegistryModeNodePort &&
			pkgConfig.InitOpts.RegistryInfo.RegistryMode != state.RegistryModeProxy {
			return fmt.Errorf("invalid registry mode %q, must be %q or %q", pkgConfig.InitOpts.RegistryInfo.RegistryMode,
				state.RegistryModeNodePort,
				state.RegistryModeProxy)
		}
	}

	return nil
}
