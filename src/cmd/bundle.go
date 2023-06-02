// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/bundler"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	goyaml "github.com/goccy/go-yaml"
	"github.com/mholt/archiver/v3"
	"github.com/spf13/cobra"
)

var bundleCmd = &cobra.Command{
	Use:     "bundle",
	Aliases: []string{"b"},
	Short:   lang.CmdBundleShort,
}

var bundleCreateCmd = &cobra.Command{
	Use:     "create [DIRECTORY]",
	Aliases: []string{"c"},
	Args:    cobra.MaximumNArgs(1),
	Short:   lang.CmdBundleCreateShort,
	Long:    lang.CmdBundleCreateLong,
	Run: func(cmd *cobra.Command, args []string) {

		var baseDir string

		// If a directory was provided, use that as the base directory
		if len(args) > 0 {
			baseDir = args[0]
		}

		var isCleanPathRegex = regexp.MustCompile(`^[a-zA-Z0-9\_\-\/\.\~\\:]+$`)
		if !isCleanPathRegex.MatchString(config.CommonOptions.CachePath) {
			message.Warnf("Invalid characters in Zarf cache path, defaulting to %s", config.ZarfDefaultCachePath)
			config.CommonOptions.CachePath = config.ZarfDefaultCachePath
		}

		// Ensure uppercase keys from viper
		viperConfig := utils.TransformMapKeys(v.GetStringMapString(V_PKG_CREATE_SET), strings.ToUpper)
		bndlConfig.CreateOpts.SetVariables = utils.MergeMap(viperConfig, bndlConfig.CreateOpts.SetVariables)

		// Configure the bundler
		bndlClient := bundler.NewOrDie(&bndlConfig)
		defer bndlClient.ClearTempPaths()

		// Create the bundle
		if err := bndlClient.Create(baseDir); err != nil {
			message.Fatalf(err, "Failed to create bundle: %s", err.Error())
		}
	},
}

var bundleDeployCmd = &cobra.Command{
	Use:     "deploy [PACKAGE]",
	Aliases: []string{"d"},
	Short:   lang.CmdBundleDeployShort,
	Long:    lang.CmdBundleDeployLong,
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		bndlConfig.BndlSource = chooseBundle(args)

		// Ensure uppercase keys from viper and CLI --set
		viperConfigSetVariables := utils.TransformMapKeys(v.GetStringMapString(V_PKG_DEPLOY_SET), strings.ToUpper)
		bndlConfig.DeployOpts.SetVariables = utils.TransformMapKeys(bndlConfig.DeployOpts.SetVariables, strings.ToUpper)

		// Merge the viper config file variables and provided CLI flag variables (CLI takes precedence))
		bndlConfig.DeployOpts.SetVariables = utils.MergeMap(viperConfigSetVariables, bndlConfig.DeployOpts.SetVariables)

		// Configure the bundler
		bndlClient := bundler.NewOrDie(&bndlConfig)
		defer bndlClient.ClearTempPaths()

		// Deploy the bundle
		if err := bndlClient.Deploy(); err != nil {
			message.Fatalf(err, "Failed to deploy bundle: %s", err.Error())
		}
	},
}

var bundleInspectCmd = &cobra.Command{
	Use:     "inspect [PACKAGE]",
	Aliases: []string{"i"},
	Short:   lang.CmdBundleInspectShort,
	Long:    lang.CmdBundleInspectLong,
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		bndlConfig.BndlSource = chooseBundle(args)

		// Configure the bundler
		bndlClient := bundler.NewOrDie(&bndlConfig)
		defer bndlClient.ClearTempPaths()

		// Inspect the bundle
		if err := bndlClient.Inspect(inspectPublicKey); err != nil {
			message.Fatalf(err, "Failed to inspect bundle: %s", err.Error())
		}
	},
}

var bundleRemoveCmd = &cobra.Command{
	Use:     "remove {PACKAGE_NAME|PACKAGE_FILE}",
	Aliases: []string{"u"},
	Args:    cobra.ExactArgs(1),
	Short:   lang.CmdBundleRemoveShort,
	Run: func(cmd *cobra.Command, args []string) {
		bndlName := args[0]

		// If the user input is a path to a bundle, extract the name from the bundle
		isTarball := regexp.MustCompile(`.*zarf-bundle-.*\.tar\.zst$`).MatchString
		if isTarball(bndlName) {
			if utils.InvalidPath(bndlName) {
				message.Fatalf(nil, lang.CmdBundleRemoveTarballErr)
			}

			var bndl types.ZarfBundle
			err := archiver.Walk(bndlName, func(f archiver.File) error {
				if f.Name() == config.ZarfYAML {
					contents, err := io.ReadAll(f)
					if err != nil {
						return err
					}
					if err := goyaml.Unmarshal(contents, &bndl); err != nil {
						message.Fatalf(err, lang.CmdBundleRemoveReadZarfErr)
					}
					return archiver.ErrStopWalk
				}
				return nil
			})

			if err != nil {
				message.Fatalf(err, lang.CmdBundleRemoveExtractErr)
			}

			bndlName = bndl.Metadata.Name
			bndlConfig.Bndl = bndl
		}

		// Configure the bundler
		bndlClient := bundler.NewOrDie(&bndlConfig)
		defer bndlClient.ClearTempPaths()

		if err := bndlClient.Remove(bndlName); err != nil {
			message.Fatalf(err, "Unable to remove the bundle with an error of: %#v", err)
		}
	},
}

var bundlePullCmd = &cobra.Command{
	Use:     "pull [REFERENCE]",
	Short:   "Pull a Zarf bundle from a remote registry and save to the local file system",
	Example: "  zarf bundle pull oci://my-registry.com/my-namespace/my-bundle:0.0.1-arm64",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if !utils.IsOCIURL(args[0]) {
			message.Fatalf(nil, "Registry must be prefixed with 'oci://'")
		}
		bndlConfig.BndlSource = chooseBundle(args)

		// Configure the bundler
		bndlClient := bundler.NewOrDie(&bndlConfig)
		defer bndlClient.ClearTempPaths()

		// Pull the bundle
		if err := bndlClient.Pull(); err != nil {
			message.Fatalf(err, "Failed to pull bundle: %s", err.Error())
		}
	},
}

func chooseBundle(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	var path string
	prompt := &survey.Input{
		Message: "Choose or type the bundle file",
		Suggest: func(toComplete string) []string {
			files, _ := filepath.Glob(config.ZarfBundlePrefix + toComplete + "*.tar")
			gzFiles, _ := filepath.Glob(config.ZarfBundlePrefix + toComplete + "*.tar.zst")
			partialFiles, _ := filepath.Glob(config.ZarfBundlePrefix + toComplete + "*.part000")

			files = append(files, gzFiles...)
			files = append(files, partialFiles...)
			return files
		},
	}

	if err := survey.AskOne(prompt, &path, survey.WithValidator(survey.Required)); err != nil {
		message.Fatalf(nil, "Bundle path selection canceled: %s", err.Error())
	}

	return path
}

func init() {
	initViper()

	rootCmd.AddCommand(bundleCmd)
	bundleCmd.AddCommand(bundleCreateCmd)
	bundleCmd.AddCommand(bundleDeployCmd)
	bundleCmd.AddCommand(bundleInspectCmd)
	bundleCmd.AddCommand(bundleRemoveCmd)
	bundleCmd.AddCommand(bundlePullCmd)
}
