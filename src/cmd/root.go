// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/zarf-dev/zarf/src/cmd/common"
	"github.com/zarf-dev/zarf/src/cmd/tools"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/layout"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/types"
)

var (
	// Default global config for the packager
	pkgConfig = types.PackagerConfig{}
	// LogLevelCLI holds the log level as input from a command
	LogLevelCLI string
	// SkipLogFile is a flag to skip logging to a file
	SkipLogFile bool
	// NoColor is a flag to disable colors in output
	NoColor bool
)

var rootCmd = &cobra.Command{
	Use: "zarf COMMAND",
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		// If --insecure was provided, set --insecure-skip-tls-verify and --plain-http to match
		if config.CommonOptions.Insecure {
			config.CommonOptions.InsecureSkipTLSVerify = true
			config.CommonOptions.PlainHTTP = true
		}

		// Skip for vendor only commands
		if common.CheckVendorOnlyFromPath(cmd) {
			return nil
		}

		skipLogFile := SkipLogFile

		// Dont write tool commands to file.
		comps := strings.Split(cmd.CommandPath(), " ")
		if len(comps) > 1 && comps[1] == "tools" {
			skipLogFile = true
		}
		if len(comps) > 1 && comps[1] == "version" {
			skipLogFile = true
		}

		// Dont write help command to file.
		if cmd.Parent() == nil {
			skipLogFile = true
		}

		err := common.SetupCLI(LogLevelCLI, skipLogFile, NoColor)
		if err != nil {
			return err
		}
		return nil
	},
	Short:         lang.RootCmdShort,
	Long:          lang.RootCmdLong,
	Args:          cobra.MaximumNArgs(1),
	SilenceUsage:  true,
	SilenceErrors: true,
	Run: func(cmd *cobra.Command, args []string) {
		zarfLogo := message.GetLogo()
		_, _ = fmt.Fprintln(os.Stderr, zarfLogo)
		err := cmd.Help()
		if err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
		}

		if len(args) > 0 {
			if strings.Contains(args[0], config.ZarfPackagePrefix) || strings.Contains(args[0], "zarf-init") {
				message.Warnf(lang.RootCmdDeprecatedDeploy, args[0])
			}
			if args[0] == layout.ZarfYAML {
				message.Warn(lang.RootCmdDeprecatedCreate)
			}
		}
	},
}

// Execute is the entrypoint for the CLI.
func Execute(ctx context.Context) {
	cmd, err := rootCmd.ExecuteContextC(ctx)
	if err == nil {
		return
	}
	defaultPrintCmds := []string{"helm", "yq", "kubectl"}
	comps := strings.Split(cmd.CommandPath(), " ")
	if len(comps) > 1 && comps[1] == "tools" && slices.Contains(defaultPrintCmds, comps[2]) {
		cmd.PrintErrln(cmd.ErrPrefix(), err.Error())
	} else {
		errParagraph := message.Paragraph(err.Error())
		pterm.Error.Println(errParagraph)
	}
	os.Exit(1)
}

func init() {
	// Add the tools commands
	tools.Include(rootCmd)

	// Skip for vendor-only commands
	if common.CheckVendorOnlyFromArgs() {
		return
	}

	v := common.InitViper()

	rootCmd.PersistentFlags().StringVarP(&LogLevelCLI, "log-level", "l", v.GetString(common.VLogLevel), lang.RootCmdFlagLogLevel)
	rootCmd.PersistentFlags().StringVarP(&config.CLIArch, "architecture", "a", v.GetString(common.VArchitecture), lang.RootCmdFlagArch)
	rootCmd.PersistentFlags().BoolVar(&SkipLogFile, "no-log-file", v.GetBool(common.VNoLogFile), lang.RootCmdFlagSkipLogFile)
	rootCmd.PersistentFlags().BoolVar(&NoColor, "no-color", v.GetBool(common.VNoColor), lang.RootCmdFlagNoColor)
	rootCmd.PersistentFlags().StringVar(&config.CommonOptions.CachePath, "zarf-cache", v.GetString(common.VZarfCache), lang.RootCmdFlagCachePath)
	rootCmd.PersistentFlags().StringVar(&config.CommonOptions.TempDirectory, "tmpdir", v.GetString(common.VTmpDir), lang.RootCmdFlagTempDir)
	rootCmd.PersistentFlags().BoolVar(&config.CommonOptions.Insecure, "insecure", v.GetBool(common.VInsecure), lang.RootCmdFlagInsecure)
	rootCmd.PersistentFlags().MarkDeprecated("insecure", "please use --plain-http, --insecure-skip-tls-verify, or --skip-signature-validation instead.")
	rootCmd.PersistentFlags().BoolVar(&config.CommonOptions.PlainHTTP, "plain-http", v.GetBool(common.VPlainHTTP), lang.RootCmdFlagPlainHTTP)
	rootCmd.PersistentFlags().BoolVar(&config.CommonOptions.InsecureSkipTLSVerify, "insecure-skip-tls-verify", v.GetBool(common.VInsecureSkipTLSVerify), lang.RootCmdFlagInsecureSkipTLSVerify)
}
