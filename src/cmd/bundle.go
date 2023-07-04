// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"fmt"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/bundler"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"

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
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if !utils.IsDir(args[0]) {
			return fmt.Errorf("first argument must be a valid path to a directory")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		bndlConfig.CreateOpts.SourceDirectory = args[0]

		bndlConfig.CreateOpts.SetVariables = bundler.MergeVariables(v.GetStringMapString(V_PKG_CREATE_SET), bndlConfig.CreateOpts.SetVariables)

		bndlClient := bundler.NewOrDie(&bndlConfig)
		defer bndlClient.ClearPaths()

		if err := bndlClient.Create(); err != nil {
			message.Fatalf(err, "Failed to create bundle: %s", err.Error())
		}
	},
}

var bundleDeployCmd = &cobra.Command{
	Use:     "deploy [BUNDLE]",
	Aliases: []string{"d"},
	Short:   lang.CmdBundleDeployShort,
	Args:    cobra.ExactArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if !utils.IsOCIURL(args[0]) && !bundler.IsValidTarballPath(args[0]) {
			return fmt.Errorf("first argument must either be a valid OCI URL or a valid path to a bundle tarball")
		}
		return oci.ValidateReference(args[0])
	},
	Run: func(cmd *cobra.Command, args []string) {
		bndlConfig.DeployOpts.Source = args[0]

		bndlConfig.DeployOpts.SetVariables = bundler.MergeVariables(v.GetStringMapString(V_PKG_DEPLOY_SET), bndlConfig.DeployOpts.SetVariables)

		bndlClient := bundler.NewOrDie(&bndlConfig)
		defer bndlClient.ClearPaths()

		if err := bndlClient.Deploy(); err != nil {
			message.Fatalf(err, "Failed to deploy bundle: %s", err.Error())
		}
	},
}

var bundleInspectCmd = &cobra.Command{
	Use:     "inspect [BUNDLE]",
	Aliases: []string{"i"},
	Short:   lang.CmdBundleInspectShort,
	Args:    cobra.ExactArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if !utils.IsOCIURL(args[0]) && !bundler.IsValidTarballPath(args[0]) {
			return fmt.Errorf("first argument must either be a valid OCI URL or a valid path to a bundle tarball")
		}
		return oci.ValidateReference(args[0])
	},
	Run: func(cmd *cobra.Command, args []string) {
		bndlConfig.InspectOpts.Source = args[0]

		bndlClient := bundler.NewOrDie(&bndlConfig)
		defer bndlClient.ClearPaths()

		if err := bndlClient.Inspect(); err != nil {
			message.Fatalf(err, "Failed to inspect bundle: %s", err.Error())
		}
	},
}

var bundleRemoveCmd = &cobra.Command{
	Use:     "remove [BUNDLE_NAME|BUNDLE_TARBALL|OCI_REF]",
	Aliases: []string{"u"},
	Args:    cobra.ExactArgs(1),
	Short:   lang.CmdBundleRemoveShort,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !utils.IsOCIURL(args[0]) && utils.InvalidPath(args[0]) {
			return fmt.Errorf("first argument must either be a valid OCI URL or a valid path to a bundle tarball")
		}
		return oci.ValidateReference(args[0])
	},
	Run: func(cmd *cobra.Command, args []string) {
		bndlConfig.RemoveOpts.Source = args[0]

		bndlClient := bundler.NewOrDie(&bndlConfig)
		defer bndlClient.ClearPaths()

		if err := bndlClient.Remove(); err != nil {
			message.Fatalf(err, "Unable to remove the bundle with an error of: %#v", err)
		}
	},
}

var bundlePullCmd = &cobra.Command{
	Use:   "pull [OCI_REF]",
	Short: lang.CmdBundlePullShort,
	Args:  cobra.ExactArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return oci.ValidateReference(args[0])
	},
	Run: func(cmd *cobra.Command, args []string) {
		bndlConfig.PullOpts.Source = args[0]

		bndlClient := bundler.NewOrDie(&bndlConfig)
		defer bndlClient.ClearPaths()

		if err := bndlClient.Pull(); err != nil {
			message.Fatalf(err, "Failed to pull bundle: %s", err.Error())
		}
	},
}

func init() {
	initViper()

	rootCmd.AddCommand(bundleCmd)
	v.SetDefault(V_BNDL_OCI_CONCURRENCY, 3)
	bundleCmd.PersistentFlags().IntVar(&config.CommonOptions.OCIConcurrency, "oci-concurrency", v.GetInt(V_BNDL_OCI_CONCURRENCY), lang.CmdBundleFlagConcurrency)

	bundleCmd.AddCommand(bundleCreateCmd)
	bundleCreateCmd.Flags().BoolVarP(&config.CommonOptions.Confirm, "confirm", "c", false, lang.CmdBundleRemoveFlagConfirm)
	bundleCreateCmd.Flags().StringVarP(&bndlConfig.CreateOpts.Output, "output", "o", v.GetString(V_BNDL_CREATE_OUTPUT), lang.CmdBundleCreateFlagOutput)
	bundleCreateCmd.Flags().StringVarP(&bndlConfig.CreateOpts.SigningKeyPath, "signing-key", "k", v.GetString(V_BNDL_CREATE_SIGNING_KEY), lang.CmdBundleCreateFlagSigningKey)
	bundleCreateCmd.Flags().StringVarP(&bndlConfig.CreateOpts.SigningKeyPassword, "signing-key-password", "p", v.GetString(V_BNDL_CREATE_SIGNING_KEY_PASSWORD), lang.CmdBundleCreateFlagSigningKeyPassword)
	bundleCreateCmd.Flags().StringToStringVarP(&bndlConfig.CreateOpts.SetVariables, "set", "s", v.GetStringMapString(V_BNDL_CREATE_SET), lang.CmdBundleCreateFlagSet)

	bundleCmd.AddCommand(bundleDeployCmd)
	bundleDeployCmd.Flags().StringSliceVarP(&bndlConfig.DeployOpts.Packages, "packages", "p", v.GetStringSlice(V_BNDL_DEPLOY_PACKAGES), lang.CmdBundleDeployFlagPackages)
	bundleDeployCmd.Flags().StringToStringVarP(&bndlConfig.DeployOpts.SetVariables, "set", "s", v.GetStringMapString(V_BNDL_DEPLOY_SET), lang.CmdBundleDeployFlagSet)

	bundleCmd.AddCommand(bundleInspectCmd)
	bundleInspectCmd.Flags().StringVarP(&bndlConfig.InspectOpts.PublicKey, "key", "k", v.GetString(V_BNDL_INSPECT_KEY), lang.CmdBundleInspectFlagKey)

	bundleCmd.AddCommand(bundleRemoveCmd)
	// confirm does not use the Viper config
	bundleRemoveCmd.Flags().BoolVarP(&config.CommonOptions.Confirm, "confirm", "c", false, lang.CmdBundleRemoveFlagConfirm)
	bundleRemoveCmd.Flags().StringSliceVarP(&bndlConfig.RemoveOpts.Packages, "packages", "p", v.GetStringSlice(V_BNDL_REMOVE_PACKAGES), lang.CmdBundleRemoveFlagPackages)
	_ = bundleRemoveCmd.MarkFlagRequired("confirm")

	bundleCmd.AddCommand(bundlePullCmd)
	bundlePullCmd.Flags().StringVarP(&bndlConfig.PullOpts.OutputDirectory, "output", "o", v.GetString(V_BNDL_PULL_OUTPUT), lang.CmdBundlePullFlagOutput)
	bundlePullCmd.Flags().StringVarP(&bndlConfig.PullOpts.PublicKey, "key", "k", v.GetString(V_BNDL_PULL_KEY), lang.CmdBundlePullFlagKey)
}
