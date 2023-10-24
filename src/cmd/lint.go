// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"os"

	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/lint"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/spf13/cobra"
)

var lintCmd = &cobra.Command{
	Use:     "lint [ DIRECTORY ]",
	Args:    cobra.MaximumNArgs(1),
	Aliases: []string{"l"},
	Short:   lang.CmdLintShort,
	Run: func(cmd *cobra.Command, args []string) {
		baseDir := ""
		if len(args) > 0 {
			baseDir = args[0]
		} else {
			var err error
			baseDir, err = os.Getwd()
			if err != nil {
				message.Fatalf(err, lang.CmdPackageCreateErr, err.Error())
			}
		}
		err := lint.ValidateZarfSchema(baseDir)
		if err != nil {
			message.Fatal(err, err.Error())
		}
	},
}

func init() {
	rootCmd.AddCommand(lintCmd)
}
