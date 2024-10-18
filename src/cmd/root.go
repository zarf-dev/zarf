// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/zarf-dev/zarf/src/cmd/say"
	"github.com/zarf-dev/zarf/src/pkg/logger"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/zarf-dev/zarf/src/cmd/common"
	"github.com/zarf-dev/zarf/src/cmd/tools"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/types"
)

var (
	// Default global config for the packager
	pkgConfig = types.PackagerConfig{}
	// LogLevelCLI holds the log level as input from a command
	LogLevelCLI string
	// LogFormat holds the log format as input from a command
	LogFormat string
	// SkipLogFile is a flag to skip logging to a file
	SkipLogFile bool
	// NoColor is a flag to disable colors in output
	NoColor bool
)

var rootCmd = &cobra.Command{
	Use:          "zarf COMMAND",
	Short:        lang.RootCmdShort,
	Long:         lang.RootCmdLong,
	Args:         cobra.MaximumNArgs(1),
	SilenceUsage: true,
	// TODO(mkcp): Do we actually want to silence errors here?
	SilenceErrors:     true,
	PersistentPreRunE: preRun,
	Run:               run,
}

func preRun(cmd *cobra.Command, _ []string) error {
	// If --insecure was provided, set --insecure-skip-tls-verify and --plain-http to match
	if config.CommonOptions.Insecure {
		config.CommonOptions.InsecureSkipTLSVerify = true
		config.CommonOptions.PlainHTTP = true
	}

	// Skip for vendor only commands
	if common.CheckVendorOnlyFromPath(cmd) {
		return nil
	}

	// Setup message
	skipLogFile := SkipLogFile

	// Don't write tool commands to file.
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
	err := setupMessage(LogLevelCLI, skipLogFile, NoColor)
	if err != nil {
		return err
	}

	// Configure logger and add it to cmd context.
	l, err := setupLogger(LogLevelCLI, LogFormat)
	if err != nil {
		return err
	}
	ctx := context.WithValue(cmd.Context(), logger.DefaultCtxKey, l)
	cmd.SetContext(ctx)

	// Print out config location
	common.PrintViperConfigUsed(cmd.Context())
	return nil
}

func run(cmd *cobra.Command, _ []string) {
	err := cmd.Help()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
	}
}

// Execute is the entrypoint for the CLI.
func Execute(ctx context.Context) {
	// Add `zarf say`
	rootCmd.AddCommand(say.Command())

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

	// Logs
	rootCmd.PersistentFlags().StringVarP(&LogLevelCLI, "log-level", "l", v.GetString(common.VLogLevel), lang.RootCmdFlagLogLevel)
	rootCmd.PersistentFlags().StringVar(&LogFormat, "log-format", v.GetString(common.VLogFormat), lang.RootCmdFlagLogFormat)
	rootCmd.PersistentFlags().BoolVar(&SkipLogFile, "no-log-file", v.GetBool(common.VNoLogFile), lang.RootCmdFlagSkipLogFile)
	rootCmd.PersistentFlags().BoolVar(&message.NoProgress, "no-progress", v.GetBool(common.VNoProgress), lang.RootCmdFlagNoProgress)
	rootCmd.PersistentFlags().BoolVar(&NoColor, "no-color", v.GetBool(common.VNoColor), lang.RootCmdFlagNoColor)

	rootCmd.PersistentFlags().StringVarP(&config.CLIArch, "architecture", "a", v.GetString(common.VArchitecture), lang.RootCmdFlagArch)
	rootCmd.PersistentFlags().StringVar(&config.CommonOptions.CachePath, "zarf-cache", v.GetString(common.VZarfCache), lang.RootCmdFlagCachePath)
	rootCmd.PersistentFlags().StringVar(&config.CommonOptions.TempDirectory, "tmpdir", v.GetString(common.VTmpDir), lang.RootCmdFlagTempDir)

	// Security
	rootCmd.PersistentFlags().BoolVar(&config.CommonOptions.Insecure, "insecure", v.GetBool(common.VInsecure), lang.RootCmdFlagInsecure)
	rootCmd.PersistentFlags().MarkDeprecated("insecure", "please use --plain-http, --insecure-skip-tls-verify, or --skip-signature-validation instead.")
	rootCmd.PersistentFlags().BoolVar(&config.CommonOptions.PlainHTTP, "plain-http", v.GetBool(common.VPlainHTTP), lang.RootCmdFlagPlainHTTP)
	rootCmd.PersistentFlags().BoolVar(&config.CommonOptions.InsecureSkipTLSVerify, "insecure-skip-tls-verify", v.GetBool(common.VInsecureSkipTLSVerify), lang.RootCmdFlagInsecureSkipTLSVerify)
}

// setup Logger handles creating a logger and setting it as the global default.
func setupLogger(level, format string) (*slog.Logger, error) {
	sLevel, err := logger.ParseLevel(level)
	if err != nil {
		return nil, err
	}
	l, err := logger.New(logger.Config{
		Level:       sLevel,
		Format:      logger.Format(format),
		Destination: logger.DestinationDefault,
	})
	if err != nil {
		return nil, err
	}
	logger.SetDefault(l)
	l.Debug("Logger successfully initialized", "logger", l)
	return l, nil
}

// setupMessage configures message while we migrate over to logger.
func setupMessage(logLevel string, skipLogFile, noColor bool) error {
	// TODO(mkcp): Delete no-color
	if noColor {
		message.DisableColor()
	}

	if logLevel != "" {
		match := map[string]message.LogLevel{
			"warn":  message.WarnLevel,
			"info":  message.InfoLevel,
			"debug": message.DebugLevel,
			"trace": message.TraceLevel,
		}
		lvl, ok := match[logLevel]
		if !ok {
			return errors.New("invalid log level, valid options are warn, info, debug, and trace")
		}
		message.SetLogLevel(lvl)
		message.Debug("Log level set to " + logLevel)
	}

	// Disable progress bars for CI envs
	if os.Getenv("CI") == "true" {
		message.Debug("CI environment detected, disabling progress bars")
		message.NoProgress = true
	}

	if !skipLogFile {
		ts := time.Now().Format("2006-01-02-15-04-05")
		f, err := os.CreateTemp("", fmt.Sprintf("zarf-%s-*.log", ts))
		if err != nil {
			return fmt.Errorf("could not create a log file in a the temporary directory: %w", err)
		}
		logFile, err := message.UseLogFile(f)
		if err != nil {
			return fmt.Errorf("could not save a log file to the temporary directory: %w", err)
		}
		pterm.SetDefaultOutput(io.MultiWriter(os.Stderr, logFile))
		message.Notef("Saving log file to %s", f.Name())
	}
	return nil
}
