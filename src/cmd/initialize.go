// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/src/cmd/common"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/packager"
	"github.com/defenseunicorns/zarf/src/pkg/packager/sources"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"

	"github.com/spf13/cobra"
)

var nodePortArg = 0
var storageClassArg = ""

// initCmd represents the init command.
var initCmd = &cobra.Command{
	Use:     "init",
	Aliases: []string{"i"},
	Short:   lang.CmdInitShort,
	Long:    lang.CmdInitLong,
	Example: lang.CmdInitExample,
	Run: func(cmd *cobra.Command, args []string) {
		zarfLogo := message.GetLogo()
		_, _ = fmt.Fprintln(os.Stderr, zarfLogo)

		if err := validateInitFlags(); err != nil {
			message.Fatal(err, lang.CmdInitErrFlags)
		}

		// Continue running package deploy for all components like any other package
		initPackageName := packager.GetInitPackageName("")
		pkgConfig.PkgOpts.PackageSource = initPackageName

		// Try to use an init-package in the executable directory if none exist in current working directory
		var err error
		if pkgConfig.PkgOpts.PackageSource, err = findInitPackage(initPackageName); err != nil {
			message.Fatal(err, err.Error())
		}

		src, err := sources.New(&pkgConfig.PkgOpts)
		if err != nil {
			message.Fatal(err, err.Error())
		}

		// Ensure uppercase keys from viper
		v := common.GetViper()
		pkgConfig.PkgOpts.SetVariables = helpers.TransformAndMergeMap(
			v.GetStringMapString(common.VPkgDeploySet), pkgConfig.PkgOpts.SetVariables, strings.ToUpper)

		// DEPRECATED_V1.0.0: these functions will need cleanup
		setRegistryStorageClass()
		setRegistryNodePort()

		// Configure the packager
		pkgClient := packager.NewOrDie(&pkgConfig, packager.WithSource(src))
		defer pkgClient.ClearTempPaths()

		// Deploy everything
		err = pkgClient.Deploy()
		if err != nil {
			message.Fatal(err, err.Error())
		}
	},
}

// DEPRECATED_V1.0.0: --nodeport should be removed from the cli in v1.0.0
func setRegistryNodePort() {
	configVar := "REGISTRY_NODEPORT"

	internalRegistry := pkgConfig.InitOpts.RegistryInfo.Address == ""

	// warn for deprecation
	if nodePortArg != 0 {
		message.Warn(lang.WarnNodePortDeprecated)
	}

	// check the --set REGISTRY_NODEPORT first
	configuredNodePort, err := strconv.Atoi(pkgConfig.PkgOpts.SetVariables[configVar])
	if err != nil {
		configuredNodePort = 0
	}

	// check the old --nodeport flag second
	if configuredNodePort == 0 {
		configuredNodePort = nodePortArg
	}

	// the user can't set both that this is an external registry and the nodeport stuff.
	if !internalRegistry && configuredNodePort != 0 {
		message.Fatal(nil, "both --registry-url and --nodeport are set, please only use one")
	}

	if internalRegistry {
		if configuredNodePort > 32767 || configuredNodePort < 30000 {
			configuredNodePort = config.ZarfInClusterContainerRegistryNodePort
		}
		pkgConfig.PkgOpts.SetVariables[configVar] = strconv.Itoa(configuredNodePort)
		pkgConfig.InitOpts.RegistryInfo.Address = fmt.Sprintf("%s:%d", helpers.IPV4Localhost, configuredNodePort)
	} else {
		// do not set the nodeport if this is an external registry
		pkgConfig.PkgOpts.SetVariables[configVar] = ""
	}

	pkgConfig.InitOpts.RegistryInfo.InternalRegistry = internalRegistry

	// TODO: test external registry and CLI.
}

// DEPRECATED_V1.0.0: --storage-class should be removed from the CLI in v1.0.0
func setRegistryStorageClass() {
	configVar := "REGISTRY_STORAGE_CLASS"
	// there is no validation if this storage class is valid
	if storageClassArg != "" {
		message.Warn(lang.WarnStorageClassDeprecated)
	}
	if pkgConfig.PkgOpts.SetVariables[configVar] == "" {
		pkgConfig.PkgOpts.SetVariables[configVar] = storageClassArg
	}
}

func findInitPackage(initPackageName string) (string, error) {
	// First, look for the init package in the current working directory
	if !utils.InvalidPath(initPackageName) {
		return initPackageName, nil
	}

	// Next, look for the init package in the executable directory
	binaryPath, err := utils.GetFinalExecutablePath()
	if err != nil {
		return "", err
	}
	executableDir := path.Dir(binaryPath)
	if !utils.InvalidPath(filepath.Join(executableDir, initPackageName)) {
		return filepath.Join(executableDir, initPackageName), nil
	}

	// Create the cache directory if it doesn't exist
	if utils.InvalidPath(config.GetAbsCachePath()) {
		if err := utils.CreateDirectory(config.GetAbsCachePath(), 0755); err != nil {
			message.Fatalf(err, lang.CmdInitErrUnableCreateCache, config.GetAbsCachePath())
		}
	}

	// Next, look in the cache directory
	if !utils.InvalidPath(filepath.Join(config.GetAbsCachePath(), initPackageName)) {
		return filepath.Join(config.GetAbsCachePath(), initPackageName), nil
	}

	// Finally, if the init-package doesn't exist in the cache directory, suggest downloading it
	downloadCacheTarget, err := downloadInitPackage(config.GetAbsCachePath())
	if err != nil {
		if errors.Is(err, lang.ErrInitNotFound) {
			message.Fatal(err, err.Error())
		} else {
			message.Fatalf(err, lang.CmdInitErrDownload, err.Error())
		}
	}
	return downloadCacheTarget, nil
}

func downloadInitPackage(cacheDirectory string) (string, error) {
	if config.CommonOptions.Confirm {
		return "", lang.ErrInitNotFound
	}

	var confirmDownload bool
	url := oci.GetInitPackageURL(config.GetArch(), config.CLIVersion)

	// Give the user the choice to download the init-package and note that this does require an internet connection
	message.Question(fmt.Sprintf(lang.CmdInitPullAsk, url))

	message.Note(lang.CmdInitPullNote)

	// Prompt the user if --confirm not specified
	if !confirmDownload {
		prompt := &survey.Confirm{
			Message: lang.CmdInitPullConfirm,
		}
		if err := survey.AskOne(prompt, &confirmDownload); err != nil {
			return "", fmt.Errorf(lang.ErrConfirmCancel, err.Error())
		}
	}

	// If the user wants to download the init-package, download it
	if confirmDownload {
		remote, err := oci.NewOrasRemote(url)
		if err != nil {
			return "", err
		}
		source := sources.OCISource{OrasRemote: remote}
		return source.Collect(cacheDirectory)
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
	v.SetDefault(common.VInitGitPushUser, config.ZarfGitPushUser)
	v.SetDefault(common.VInitRegistryPushUser, config.ZarfRegistryPushUser)

	// Init package set variable flags
	initCmd.Flags().StringToStringVar(&pkgConfig.PkgOpts.SetVariables, "set", v.GetStringMapString(common.VPkgDeploySet), lang.CmdInitFlagSet)

	// Continue to require --confirm flag for init command to avoid accidental deployments
	initCmd.Flags().BoolVar(&config.CommonOptions.Confirm, "confirm", false, lang.CmdInitFlagConfirm)
	initCmd.Flags().StringVar(&pkgConfig.PkgOpts.OptionalComponents, "components", v.GetString(common.VInitComponents), lang.CmdInitFlagComponents)

	// [Deprecated] --storage-class is deprecated in favor of --set REGISTRY_STORAGE_CLASS (to be removed in v1.0.0)
	initCmd.Flags().StringVar(&storageClassArg, "storage-class", v.GetString(common.VInitStorageClass), lang.CmdInitFlagStorageClass)
	initCmd.Flags().MarkHidden("storage-class")

	// Flags for using an external Git server
	initCmd.Flags().StringVar(&pkgConfig.InitOpts.GitServer.Address, "git-url", v.GetString(common.VInitGitURL), lang.CmdInitFlagGitURL)
	initCmd.Flags().StringVar(&pkgConfig.InitOpts.GitServer.PushUsername, "git-push-username", v.GetString(common.VInitGitPushUser), lang.CmdInitFlagGitPushUser)
	initCmd.Flags().StringVar(&pkgConfig.InitOpts.GitServer.PushPassword, "git-push-password", v.GetString(common.VInitGitPushPass), lang.CmdInitFlagGitPushPass)
	initCmd.Flags().StringVar(&pkgConfig.InitOpts.GitServer.PullUsername, "git-pull-username", v.GetString(common.VInitGitPullUser), lang.CmdInitFlagGitPullUser)
	initCmd.Flags().StringVar(&pkgConfig.InitOpts.GitServer.PullPassword, "git-pull-password", v.GetString(common.VInitGitPullPass), lang.CmdInitFlagGitPullPass)

	// Flags for using an external registry
	initCmd.Flags().StringVar(&pkgConfig.InitOpts.RegistryInfo.Address, "registry-url", v.GetString(common.VInitRegistryURL), lang.CmdInitFlagRegURL)

	// [Deprecated] --nodeport is deprecated in favor of --set REGISTRY_NODEPORT
	initCmd.Flags().IntVar(&nodePortArg, "nodeport", v.GetInt(common.VInitRegistryNodeport), lang.CmdInitFlagRegNodePort)
	initCmd.Flags().MarkHidden("nodeport")

	initCmd.Flags().StringVar(&pkgConfig.InitOpts.RegistryInfo.PushUsername, "registry-push-username", v.GetString(common.VInitRegistryPushUser), lang.CmdInitFlagRegPushUser)
	initCmd.Flags().StringVar(&pkgConfig.InitOpts.RegistryInfo.PushPassword, "registry-push-password", v.GetString(common.VInitRegistryPushPass), lang.CmdInitFlagRegPushPass)
	initCmd.Flags().StringVar(&pkgConfig.InitOpts.RegistryInfo.PullUsername, "registry-pull-username", v.GetString(common.VInitRegistryPullUser), lang.CmdInitFlagRegPullUser)
	initCmd.Flags().StringVar(&pkgConfig.InitOpts.RegistryInfo.PullPassword, "registry-pull-password", v.GetString(common.VInitRegistryPullPass), lang.CmdInitFlagRegPullPass)
	initCmd.Flags().StringVar(&pkgConfig.InitOpts.RegistryInfo.Secret, "registry-secret", v.GetString(common.VInitRegistrySecret), lang.CmdInitFlagRegSecret)

	// Flags for using an external artifact server
	initCmd.Flags().StringVar(&pkgConfig.InitOpts.ArtifactServer.Address, "artifact-url", v.GetString(common.VInitArtifactURL), lang.CmdInitFlagArtifactURL)
	initCmd.Flags().StringVar(&pkgConfig.InitOpts.ArtifactServer.PushUsername, "artifact-push-username", v.GetString(common.VInitArtifactPushUser), lang.CmdInitFlagArtifactPushUser)
	initCmd.Flags().StringVar(&pkgConfig.InitOpts.ArtifactServer.PushToken, "artifact-push-token", v.GetString(common.VInitArtifactPushToken), lang.CmdInitFlagArtifactPushToken)

	initCmd.Flags().SortFlags = true
}
