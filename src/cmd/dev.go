// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"strings"

	"github.com/defenseunicorns/zarf/src/cmd/common"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/packager"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var devCmd = &cobra.Command{
	Use:     "dev",
	Aliases: []string{"prepare"},
	Short:   lang.CmdDevShort,
}

var devDeployCmd = &cobra.Command{
	Use:  "deploy",
	Args: cobra.ExactArgs(1),
	Short: lang.CmdDevDeployShort,
	Run: func(cmd *cobra.Command, args []string) {
		pkgConfig.CreateOpts.BaseDir = args[0]

		v := common.GetViper()
		pkgConfig.CreateOpts.SetVariables = helpers.TransformAndMergeMap(
			v.GetStringMapString(common.VPkgCreateSet), pkgConfig.CreateOpts.SetVariables, strings.ToUpper)

		pkgConfig.PkgOpts.SetVariables = helpers.TransformAndMergeMap(
			v.GetStringMapString(common.VPkgDeploySet), pkgConfig.PkgOpts.SetVariables, strings.ToUpper)

		// Configure the packager
		pkgClient := packager.NewOrDie(&pkgConfig)
		defer pkgClient.ClearTempPaths()

		// Create the package
		if err := pkgClient.DevDeploy(); err != nil {
			message.Fatalf(err, lang.CmdDevDeployErr, err.Error())
		}
	},
}

func init() {
	v := common.GetViper()
	rootCmd.AddCommand(devCmd)

	devCmd.AddCommand(devDeployCmd)
	bindDevDeployFlags(v)
}

func bindDevDeployFlags(v *viper.Viper) {
	devDeployFlags := devDeployCmd.Flags()

	devDeployFlags.StringToStringVar(&pkgConfig.CreateOpts.SetVariables, "create-set", v.GetStringMapString(common.VPkgCreateSet), lang.CmdPackageCreateFlagSet)
	devDeployFlags.StringToStringVar(&pkgConfig.PkgOpts.SetVariables, "deploy-set", v.GetStringMapString(common.VPkgDeploySet), lang.CmdPackageDeployFlagSet)

	devDeployFlags.StringVar(&pkgConfig.PkgOpts.OptionalComponents, "components", v.GetString(common.VPkgDeployComponents), lang.CmdPackageDeployFlagComponents)

	devDeployFlags.BoolVar(&pkgConfig.CreateOpts.IsYOLO, "yolo", v.GetBool(common.VDevDeployYolo), lang.CmdDevDeployFlagYolo)
}
