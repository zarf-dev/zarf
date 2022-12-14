// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for zarf
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	skipLogFile bool
	logLevel    string
	arch        string

	// Default global config for the CLI
	pkgConfig = types.PackagerConfig{}

	// Viper instance used by the cmd package
	v *viper.Viper
)

var rootCmd = &cobra.Command{
	Use: "zarf [COMMAND]",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Don't add the logo to the help command
		if cmd.Parent() == nil {
			skipLogFile = true
		}
		cliSetup()
	},
	Short: lang.RootCmdShort,
	Long:  lang.RootCmdLong,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) > 0 {
			pterm.Println()
			if strings.Contains(args[0], config.ZarfPackagePrefix) || strings.Contains(args[0], "zarf-init") {
				pterm.FgYellow.Printfln(lang.RootCmdDeprecatedDeploy, args[0])
			}
			if args[0] == config.ZarfYAML {
				pterm.FgYellow.Printfln(lang.RootCmdDeprecatedCreate)
			}
		} else {
			zarfLogo := message.GetLogo()
			_, _ = fmt.Fprintln(os.Stderr, zarfLogo)
			cmd.Help()
		}
	},
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	initViper()

	v.SetDefault(V_LOG_LEVEL, "info")
	v.SetDefault(V_ARCHITECTURE, "")
	v.SetDefault(V_NO_LOG_FILE, false)
	v.SetDefault(V_NO_PROGRESS, false)
	v.SetDefault(V_ZARF_CACHE, config.ZarfDefaultCachePath)
	v.SetDefault(V_TMP_DIR, "")

	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "l", v.GetString(V_LOG_LEVEL), lang.RootCmdFlagLogLevel)
	rootCmd.PersistentFlags().StringVarP(&arch, "architecture", "a", v.GetString(V_ARCHITECTURE), lang.RootCmdFlagArch)
	rootCmd.PersistentFlags().BoolVar(&skipLogFile, "no-log-file", v.GetBool(V_NO_LOG_FILE), lang.RootCmdFlagSkipLogFile)
	rootCmd.PersistentFlags().BoolVar(&message.NoProgress, "no-progress", v.GetBool(V_NO_PROGRESS), lang.RootCmdFlagNoProgress)
	rootCmd.PersistentFlags().StringVar(&config.CommonOptions.CachePath, "zarf-cache", v.GetString(V_ZARF_CACHE), lang.RootCmdFlagCachePath)
	rootCmd.PersistentFlags().StringVar(&config.CommonOptions.TempDirectory, "tmpdir", v.GetString(V_TMP_DIR), lang.RootCmdFlagTempDir)
}

func cliSetup() {
	config.CliArch = arch

	match := map[string]message.LogLevel{
		"warn":  message.WarnLevel,
		"info":  message.InfoLevel,
		"debug": message.DebugLevel,
		"trace": message.TraceLevel,
	}

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

	if !skipLogFile {
		message.UseLogFile()
	}
}
