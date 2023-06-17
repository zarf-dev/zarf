// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"fmt"

	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/bundler"
	"github.com/defenseunicorns/zarf/src/pkg/message"

	"github.com/defenseunicorns/zarf/src/pkg/utils"
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
	Args:    cobra.ExactArgs(1),
	Short:   lang.CmdBundleCreateShort,
	Long:    lang.CmdBundleCreateLong,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if utils.InvalidPath(args[0]) || !utils.IsDir(args[0]) {
			return fmt.Errorf("first argument must be a valid path to a directory")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		bndlConfig.CreateOpts.SourceDirectory = args[0]

		bndlConfig.CreateOpts.SetVariables = bundler.MergeVariables(v.GetStringMapString(V_PKG_CREATE_SET), bndlConfig.CreateOpts.SetVariables)

		// Configure the bundler
		bndlClient := bundler.NewOrDie(&bndlConfig)
		defer bndlClient.ClearPaths()

		// Create the bundle
		if err := bndlClient.Create(); err != nil {
			message.Fatalf(err, "Failed to create bundle: %s", err.Error())
		}
	},
}

var bundleDeployCmd = &cobra.Command{
	Use:     "deploy [PACKAGE]",
	Aliases: []string{"d"},
	Short:   lang.CmdBundleDeployShort,
	Long:    lang.CmdBundleDeployLong,
	Args:    cobra.ExactArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if !utils.IsOCIURL(args[0]) || utils.InvalidPath(args[0]) {
			return fmt.Errorf("first argument must either be a valid OCI URL or a valid path to a bundle tarball")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		bndlConfig.DeployOpts.Source = args[0]

		bndlConfig.DeployOpts.SetVariables = bundler.MergeVariables(v.GetStringMapString(V_PKG_DEPLOY_SET), bndlConfig.DeployOpts.SetVariables)

		// Configure the bundler
		bndlClient := bundler.NewOrDie(&bndlConfig)
		defer bndlClient.ClearPaths()

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
	Args:    cobra.ExactArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if !utils.IsOCIURL(args[0]) || utils.InvalidPath(args[0]) {
			return fmt.Errorf("first argument must either be a valid OCI URL or a valid path to a bundle tarball")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		bndlConfig.InspectOpts.Source = args[0]

		// Configure the bundler
		bndlClient := bundler.NewOrDie(&bndlConfig)
		defer bndlClient.ClearPaths()

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
	RunE: func(cmd *cobra.Command, args []string) error {
		if !utils.IsOCIURL(args[0]) || utils.InvalidPath(args[0]) {
			return fmt.Errorf("first argument must either be a valid OCI URL or a valid path to a bundle tarball")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		bndlConfig.RemoveOpts.Source = args[0]

		// Configure the bundler
		bndlClient := bundler.NewOrDie(&bndlConfig)
		defer bndlClient.ClearPaths()

		if err := bndlClient.Remove(); err != nil {
			message.Fatalf(err, "Unable to remove the bundle with an error of: %#v", err)
		}
	},
}

var bundlePullCmd = &cobra.Command{
	Use:     "pull [REFERENCE]",
	Short:   "Pull a Zarf bundle from a remote registry and save to the local file system",
	Example: "  zarf bundle pull oci://my-registry.com/my-namespace/my-bundle:0.0.1-arm64",
	Args:    cobra.ExactArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if !utils.IsOCIURL(args[0]) {
			return fmt.Errorf("invalid 'oci://...' reference: %s", args[0])
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		bndlConfig.PullOpts.Source = args[0]

		// Configure the bundler
		bndlClient := bundler.NewOrDie(&bndlConfig)
		defer bndlClient.ClearPaths()

		// Pull the bundle
		if err := bndlClient.Pull(); err != nil {
			message.Fatalf(err, "Failed to pull bundle: %s", err.Error())
		}
	},
}

func init() {
	rootCmd.AddCommand(bundleCmd)
	bundleCmd.AddCommand(bundleCreateCmd)
	bundleCmd.AddCommand(bundleDeployCmd)
	bundleCmd.AddCommand(bundleInspectCmd)
	bundleCmd.AddCommand(bundleRemoveCmd)
	bundleCmd.AddCommand(bundlePullCmd)
}
