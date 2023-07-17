// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package tools contains the CLI commands for Zarf.
package tools

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/src/cmd/common"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/internal/packager/git"
	"github.com/defenseunicorns/zarf/src/internal/packager/helm"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/packager"
	"github.com/defenseunicorns/zarf/src/pkg/pki"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/sigstore/cosign/pkg/cosign"
	"github.com/spf13/cobra"
)

var subAltNames []string
var outputDirectory string
var updateCredsInitOpts types.ZarfInitOptions

var deprecatedGetGitCredsCmd = &cobra.Command{
	Use:    "get-git-password",
	Hidden: true,
	Short:  lang.CmdToolsGetGitPasswdShort,
	Long:   lang.CmdToolsGetGitPasswdLong,
	Run: func(cmd *cobra.Command, args []string) {
		state, err := cluster.NewClusterOrDie().LoadZarfState()
		if err != nil || state.Distro == "" {
			// If no distro the zarf secret did not load properly
			message.Fatalf(nil, lang.ErrLoadState)
		}

		message.Note(lang.CmdToolsGetGitPasswdInfo)
		message.Warn(lang.CmdToolsGetGitPasswdDeprecation)
		message.PrintComponentCredential(state, "git")
	},
}

var getCredsCmd = &cobra.Command{
	Use:     "get-creds",
	Short:   lang.CmdToolsGetCredsShort,
	Long:    lang.CmdToolsGetCredsLong,
	Example: lang.CmdToolsGetCredsExample,
	Aliases: []string{"gc"},
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		state, err := cluster.NewClusterOrDie().LoadZarfState()
		if err != nil || state.Distro == "" {
			// If no distro the zarf secret did not load properly
			message.Fatalf(nil, lang.ErrLoadState)
		}

		if len(args) > 0 {
			// If a component name is provided, only show that component's credentials
			message.PrintComponentCredential(state, args[0])
		} else {
			message.PrintCredentialTable(state, nil)
		}
	},
}

var updateCredsCmd = &cobra.Command{
	Use:     "update-creds",
	Short:   lang.CmdToolsUpdateCredsShort,
	Long:    lang.CmdToolsUpdateCredsLong,
	Example: lang.CmdToolsUpdateCredsExample,
	Aliases: []string{"uc"},
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		validKeys := []string{message.RegistryKey, message.GitKey, message.ArtifactKey, message.LoggingKey}
		if len(args) == 0 {
			args = validKeys
		} else {
			if !helpers.SliceContains(validKeys, args[0]) {
				cmd.Help()
				message.Fatalf(nil, "Invalid service key specified - valid keys are: %s, %s, %s, and %s", message.RegistryKey, message.GitKey, message.ArtifactKey, message.LoggingKey)
			}
		}

		c := cluster.NewClusterOrDie()
		oldState, err := c.LoadZarfState()
		if err != nil || oldState.Distro == "" {
			// If no distro the zarf secret did not load properly
			message.Fatalf(nil, lang.ErrLoadState)
		}
		initPackage, err := c.GetDeployedPackage("init")
		if err != nil || oldState.Distro == "" {
			// If no distro the zarf secret did not load properly
			message.Fatalf(nil, "Unable to load init package information from the cluster")
		}

		hasRegistry := false
		hasGitServer := false
		hasLogging := false
		for _, dc := range initPackage.DeployedComponents {
			if dc.Name == "zarf-registry" {
				hasRegistry = true
			}
			if dc.Name == "git-server" {
				hasGitServer = true
			}
			if dc.Name == "logging" {
				hasGitServer = true
			}
		}

		newState := oldState

		if helpers.SliceContains(args, message.RegistryKey) {
			newState.RegistryInfo = helpers.MergeNonZero(newState.RegistryInfo, updateCredsInitOpts.RegistryInfo)
		}
		if helpers.SliceContains(args, message.GitKey) {
			newState.GitServer = helpers.MergeNonZero(newState.GitServer, updateCredsInitOpts.GitServer)
		}
		if helpers.SliceContains(args, message.ArtifactKey) {
			newState.ArtifactServer = helpers.MergeNonZero(newState.ArtifactServer, updateCredsInitOpts.ArtifactServer)
		}
		if helpers.SliceContains(args, message.LoggingKey) {
			newState.LoggingSecret = ""
		}

		message.PrintCredentialUpdates(oldState, newState, args)

		confirm := false
		prompt := &survey.Confirm{
			Message: "Continue with these changes?",
		}
		if err := survey.AskOne(prompt, &confirm); err != nil {
			message.Fatalf(nil, lang.ErrConfirmCancel, err)
		}

		if confirm {
			if helpers.SliceContains(args, message.RegistryKey) {
				if newState.RegistryInfo.PushPassword == oldState.RegistryInfo.PushPassword && hasRegistry {
					newState.RegistryInfo.PushPassword = utils.RandomString(config.ZarfGeneratedPasswordLen)
				}
				if newState.RegistryInfo.PullPassword == oldState.RegistryInfo.PullPassword && hasRegistry {
					newState.RegistryInfo.PullPassword = utils.RandomString(config.ZarfGeneratedPasswordLen)
				}
				c.UpdateZarfManagedImageSecrets(newState)
			}
			if helpers.SliceContains(args, message.GitKey) {
				if newState.GitServer.PushPassword == oldState.GitServer.PushPassword && hasGitServer {
					newState.GitServer.PushPassword = utils.RandomString(config.ZarfGeneratedPasswordLen)
				}
				if newState.GitServer.PullPassword == oldState.GitServer.PullPassword && hasGitServer {
					newState.GitServer.PullPassword = utils.RandomString(config.ZarfGeneratedPasswordLen)
				}
				c.UpdateZarfManagedGitSecrets(newState)
			}
			if helpers.SliceContains(args, message.ArtifactKey) {
				if newState.ArtifactServer.PushToken == oldState.ArtifactServer.PushToken && hasGitServer {
					g := git.New(newState.GitServer)
					tokenResponse, err := g.CreatePackageRegistryToken()
					if err != nil {
						message.Fatalf(nil, "Unable to create the new Gitea artifact token")
					}
					newState.ArtifactServer.PushToken = tokenResponse.Sha1
				}
			}
			if helpers.SliceContains(args, message.LoggingKey) {
				newState.LoggingSecret = utils.RandomString(config.ZarfGeneratedPasswordLen)
			}

			err = c.SaveZarfState(newState)
			if err != nil {
				message.Fatalf(nil, lang.ErrSaveState)
			}

			if helpers.SliceContains(args, message.RegistryKey) && hasRegistry {
				pushUser, err := utils.GetHtpasswdString(newState.RegistryInfo.PushUsername, newState.RegistryInfo.PushPassword)
				if err != nil {
					message.Fatalf(nil, "error generating htpasswd string: %s", err.Error())
				}

				pullUser, err := utils.GetHtpasswdString(newState.RegistryInfo.PullUsername, newState.RegistryInfo.PullPassword)
				if err != nil {
					message.Fatalf(nil, "error generating htpasswd string: %s", err.Error())
				}

				registryValues := map[string]interface{}{}
				registrySecrets := map[string]interface{}{}
				registrySecrets["htpasswd"] = fmt.Sprintf("%s\n%s", pushUser, pullUser)
				registryValues["secrets"] = registrySecrets

				h := helm.Helm{
					Chart: types.ZarfChart{
						Namespace: "zarf",
					},
					Cluster:     c,
					ReleaseName: "zarf-docker-registry",
					Cfg: &types.PackagerConfig{
						State: newState,
					},
				}
				_, err = h.UpdateReleaseValues(registryValues)
				if err != nil {
					message.Fatalf(nil, "error updating the release values: %s", err.Error())
				}
			}
			if helpers.SliceContains(args, message.GitKey) && hasGitServer {
				// TODO: Apply the updates to the gitea helm chart
			}
			if helpers.SliceContains(args, message.LoggingKey) && hasLogging {
				// TODO: Apply the updates to the logging helm chart
			}
		}
	},
}

var clearCacheCmd = &cobra.Command{
	Use:     "clear-cache",
	Aliases: []string{"c"},
	Short:   lang.CmdToolsClearCacheShort,
	Run: func(cmd *cobra.Command, args []string) {
		message.Notef(lang.CmdToolsClearCacheDir, config.GetAbsCachePath())
		if err := os.RemoveAll(config.GetAbsCachePath()); err != nil {
			message.Fatalf(err, lang.CmdToolsClearCacheErr, config.GetAbsCachePath())
		}
		message.Successf(lang.CmdToolsClearCacheSuccess, config.GetAbsCachePath())
	},
}

var downloadInitCmd = &cobra.Command{
	Use:   "download-init",
	Short: lang.CmdToolsDownloadInitShort,
	Run: func(cmd *cobra.Command, args []string) {
		initPackageName := packager.GetInitPackageName("")
		target := filepath.Join(outputDirectory, initPackageName)
		url := packager.GetInitPackageRemote("")
		err := utils.DownloadToFile(url, target, "")
		if err != nil {
			message.Fatalf(err, lang.CmdToolsDownloadInitErr, err.Error())
		}
	},
}

var generatePKICmd = &cobra.Command{
	Use:     "gen-pki HOST",
	Aliases: []string{"pki"},
	Short:   lang.CmdToolsGenPkiShort,
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		pki := pki.GeneratePKI(args[0], subAltNames...)
		if err := os.WriteFile("tls.ca", pki.CA, 0644); err != nil {
			message.Fatalf(err, lang.ErrWritingFile, "tls.ca", err.Error())
		}
		if err := os.WriteFile("tls.crt", pki.Cert, 0644); err != nil {
			message.Fatalf(err, lang.ErrWritingFile, "tls.crt", err.Error())
		}
		if err := os.WriteFile("tls.key", pki.Key, 0600); err != nil {
			message.Fatalf(err, lang.ErrWritingFile, "tls.key", err.Error())
		}
		message.Successf(lang.CmdToolsGenPkiSuccess, args[0])
	},
}

var generateKeyCmd = &cobra.Command{
	Use:     "gen-key",
	Aliases: []string{"key"},
	Short:   lang.CmdToolsGenKeyShort,
	Run: func(cmd *cobra.Command, args []string) {
		// Utility function to prompt the user for the password to the private key
		passwordFunc := func(bool) ([]byte, error) {
			// perform the first prompt
			var password string
			prompt := &survey.Password{
				Message: lang.CmdToolsGenKeyPrompt,
			}
			if err := survey.AskOne(prompt, &password); err != nil {
				return nil, fmt.Errorf(lang.CmdToolsGenKeyErrUnableGetPassword, err.Error())
			}

			// perform the second prompt
			var doubleCheck string
			rePrompt := &survey.Password{
				Message: lang.CmdToolsGenKeyPromptAgain,
			}
			if err := survey.AskOne(rePrompt, &doubleCheck); err != nil {
				return nil, fmt.Errorf(lang.CmdToolsGenKeyErrUnableGetPassword, err.Error())
			}

			// check if the passwords match
			if password != doubleCheck {
				return nil, fmt.Errorf(lang.CmdToolsGenKeyErrPasswordsNotMatch)
			}

			return []byte(password), nil
		}

		// Use cosign to generate the keypair
		keyBytes, err := cosign.GenerateKeyPair(passwordFunc)
		if err != nil {
			message.Fatalf(err, lang.CmdToolsGenKeyErrUnableToGenKeypair, err.Error())
		}

		prvKeyFileName := "cosign.key"
		pubKeyFileName := "cosign.pub"

		// Check if we are about to overwrite existing key files
		_, prvKeyExistsErr := os.Stat(prvKeyFileName)
		_, pubKeyExistsErr := os.Stat(pubKeyFileName)
		if prvKeyExistsErr == nil || pubKeyExistsErr == nil {
			var confirm bool
			confirmOverwritePrompt := &survey.Confirm{
				Message: fmt.Sprintf(lang.CmdToolsGenKeyPromptExists, prvKeyFileName),
			}
			err := survey.AskOne(confirmOverwritePrompt, &confirm)
			if err != nil {
				message.Fatalf(err, lang.CmdToolsGenKeyErrNoConfirmOverwrite)
			}

			if !confirm {
				message.Fatal(nil, lang.CmdToolsGenKeyErrNoConfirmOverwrite)
			}
		}

		// Write the key file contents to disk
		if err := os.WriteFile(prvKeyFileName, keyBytes.PrivateBytes, 0600); err != nil {
			message.Fatalf(err, lang.ErrWritingFile, prvKeyFileName, err.Error())
		}
		if err := os.WriteFile(pubKeyFileName, keyBytes.PublicBytes, 0644); err != nil {
			message.Fatalf(err, lang.ErrWritingFile, pubKeyFileName, err.Error())
		}

		message.Successf(lang.CmdToolsGenKeySuccess, prvKeyFileName, pubKeyFileName)
	},
}

func init() {
	v := common.InitViper()

	toolsCmd.AddCommand(deprecatedGetGitCredsCmd)
	toolsCmd.AddCommand(getCredsCmd)

	toolsCmd.AddCommand(updateCredsCmd)

	// Flags for using an external Git server
	updateCredsCmd.Flags().StringVar(&updateCredsInitOpts.GitServer.Address, "git-url", v.GetString(common.V_INIT_GIT_URL), lang.CmdInitFlagGitURL)
	updateCredsCmd.Flags().StringVar(&updateCredsInitOpts.GitServer.PushUsername, "git-push-username", v.GetString(common.V_INIT_GIT_PUSH_USER), lang.CmdInitFlagGitPushUser)
	updateCredsCmd.Flags().StringVar(&updateCredsInitOpts.GitServer.PushPassword, "git-push-password", v.GetString(common.V_INIT_GIT_PUSH_PASS), lang.CmdInitFlagGitPushPass)
	updateCredsCmd.Flags().StringVar(&updateCredsInitOpts.GitServer.PullUsername, "git-pull-username", v.GetString(common.V_INIT_GIT_PULL_USER), lang.CmdInitFlagGitPullUser)
	updateCredsCmd.Flags().StringVar(&updateCredsInitOpts.GitServer.PullPassword, "git-pull-password", v.GetString(common.V_INIT_GIT_PULL_PASS), lang.CmdInitFlagGitPullPass)

	// Flags for using an external registry
	updateCredsCmd.Flags().StringVar(&updateCredsInitOpts.RegistryInfo.Address, "registry-url", v.GetString(common.V_INIT_REGISTRY_URL), lang.CmdInitFlagRegURL)
	updateCredsCmd.Flags().StringVar(&updateCredsInitOpts.RegistryInfo.PushUsername, "registry-push-username", v.GetString(common.V_INIT_REGISTRY_PUSH_USER), lang.CmdInitFlagRegPushUser)
	updateCredsCmd.Flags().StringVar(&updateCredsInitOpts.RegistryInfo.PushPassword, "registry-push-password", v.GetString(common.V_INIT_REGISTRY_PUSH_PASS), lang.CmdInitFlagRegPushPass)
	updateCredsCmd.Flags().StringVar(&updateCredsInitOpts.RegistryInfo.PullUsername, "registry-pull-username", v.GetString(common.V_INIT_REGISTRY_PULL_USER), lang.CmdInitFlagRegPullUser)
	updateCredsCmd.Flags().StringVar(&updateCredsInitOpts.RegistryInfo.PullPassword, "registry-pull-password", v.GetString(common.V_INIT_REGISTRY_PULL_PASS), lang.CmdInitFlagRegPullPass)

	// Flags for using an external artifact server
	updateCredsCmd.Flags().StringVar(&updateCredsInitOpts.ArtifactServer.Address, "artifact-url", v.GetString(common.V_INIT_ARTIFACT_URL), lang.CmdInitFlagArtifactURL)
	updateCredsCmd.Flags().StringVar(&updateCredsInitOpts.ArtifactServer.PushUsername, "artifact-push-username", v.GetString(common.V_INIT_ARTIFACT_PUSH_USER), lang.CmdInitFlagArtifactPushUser)
	updateCredsCmd.Flags().StringVar(&updateCredsInitOpts.ArtifactServer.PushToken, "artifact-push-token", v.GetString(common.V_INIT_ARTIFACT_PUSH_TOKEN), lang.CmdInitFlagArtifactPushToken)

	updateCredsCmd.Flags().SortFlags = true

	toolsCmd.AddCommand(clearCacheCmd)
	clearCacheCmd.Flags().StringVar(&config.CommonOptions.CachePath, "zarf-cache", config.ZarfDefaultCachePath, lang.CmdToolsClearCacheFlagCachePath)

	toolsCmd.AddCommand(downloadInitCmd)
	downloadInitCmd.Flags().StringVarP(&outputDirectory, "output-directory", "o", "", lang.CmdToolsDownloadInitFlagOutputDirectory)

	toolsCmd.AddCommand(generatePKICmd)
	generatePKICmd.Flags().StringArrayVar(&subAltNames, "sub-alt-name", []string{}, lang.CmdToolsGenPkiFlagAltName)

	toolsCmd.AddCommand(generateKeyCmd)
}
