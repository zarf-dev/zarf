// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/defenseunicorns/zarf/src/cmd/tools"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils/exec"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	logLevel string

	// Default global config for the packager
	pkgConfig = types.PackagerConfig{}

	// Viper instance used by the cmd package
	v *viper.Viper

	// holds any error from reading in Viper config
	vConfigError error
)

var rootCmd = &cobra.Command{
	Use: "zarf COMMAND",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Skip for vendor-only commands
		if tools.CheckVendorOnlyFromPath(cmd) {
			return
		}

		exec.ExitOnInterrupt()

		// Don't log the help command
		if cmd.Parent() == nil {
			config.SkipLogFile = true
		}
		cliSetup()
	},
	Short: lang.RootCmdShort,
	Long:  lang.RootCmdLong,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		zarfLogo := message.GetLogo()
		_, _ = fmt.Fprintln(os.Stderr, zarfLogo)
		cmd.Help()

		if len(args) > 0 {
			if strings.Contains(args[0], config.ZarfPackagePrefix) || strings.Contains(args[0], "zarf-init") {
				pterm.FgYellow.Printfln("\n"+lang.RootCmdDeprecatedDeploy, args[0])
			}
			if args[0] == config.ZarfYAML {
				pterm.FgYellow.Printfln("\n" + lang.RootCmdDeprecatedCreate)
			}
		}
	},
}

// Execute is the entrypoint for the CLI.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

// RootCmd returns the root command.
func RootCmd() *cobra.Command {
	return rootCmd
}

func init() {
	// Add the tools commands
	tools.Include(rootCmd)

	// Skip for vendor-only commands
	if tools.CheckVendorOnlyFromArgs() {
		return
	}

	initViper()

	v.SetDefault(V_LOG_LEVEL, "info")
	v.SetDefault(V_ARCHITECTURE, "")
	v.SetDefault(V_NO_LOG_FILE, false)
	v.SetDefault(V_NO_PROGRESS, false)
	v.SetDefault(V_NO_COLOR, false)
	v.SetDefault(V_INSECURE, false)
	v.SetDefault(V_ZARF_CACHE, config.ZarfDefaultCachePath)
	v.SetDefault(V_TMP_DIR, "")

	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "l", v.GetString(V_LOG_LEVEL), lang.RootCmdFlagLogLevel)
	rootCmd.PersistentFlags().StringVarP(&config.CLIArch, "architecture", "a", v.GetString(V_ARCHITECTURE), lang.RootCmdFlagArch)
	rootCmd.PersistentFlags().BoolVar(&config.SkipLogFile, "no-log-file", v.GetBool(V_NO_LOG_FILE), lang.RootCmdFlagSkipLogFile)
	rootCmd.PersistentFlags().BoolVar(&message.NoProgress, "no-progress", v.GetBool(V_NO_PROGRESS), lang.RootCmdFlagNoProgress)
	rootCmd.PersistentFlags().BoolVar(&config.NoColor, "no-color", v.GetBool(V_NO_COLOR), lang.RootCmdFlagNoColor)
	rootCmd.PersistentFlags().StringVar(&config.CommonOptions.CachePath, "zarf-cache", v.GetString(V_ZARF_CACHE), lang.RootCmdFlagCachePath)
	rootCmd.PersistentFlags().StringVar(&config.CommonOptions.TempDirectory, "tmpdir", v.GetString(V_TMP_DIR), lang.RootCmdFlagTempDir)
	rootCmd.PersistentFlags().BoolVar(&config.CommonOptions.Insecure, "insecure", v.GetBool(V_INSECURE), lang.RootCmdFlagInsecure)
}

func cliSetup() {
	match := map[string]message.LogLevel{
		"warn":  message.WarnLevel,
		"info":  message.InfoLevel,
		"debug": message.DebugLevel,
		"trace": message.TraceLevel,
	}

	if config.NoColor {
		message.DisableColor()
	}

	printViperConfigUsed()

	// No log level set, so use the default
	if logLevel != "" {
		if lvl, ok := match[logLevel]; ok {
			message.SetLogLevel(lvl)
			message.Debug("Log level set to " + logLevel)
		} else {
			message.Warn(lang.RootCmdErrInvalidLogLevel)
		}
	}

	// Disable progress bars for CI envs
	if os.Getenv("CI") == "true" {
		message.Debug("CI environment detected, disabling progress bars")
		message.NoProgress = true
	}

	if !config.SkipLogFile {
		message.UseLogFile()
	}
}
