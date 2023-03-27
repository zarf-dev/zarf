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
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/packager"
	"github.com/defenseunicorns/zarf/src/pkg/utils"

	"github.com/spf13/cobra"
)

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
		pkgConfig.DeployOpts.PackagePath = initPackageName

		// Try to use an init-package in the executable directory if none exist in current working directory
		var err error
		if pkgConfig.DeployOpts.PackagePath, err = findInitPackage(initPackageName); err != nil {
			message.Fatal(err, err.Error())
		}

		// Ensure uppercase keys from viper
		viperConfig := utils.TransformMapKeys(v.GetStringMapString(V_PKG_DEPLOY_SET), strings.ToUpper)
		pkgConfig.DeployOpts.SetVariables = utils.MergeMap(viperConfig, pkgConfig.DeployOpts.SetVariables)

		// Configure the packager
		pkgClient := packager.NewOrDie(&pkgConfig)
		defer pkgClient.ClearTempPaths()

		// Deploy everything
		err = pkgClient.Deploy()
		if err != nil {
			message.Fatal(err, err.Error())
		}
	},
}

func findInitPackage(initPackageName string) (string, error) {
	// First, look for the init package in the current working directory
	if !utils.InvalidPath(initPackageName) {
		return initPackageName, nil
	}

	// Next, look for the init package in the executable directory
	executablePath, err := utils.GetFinalExecutablePath()
	if err != nil {
		return "", err
	}
	executableDir := path.Dir(executablePath)
	if !utils.InvalidPath(filepath.Join(executableDir, initPackageName)) {
		return filepath.Join(executableDir, initPackageName), nil
	}

	// Create the cache directory if it doesn't exist
	if utils.InvalidPath(config.GetAbsCachePath()) {
		if err := os.MkdirAll(config.GetAbsCachePath(), 0755); err != nil {
			message.Fatalf(err, lang.CmdInitErrUnableCreateCache, config.GetAbsCachePath())
		}
	}

	// Next, look in the cache directory
	if !utils.InvalidPath(filepath.Join(config.GetAbsCachePath(), initPackageName)) {
		return filepath.Join(config.GetAbsCachePath(), initPackageName), nil
	}

	// Finally, if the init-package doesn't exist in the cache directory, suggest downloading it
	downloadCacheTarget := filepath.Join(config.GetAbsCachePath(), initPackageName)
	if err := downloadInitPackage(initPackageName, downloadCacheTarget); err != nil {
		if errors.Is(err, lang.ErrInitNotFound) {
			message.Fatal(err, err.Error())
		} else {
			message.Fatalf(err, lang.CmdInitErrDownload, err.Error())
		}
	}
	return downloadCacheTarget, nil
}

func downloadInitPackage(initPackageName, downloadCacheTarget string) error {
	if config.CommonOptions.Confirm {
		return lang.ErrInitNotFound
	}

	var confirmDownload bool
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", config.GithubProject, config.CLIVersion, initPackageName)

	// Give the user the choice to download the init-package and note that this does require an internet connection
	message.Question(fmt.Sprintf(lang.CmdInitDownloadAsk, url))

	message.Note(lang.CmdInitDownloadNote)

	// Prompt the user if --confirm not specified
	if !confirmDownload {
		prompt := &survey.Confirm{
			Message: lang.CmdInitDownloadConfirm,
		}
		if err := survey.AskOne(prompt, &confirmDownload); err != nil {
			message.Fatalf(nil, lang.CmdInitDownloadCancel, err.Error())
		}
	}

	// If the user wants to download the init-package, download it
	if confirmDownload {
		utils.DownloadToFile(url, downloadCacheTarget, "")
	} else {
		// Otherwise, exit and tell the user to manually download the init-package
		return errors.New(lang.CmdInitDownloadErrManual)
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

	//If 'registry-url' is provided, make sure they provided values for the username and password of the push user
	if pkgConfig.InitOpts.RegistryInfo.Address != "" {
		if pkgConfig.InitOpts.RegistryInfo.PushUsername == "" || pkgConfig.InitOpts.RegistryInfo.PushPassword == "" {
			return fmt.Errorf(lang.CmdInitErrValidateRegistry)
		}
	}
	return nil
}

func init() {
	initViper()

	rootCmd.AddCommand(initCmd)

	// Init package variables
	v.SetDefault(V_PKG_DEPLOY_SET, map[string]string{})

	v.SetDefault(V_INIT_COMPONENTS, "")
	v.SetDefault(V_INIT_STORAGE_CLASS, "")

	v.SetDefault(V_INIT_GIT_URL, "")
	v.SetDefault(V_INIT_GIT_PUSH_USER, config.ZarfGitPushUser)
	v.SetDefault(V_INIT_GIT_PUSH_PASS, "")
	v.SetDefault(V_INIT_GIT_PULL_USER, "")
	v.SetDefault(V_INIT_GIT_PULL_PASS, "")

	v.SetDefault(V_INIT_REGISTRY_URL, "")
	v.SetDefault(V_INIT_REGISTRY_NODEPORT, 0)
	v.SetDefault(V_INIT_REGISTRY_SECRET, "")
	v.SetDefault(V_INIT_REGISTRY_PUSH_USER, config.ZarfRegistryPushUser)
	v.SetDefault(V_INIT_REGISTRY_PUSH_PASS, "")
	v.SetDefault(V_INIT_REGISTRY_PULL_USER, "")
	v.SetDefault(V_INIT_REGISTRY_PULL_PASS, "")

	// Init package set variable flags
	initCmd.Flags().StringToStringVar(&pkgConfig.DeployOpts.SetVariables, "set", v.GetStringMapString(V_PKG_DEPLOY_SET), lang.CmdInitFlagSet)

	// Continue to require --confirm flag for init command to avoid accidental deployments
	initCmd.Flags().BoolVar(&config.CommonOptions.Confirm, "confirm", false, lang.CmdInitFlagConfirm)
	initCmd.Flags().StringVar(&pkgConfig.DeployOpts.Components, "components", v.GetString(V_INIT_COMPONENTS), lang.CmdInitFlagComponents)
	initCmd.Flags().StringVar(&pkgConfig.InitOpts.StorageClass, "storage-class", v.GetString(V_INIT_STORAGE_CLASS), lang.CmdInitFlagStorageClass)

	// Flags for using an external Git server
	initCmd.Flags().StringVar(&pkgConfig.InitOpts.GitServer.Address, "git-url", v.GetString(V_INIT_GIT_URL), lang.CmdInitFlagGitURL)
	initCmd.Flags().StringVar(&pkgConfig.InitOpts.GitServer.PushUsername, "git-push-username", v.GetString(V_INIT_GIT_PUSH_USER), lang.CmdInitFlagGitPushUser)
	initCmd.Flags().StringVar(&pkgConfig.InitOpts.GitServer.PushPassword, "git-push-password", v.GetString(V_INIT_GIT_PUSH_PASS), lang.CmdInitFlagGitPushPass)
	initCmd.Flags().StringVar(&pkgConfig.InitOpts.GitServer.PullUsername, "git-pull-username", v.GetString(V_INIT_GIT_PULL_USER), lang.CmdInitFlagGitPullUser)
	initCmd.Flags().StringVar(&pkgConfig.InitOpts.GitServer.PullPassword, "git-pull-password", v.GetString(V_INIT_GIT_PULL_PASS), lang.CmdInitFlagGitPullPass)

	// Flags for using an external registry
	initCmd.Flags().StringVar(&pkgConfig.InitOpts.RegistryInfo.Address, "registry-url", v.GetString(V_INIT_REGISTRY_URL), lang.CmdInitFlagRegURL)
	initCmd.Flags().IntVar(&pkgConfig.InitOpts.RegistryInfo.NodePort, "nodeport", v.GetInt(V_INIT_REGISTRY_NODEPORT), lang.CmdInitFlagRegNodePort)
	initCmd.Flags().StringVar(&pkgConfig.InitOpts.RegistryInfo.PushUsername, "registry-push-username", v.GetString(V_INIT_REGISTRY_PUSH_USER), lang.CmdInitFlagRegPushUser)
	initCmd.Flags().StringVar(&pkgConfig.InitOpts.RegistryInfo.PushPassword, "registry-push-password", v.GetString(V_INIT_REGISTRY_PUSH_PASS), lang.CmdInitFlagRegPushPass)
	initCmd.Flags().StringVar(&pkgConfig.InitOpts.RegistryInfo.PullUsername, "registry-pull-username", v.GetString(V_INIT_REGISTRY_PULL_USER), lang.CmdInitFlagRegPullUser)
	initCmd.Flags().StringVar(&pkgConfig.InitOpts.RegistryInfo.PullPassword, "registry-pull-password", v.GetString(V_INIT_REGISTRY_PULL_PASS), lang.CmdInitFlagRegPullPass)
	initCmd.Flags().StringVar(&pkgConfig.InitOpts.RegistryInfo.Secret, "registry-secret", v.GetString(V_INIT_REGISTRY_SECRET), lang.CmdInitFlagRegSecret)

	initCmd.Flags().SortFlags = true
}
