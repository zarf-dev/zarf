// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"crypto"
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/packager"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/spf13/cobra"
)

var repoHelmChartPath string
var prepareCmd = &cobra.Command{
	Use:     "prepare",
	Aliases: []string{"prep"},
	Short:   lang.CmdPrepareShort,
}

var prepareTransformGitLinks = &cobra.Command{
	Use:     "patch-git [HOST] [FILE]",
	Aliases: []string{"p"},
	Short:   lang.CmdPreparePatchGitShort,
	Args:    cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		host, fileName := args[0], args[1]

		// Read the contents of the given file
		content, err := os.ReadFile(fileName)
		if err != nil {
			message.Fatalf(err, "Unable to read the file %s", fileName)
		}

		pkgConfig.InitOpts.GitServer.Address = host

		// Perform git url transformation via regex
		text := string(content)
		processedText := transform.MutateGitURLsInText(pkgConfig.InitOpts.GitServer.Address, text, pkgConfig.InitOpts.GitServer.PushUsername)

		// Print the differences
		message.PrintDiff(text, processedText)

		// Ask the user before this destructive action
		confirm := false
		prompt := &survey.Confirm{
			Message: "Overwrite the file " + fileName + " with these changes?",
		}
		if err := survey.AskOne(prompt, &confirm); err != nil {
			message.Fatalf(nil, "Confirm selection canceled: %s", err.Error())
		}

		if confirm {
			// Overwrite the file
			err = os.WriteFile(fileName, []byte(processedText), 0640)
			if err != nil {
				message.Fatal(err, lang.CmdPreparePatchGitFileWriteErr)
			}
		}

	},
}

var prepareComputeFileSha256sum = &cobra.Command{
	Use:     "sha256sum [FILE|URL]",
	Aliases: []string{"s"},
	Short:   lang.CmdPrepareSha256sumShort,
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fileName := args[0]
		hash, err := utils.GetCryptoHash(fileName, crypto.SHA256)
		if err != nil {
			message.Fatal(err, lang.CmdPrepareSha256sumHashErr)
		} else {
			fmt.Println(hash)
		}
	},
}

var prepareFindImages = &cobra.Command{
	Use:     "find-images [PACKAGE]",
	Aliases: []string{"f"},
	Args:    cobra.MaximumNArgs(1),
	Short:   lang.CmdPrepareFindImagesShort,
	Long:    lang.CmdPrepareFindImagesLong,
	Run: func(cmd *cobra.Command, args []string) {
		var baseDir string

		// If a directory was provided, use that as the base directory
		if len(args) > 0 {
			baseDir = args[0]
		}

		// Ensure uppercase keys from viper
		viperConfig := utils.TransformMapKeys(v.GetStringMapString(V_PKG_CREATE_SET), strings.ToUpper)
		pkgConfig.CreateOpts.SetVariables = utils.MergeMap(viperConfig, pkgConfig.CreateOpts.SetVariables)

		// Configure the packager
		pkgClient := packager.NewOrDie(&pkgConfig)
		defer pkgClient.ClearTempPaths()

		// Find all the images the package might need
		if err := pkgClient.FindImages(baseDir, repoHelmChartPath); err != nil {
			message.Fatalf(err, "Unable to find images for the package definition %s", baseDir)
		}
	},
}

var prepareGenerateConfigFile = &cobra.Command{
	Use:     "generate-config [FILENAME]",
	Aliases: []string{"gc"},
	Args:    cobra.MaximumNArgs(1),
	Short:   lang.CmdPrepareGenerateConfigShort,
	Long:    lang.CmdPrepareGenerateConfigLong,
	Run: func(cmd *cobra.Command, args []string) {
		fileName := "zarf-config.toml"

		// If a filename was provided, use that
		if len(args) > 0 {
			fileName = args[0]
		}

		if err := v.SafeWriteConfigAs(fileName); err != nil {
			message.Fatalf(err, "Unable to write the config file %s, make sure the file doesn't already exist", fileName)
		}
	},
}

func init() {
	initViper()

	rootCmd.AddCommand(prepareCmd)
	prepareCmd.AddCommand(prepareTransformGitLinks)
	prepareCmd.AddCommand(prepareComputeFileSha256sum)
	prepareCmd.AddCommand(prepareFindImages)
	prepareCmd.AddCommand(prepareGenerateConfigFile)

	v.SetDefault(V_PKG_CREATE_SET, map[string]string{})

	prepareFindImages.Flags().StringVarP(&repoHelmChartPath, "repo-chart-path", "p", "", lang.CmdPrepareFlagRepoChartPath)
	// use the package create config for this and reset it here to avoid overwriting the config.CreateOptions.SetVariables
	prepareFindImages.Flags().StringToStringVar(&pkgConfig.CreateOpts.SetVariables, "set", v.GetStringMapString(V_PKG_CREATE_SET), lang.CmdPrepareFlagSet)

	prepareTransformGitLinks.Flags().StringVar(&pkgConfig.InitOpts.GitServer.PushUsername, "git-account", config.ZarfGitPushUser, lang.CmdPrepareFlagGitAccount)
}
