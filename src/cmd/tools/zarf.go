// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package tools contains the CLI commands for Zarf.
package tools

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/packager"
	"github.com/defenseunicorns/zarf/src/pkg/pki"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/sigstore/cosign/pkg/cosign"
	"github.com/spf13/cobra"
)

func init() {
	var subAltNames []string
	var outputDirectory string

	readCredsCmd := &cobra.Command{
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
			message.Warn(lang.CmdToolGetGitDeprecation)
			utils.PrintComponentCredential(state, "git")
		},
	}

	readAllCredsCmd := &cobra.Command{
		Use:     "get-creds",
		Short:   lang.CmdToolsGetCredsShort,
		Long:    lang.CmdToolsGetCredsLong,
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
				utils.PrintComponentCredential(state, args[0])
			} else {
				utils.PrintCredentialTable(state, nil)
			}
		},
	}

	clearCacheCmd := &cobra.Command{
		Use:     "clear-cache",
		Aliases: []string{"c"},
		Short:   lang.CmdToolsClearCacheShort,
		Run: func(cmd *cobra.Command, args []string) {
			message.Debugf("Cache directory set to: %s", config.GetAbsCachePath())
			if err := os.RemoveAll(config.GetAbsCachePath()); err != nil {
				message.Fatalf(err, lang.CmdToolsClearCacheErr, config.GetAbsCachePath())
			}
			message.Successf(lang.CmdToolsClearCacheSuccess, config.GetAbsCachePath())
		},
	}

	downloadInitCmd := &cobra.Command{
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

	generatePKICmd := &cobra.Command{
		Use:     "gen-pki {HOST}",
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

	generateKeyCmd := &cobra.Command{
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

	toolsCmd.AddCommand(readCredsCmd)
	toolsCmd.AddCommand(readAllCredsCmd)

	toolsCmd.AddCommand(clearCacheCmd)
	clearCacheCmd.Flags().StringVar(&config.CommonOptions.CachePath, "zarf-cache", config.ZarfDefaultCachePath, lang.CmdToolsClearCacheFlagCachePath)

	toolsCmd.AddCommand(downloadInitCmd)
	downloadInitCmd.Flags().StringVarP(&outputDirectory, "output-directory", "o", "", lang.CmdToolsDownloadInitFlagOutputDirectory)

	toolsCmd.AddCommand(generatePKICmd)
	generatePKICmd.Flags().StringArrayVar(&subAltNames, "sub-alt-name", []string{}, lang.CmdToolsGenPkiFlagAltName)

	toolsCmd.AddCommand(generateKeyCmd)
}
