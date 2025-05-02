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
	"runtime"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/pkg/oci"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/internal/packager2"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/packager/sources"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
	"github.com/zarf-dev/zarf/src/types"

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
	v.SetDefault(VInitGitPushUser, types.ZarfGitPushUser)
	v.SetDefault(VInitRegistryPushUser, types.ZarfRegistryPushUser)

	// Init package set variable flags
	cmd.Flags().StringToStringVar(&pkgConfig.PkgOpts.SetVariables, "set", v.GetStringMapString(VPkgDeploySet), lang.CmdInitFlagSet)

	// Continue to require --confirm flag for init command to avoid accidental deployments
	cmd.Flags().BoolVar(&config.CommonOptions.Confirm, "confirm", false, lang.CmdInitFlagConfirm)
	cmd.Flags().StringVar(&pkgConfig.PkgOpts.OptionalComponents, "components", v.GetString(VInitComponents), lang.CmdInitFlagComponents)
	cmd.Flags().StringVar(&pkgConfig.InitOpts.StorageClass, "storage-class", v.GetString(VInitStorageClass), lang.CmdInitFlagStorageClass)

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

	cmd.Flags().SortFlags = true

	return cmd
}

func (o *initOptions) run(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	if err := validateInitFlags(); err != nil {
		return fmt.Errorf("invalid command flags were provided: %w", err)
	}

	// Continue running package deploy for all components like any other package
	initPackageName := sources.GetInitPackageName()

	// Try to use an init-package in the executable directory if none exist in current working directory
	packageSource, err := findInitPackage(cmd.Context(), initPackageName)
	if err != nil {
		return err
	}

	v := getViper()
	pkgConfig.PkgOpts.SetVariables = helpers.TransformAndMergeMap(
		v.GetStringMapString(VPkgDeploySet), pkgConfig.PkgOpts.SetVariables, strings.ToUpper)

	loadOpt := packager2.LoadOptions{
		Source:                  packageSource,
		Shasum:                  pkgConfig.PkgOpts.Shasum,
		PublicKeyPath:           pkgConfig.PkgOpts.PublicKeyPath,
		SkipSignatureValidation: pkgConfig.PkgOpts.SkipSignatureValidation,
		Filter:                  filters.Empty(),
	}
	pkgLayout, err := packager2.LoadPackage(ctx, loadOpt)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, pkgLayout.Cleanup())
	}()

	deployConfirmed, err := confirmDeploy(ctx, pkgLayout, pkgConfig.PkgOpts.SetVariables)
	if err != nil {
		return err
	}
	if !deployConfirmed {
		return fmt.Errorf("deployment cancelled")
	}

	filter := filters.Combine(
		filters.ByLocalOS(runtime.GOOS),
		filters.ForDeploy(pkgConfig.PkgOpts.OptionalComponents, !config.CommonOptions.Confirm),
	)

	pkgLayout.Pkg.Components, err = filter.Apply(pkgLayout.Pkg)
	if err != nil {
		return err
	}

	_, err = packager2.Deploy(ctx, pkgLayout, packager2.DeployOpts{
		GitServer:              pkgConfig.InitOpts.GitServer,
		RegistryInfo:           pkgConfig.InitOpts.RegistryInfo,
		ArtifactServer:         pkgConfig.InitOpts.ArtifactServer,
		AdoptExistingResources: pkgConfig.DeployOpts.AdoptExistingResources,
		Timeout:                pkgConfig.DeployOpts.Timeout,
		Retries:                pkgConfig.PkgOpts.Retries,
		OCIConcurrency:         config.CommonOptions.OCIConcurrency,
		PlainHTTP:              config.CommonOptions.PlainHTTP,
		InsecureTLSSkipVerify:  config.CommonOptions.InsecureSkipTLSVerify,
		SetVariables:           pkgConfig.PkgOpts.SetVariables,
	})
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
	absCachePath, err := config.GetAbsCachePath()
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
	downloadCacheTarget, err := downloadInitPackage(ctx, absCachePath)
	if err != nil {
		return "", fmt.Errorf("failed to download the init package: %w", err)
	}
	return downloadCacheTarget, nil
}

func downloadInitPackage(ctx context.Context, cacheDirectory string) (string, error) {
	l := logger.From(ctx)
	url := zoci.GetInitPackageURL(config.CLIVersion)

	// Give the user the choice to download the init-package and note that this does require an internet connection
	l.Info("the init package was not found locally, but can be pulled in connected environments", "url", fmt.Sprintf("oci://%s", url))

	var confirmDownload bool
	prompt := &survey.Confirm{
		Message: lang.CmdInitPullConfirm,
	}
	if err := survey.AskOne(prompt, &confirmDownload); err != nil {
		return "", fmt.Errorf("confirm download canceled: %w", err)
	}

	// If the user wants to download the init-package, download it
	if confirmDownload {
		remote, err := zoci.NewRemote(ctx, url, oci.PlatformForArch(config.GetArch()))
		if err != nil {
			return "", err
		}
		source := &sources.OCISource{Remote: remote}
		return source.Collect(ctx, cacheDirectory)
	}
	// Otherwise, exit and tell the user to manually download the init-package
	return "", errors.New(lang.CmdInitPullErrManual)
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
	return nil
}
