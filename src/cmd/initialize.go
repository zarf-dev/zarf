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
	"github.com/defenseunicorns/pkg/oci"
	"github.com/zarf-dev/zarf/src/cmd/common"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/packager"
	"github.com/zarf-dev/zarf/src/pkg/packager/sources"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
	"github.com/zarf-dev/zarf/src/types"

	"github.com/spf13/cobra"
)

// initCmd represents the init command.
var initCmd = &cobra.Command{
	Use:     "init",
	Aliases: []string{"i"},
	Short:   lang.CmdInitShort,
	Long:    lang.CmdInitLong,
	Example: lang.CmdInitExample,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		if err := validateInitFlags(); err != nil {
			return fmt.Errorf("invalid command flags were provided: %w", err)
		}

		// Continue running package deploy for all components like any other package
		initPackageName := sources.GetInitPackageName()
		pkgConfig.PkgOpts.PackageSource = initPackageName

		// Try to use an init-package in the executable directory if none exist in current working directory
		var err error
		if pkgConfig.PkgOpts.PackageSource, err = findInitPackage(cmd.Context(), initPackageName); err != nil {
			return err
		}

		src, err := sources.New(ctx, &pkgConfig.PkgOpts)
		if err != nil {
			return err
		}

		v := common.GetViper()
		pkgConfig.PkgOpts.SetVariables = helpers.TransformAndMergeMap(
			v.GetStringMapString(common.VPkgDeploySet), pkgConfig.PkgOpts.SetVariables, strings.ToUpper)

		pkgClient, err := packager.New(&pkgConfig, packager.WithSource(src))
		if err != nil {
			return err
		}
		defer pkgClient.ClearTempPaths()

		err = pkgClient.Deploy(ctx)
		if err != nil {
			return err
		}
		// Since the new logger ignores pterm output the credential table is no longer printed on init.
		// This note is the intended replacement, rather than printing creds by default.
		logger.From(ctx).Info("init complete. To get credentials for Zarf deployed services run `zarf tools get-creds`")
		return nil
	},
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
	message.Question(fmt.Sprintf(lang.CmdInitPullAsk, url))
	message.Note(lang.CmdInitPullNote)
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

func init() {
	v := common.InitViper()

	rootCmd.AddCommand(initCmd)

	// Init package variable defaults that are non-zero values
	// NOTE: these are not in common.setDefaults so that zarf tools update-creds does not erroneously update values back to the default
	v.SetDefault(common.VInitGitPushUser, types.ZarfGitPushUser)
	v.SetDefault(common.VInitRegistryPushUser, types.ZarfRegistryPushUser)

	// Init package set variable flags
	initCmd.Flags().StringToStringVar(&pkgConfig.PkgOpts.SetVariables, "set", v.GetStringMapString(common.VPkgDeploySet), lang.CmdInitFlagSet)

	// Continue to require --confirm flag for init command to avoid accidental deployments
	initCmd.Flags().BoolVar(&config.CommonOptions.Confirm, "confirm", false, lang.CmdInitFlagConfirm)
	initCmd.Flags().StringVar(&pkgConfig.PkgOpts.OptionalComponents, "components", v.GetString(common.VInitComponents), lang.CmdInitFlagComponents)
	initCmd.Flags().StringVar(&pkgConfig.InitOpts.StorageClass, "storage-class", v.GetString(common.VInitStorageClass), lang.CmdInitFlagStorageClass)

	// Flags for using an external Git server
	initCmd.Flags().StringVar(&pkgConfig.InitOpts.GitServer.Address, "git-url", v.GetString(common.VInitGitURL), lang.CmdInitFlagGitURL)
	initCmd.Flags().StringVar(&pkgConfig.InitOpts.GitServer.PushUsername, "git-push-username", v.GetString(common.VInitGitPushUser), lang.CmdInitFlagGitPushUser)
	initCmd.Flags().StringVar(&pkgConfig.InitOpts.GitServer.PushPassword, "git-push-password", v.GetString(common.VInitGitPushPass), lang.CmdInitFlagGitPushPass)
	initCmd.Flags().StringVar(&pkgConfig.InitOpts.GitServer.PullUsername, "git-pull-username", v.GetString(common.VInitGitPullUser), lang.CmdInitFlagGitPullUser)
	initCmd.Flags().StringVar(&pkgConfig.InitOpts.GitServer.PullPassword, "git-pull-password", v.GetString(common.VInitGitPullPass), lang.CmdInitFlagGitPullPass)

	// Flags for using an external registry
	initCmd.Flags().StringVar(&pkgConfig.InitOpts.RegistryInfo.Address, "registry-url", v.GetString(common.VInitRegistryURL), lang.CmdInitFlagRegURL)
	initCmd.Flags().IntVar(&pkgConfig.InitOpts.RegistryInfo.NodePort, "nodeport", v.GetInt(common.VInitRegistryNodeport), lang.CmdInitFlagRegNodePort)
	initCmd.Flags().StringVar(&pkgConfig.InitOpts.RegistryInfo.PushUsername, "registry-push-username", v.GetString(common.VInitRegistryPushUser), lang.CmdInitFlagRegPushUser)
	initCmd.Flags().StringVar(&pkgConfig.InitOpts.RegistryInfo.PushPassword, "registry-push-password", v.GetString(common.VInitRegistryPushPass), lang.CmdInitFlagRegPushPass)
	initCmd.Flags().StringVar(&pkgConfig.InitOpts.RegistryInfo.PullUsername, "registry-pull-username", v.GetString(common.VInitRegistryPullUser), lang.CmdInitFlagRegPullUser)
	initCmd.Flags().StringVar(&pkgConfig.InitOpts.RegistryInfo.PullPassword, "registry-pull-password", v.GetString(common.VInitRegistryPullPass), lang.CmdInitFlagRegPullPass)
	initCmd.Flags().StringVar(&pkgConfig.InitOpts.RegistryInfo.Secret, "registry-secret", v.GetString(common.VInitRegistrySecret), lang.CmdInitFlagRegSecret)

	// Flags for using an external artifact server
	initCmd.Flags().StringVar(&pkgConfig.InitOpts.ArtifactServer.Address, "artifact-url", v.GetString(common.VInitArtifactURL), lang.CmdInitFlagArtifactURL)
	initCmd.Flags().StringVar(&pkgConfig.InitOpts.ArtifactServer.PushUsername, "artifact-push-username", v.GetString(common.VInitArtifactPushUser), lang.CmdInitFlagArtifactPushUser)
	initCmd.Flags().StringVar(&pkgConfig.InitOpts.ArtifactServer.PushToken, "artifact-push-token", v.GetString(common.VInitArtifactPushToken), lang.CmdInitFlagArtifactPushToken)

	// Flags that control how a deployment proceeds
	// Always require adopt-existing-resources flag (no viper)
	initCmd.Flags().BoolVar(&pkgConfig.DeployOpts.AdoptExistingResources, "adopt-existing-resources", false, lang.CmdPackageDeployFlagAdoptExistingResources)
	initCmd.Flags().DurationVar(&pkgConfig.DeployOpts.Timeout, "timeout", v.GetDuration(common.VPkgDeployTimeout), lang.CmdPackageDeployFlagTimeout)

	initCmd.Flags().IntVar(&pkgConfig.PkgOpts.Retries, "retries", v.GetInt(common.VPkgRetries), lang.CmdPackageFlagRetries)
	initCmd.Flags().StringVarP(&pkgConfig.PkgOpts.PublicKeyPath, "key", "k", v.GetString(common.VPkgPublicKey), lang.CmdPackageFlagFlagPublicKey)
	initCmd.Flags().BoolVar(&pkgConfig.PkgOpts.SkipSignatureValidation, "skip-signature-validation", false, lang.CmdPackageFlagSkipSignatureValidation)

	initCmd.Flags().SortFlags = true
}
