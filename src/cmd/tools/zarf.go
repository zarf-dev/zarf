// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package tools contains the CLI commands for Zarf.
package tools

import (
	"fmt"
	"os"

	"slices"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/pkg/helpers"
	"github.com/defenseunicorns/pkg/oci"
	"github.com/defenseunicorns/zarf/src/cmd/common"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/internal/packager/git"
	"github.com/defenseunicorns/zarf/src/internal/packager/helm"
	"github.com/defenseunicorns/zarf/src/pkg/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/packager/sources"
	"github.com/defenseunicorns/zarf/src/pkg/pki"
	"github.com/defenseunicorns/zarf/src/pkg/zoci"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/sigstore/cosign/v2/pkg/cosign"
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
	Run: func(_ *cobra.Command, _ []string) {
		message.Warn(lang.CmdToolsGetGitPasswdDeprecation)
		getCredsCmd.Run(getCredsCmd, []string{"git"})
	},
}

var getCredsCmd = &cobra.Command{
	Use:     "get-creds",
	Short:   lang.CmdToolsGetCredsShort,
	Long:    lang.CmdToolsGetCredsLong,
	Example: lang.CmdToolsGetCredsExample,
	Aliases: []string{"gc"},
	Args:    cobra.MaximumNArgs(1),
	Run: func(_ *cobra.Command, args []string) {
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
		validKeys := []string{message.RegistryKey, message.GitKey, message.ArtifactKey, message.AgentKey}
		if len(args) == 0 {
			args = validKeys
		} else {
			if !slices.Contains(validKeys, args[0]) {
				cmd.Help()
				message.Fatalf(nil, lang.CmdToolsUpdateCredsInvalidServiceErr, message.RegistryKey, message.GitKey, message.ArtifactKey)
			}
		}

		c := cluster.NewClusterOrDie()
		oldState, err := c.LoadZarfState()
		if err != nil || oldState.Distro == "" {
			// If no distro the zarf secret did not load properly
			message.Fatalf(nil, lang.ErrLoadState)
		}
		var newState *types.ZarfState
		if newState, err = c.MergeZarfState(oldState, updateCredsInitOpts, args); err != nil {
			message.Fatal(err, lang.CmdToolsUpdateCredsUnableUpdateCreds)
		}

		message.PrintCredentialUpdates(oldState, newState, args)

		confirm := config.CommonOptions.Confirm

		if confirm {
			message.Note(lang.CmdToolsUpdateCredsConfirmProvided)
		} else {
			prompt := &survey.Confirm{
				Message: lang.CmdToolsUpdateCredsConfirmContinue,
			}
			if err := survey.AskOne(prompt, &confirm); err != nil {
				message.Fatalf(nil, lang.ErrConfirmCancel, err)
			}
		}

		if confirm {
			// Update registry and git pull secrets
			if slices.Contains(args, message.RegistryKey) {
				c.UpdateZarfManagedImageSecrets(newState)
			}
			if slices.Contains(args, message.GitKey) {
				c.UpdateZarfManagedGitSecrets(newState)
			}

			// Update artifact token (if internal)
			if slices.Contains(args, message.ArtifactKey) && newState.ArtifactServer.PushToken == "" && newState.ArtifactServer.InternalServer {
				g := git.New(oldState.GitServer)
				tokenResponse, err := g.CreatePackageRegistryToken()
				if err != nil {
					// Warn if we couldn't actually update the git server (it might not be installed and we should try to continue)
					message.Warnf(lang.CmdToolsUpdateCredsUnableCreateToken, err.Error())
				} else {
					newState.ArtifactServer.PushToken = tokenResponse.Sha1
				}
			}

			// Save the final Zarf State
			err = c.SaveZarfState(newState)
			if err != nil {
				message.Fatalf(err, lang.ErrSaveState)
			}

			// Update Zarf 'init' component Helm releases if present
			h := helm.NewClusterOnly(&types.PackagerConfig{State: newState}, c)

			if slices.Contains(args, message.RegistryKey) && newState.RegistryInfo.InternalRegistry {
				err = h.UpdateZarfRegistryValues()
				if err != nil {
					// Warn if we couldn't actually update the registry (it might not be installed and we should try to continue)
					message.Warnf(lang.CmdToolsUpdateCredsUnableUpdateRegistry, err.Error())
				}
			}
			if slices.Contains(args, message.GitKey) && newState.GitServer.InternalServer {
				g := git.New(newState.GitServer)
				err = g.UpdateZarfGiteaUsers(oldState)
				if err != nil {
					// Warn if we couldn't actually update the git server (it might not be installed and we should try to continue)
					message.Warnf(lang.CmdToolsUpdateCredsUnableUpdateGit, err.Error())
				}
			}
			if slices.Contains(args, message.AgentKey) {
				err = h.UpdateZarfAgentValues()
				if err != nil {
					// Warn if we couldn't actually update the agent (it might not be installed and we should try to continue)
					message.Warnf(lang.CmdToolsUpdateCredsUnableUpdateAgent, err.Error())
				}
			}
		}
	},
}

var clearCacheCmd = &cobra.Command{
	Use:     "clear-cache",
	Aliases: []string{"c"},
	Short:   lang.CmdToolsClearCacheShort,
	Run: func(_ *cobra.Command, _ []string) {
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
	Run: func(_ *cobra.Command, _ []string) {
		url := zoci.GetInitPackageURL(config.CLIVersion)

		remote, err := zoci.NewRemote(url, oci.PlatformForArch(config.GetArch()))
		if err != nil {
			message.Fatalf(err, lang.CmdToolsDownloadInitErr, err.Error())
		}

		source := &sources.OCISource{Remote: remote}

		_, err = source.Collect(outputDirectory)
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
	Run: func(_ *cobra.Command, args []string) {
		pki := pki.GeneratePKI(args[0], subAltNames...)
		if err := os.WriteFile("tls.ca", pki.CA, helpers.ReadAllWriteUser); err != nil {
			message.Fatalf(err, lang.ErrWritingFile, "tls.ca", err.Error())
		}
		if err := os.WriteFile("tls.crt", pki.Cert, helpers.ReadAllWriteUser); err != nil {
			message.Fatalf(err, lang.ErrWritingFile, "tls.crt", err.Error())
		}
		if err := os.WriteFile("tls.key", pki.Key, helpers.ReadWriteUser); err != nil {
			message.Fatalf(err, lang.ErrWritingFile, "tls.key", err.Error())
		}
		message.Successf(lang.CmdToolsGenPkiSuccess, args[0])
	},
}

var generateKeyCmd = &cobra.Command{
	Use:     "gen-key",
	Aliases: []string{"key"},
	Short:   lang.CmdToolsGenKeyShort,
	Run: func(_ *cobra.Command, _ []string) {
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
		if err := os.WriteFile(prvKeyFileName, keyBytes.PrivateBytes, helpers.ReadWriteUser); err != nil {
			message.Fatalf(err, lang.ErrWritingFile, prvKeyFileName, err.Error())
		}
		if err := os.WriteFile(pubKeyFileName, keyBytes.PublicBytes, helpers.ReadAllWriteUser); err != nil {
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

	// Always require confirm flag (no viper)
	updateCredsCmd.Flags().BoolVar(&config.CommonOptions.Confirm, "confirm", false, lang.CmdToolsUpdateCredsConfirmFlag)

	// Flags for using an external Git server
	updateCredsCmd.Flags().StringVar(&updateCredsInitOpts.GitServer.Address, "git-url", v.GetString(common.VInitGitURL), lang.CmdInitFlagGitURL)
	updateCredsCmd.Flags().StringVar(&updateCredsInitOpts.GitServer.PushUsername, "git-push-username", v.GetString(common.VInitGitPushUser), lang.CmdInitFlagGitPushUser)
	updateCredsCmd.Flags().StringVar(&updateCredsInitOpts.GitServer.PushPassword, "git-push-password", v.GetString(common.VInitGitPushPass), lang.CmdInitFlagGitPushPass)
	updateCredsCmd.Flags().StringVar(&updateCredsInitOpts.GitServer.PullUsername, "git-pull-username", v.GetString(common.VInitGitPullUser), lang.CmdInitFlagGitPullUser)
	updateCredsCmd.Flags().StringVar(&updateCredsInitOpts.GitServer.PullPassword, "git-pull-password", v.GetString(common.VInitGitPullPass), lang.CmdInitFlagGitPullPass)

	// Flags for using an external registry
	updateCredsCmd.Flags().StringVar(&updateCredsInitOpts.RegistryInfo.Address, "registry-url", v.GetString(common.VInitRegistryURL), lang.CmdInitFlagRegURL)
	updateCredsCmd.Flags().StringVar(&updateCredsInitOpts.RegistryInfo.PushUsername, "registry-push-username", v.GetString(common.VInitRegistryPushUser), lang.CmdInitFlagRegPushUser)
	updateCredsCmd.Flags().StringVar(&updateCredsInitOpts.RegistryInfo.PushPassword, "registry-push-password", v.GetString(common.VInitRegistryPushPass), lang.CmdInitFlagRegPushPass)
	updateCredsCmd.Flags().StringVar(&updateCredsInitOpts.RegistryInfo.PullUsername, "registry-pull-username", v.GetString(common.VInitRegistryPullUser), lang.CmdInitFlagRegPullUser)
	updateCredsCmd.Flags().StringVar(&updateCredsInitOpts.RegistryInfo.PullPassword, "registry-pull-password", v.GetString(common.VInitRegistryPullPass), lang.CmdInitFlagRegPullPass)

	// Flags for using an external artifact server
	updateCredsCmd.Flags().StringVar(&updateCredsInitOpts.ArtifactServer.Address, "artifact-url", v.GetString(common.VInitArtifactURL), lang.CmdInitFlagArtifactURL)
	updateCredsCmd.Flags().StringVar(&updateCredsInitOpts.ArtifactServer.PushUsername, "artifact-push-username", v.GetString(common.VInitArtifactPushUser), lang.CmdInitFlagArtifactPushUser)
	updateCredsCmd.Flags().StringVar(&updateCredsInitOpts.ArtifactServer.PushToken, "artifact-push-token", v.GetString(common.VInitArtifactPushToken), lang.CmdInitFlagArtifactPushToken)

	updateCredsCmd.Flags().SortFlags = true

	toolsCmd.AddCommand(clearCacheCmd)
	clearCacheCmd.Flags().StringVar(&config.CommonOptions.CachePath, "zarf-cache", config.ZarfDefaultCachePath, lang.CmdToolsClearCacheFlagCachePath)

	toolsCmd.AddCommand(downloadInitCmd)
	downloadInitCmd.Flags().StringVarP(&outputDirectory, "output-directory", "o", "", lang.CmdToolsDownloadInitFlagOutputDirectory)

	toolsCmd.AddCommand(generatePKICmd)
	generatePKICmd.Flags().StringArrayVar(&subAltNames, "sub-alt-name", []string{}, lang.CmdToolsGenPkiFlagAltName)

	toolsCmd.AddCommand(generateKeyCmd)
}
