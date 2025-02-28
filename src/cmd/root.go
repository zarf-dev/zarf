// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"

	"github.com/zarf-dev/zarf/src/pkg/logger"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

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
	// IsColorDisabled corresponds to the --no-color flag. It disables color codes in terminal output
	IsColorDisabled bool
	// OutputWriter provides a default writer to Stdout for user-facing command output
	OutputWriter = os.Stdout
)

type outputFormat string

const (
	outputTable outputFormat = "table"
	outputJSON  outputFormat = "json"
	outputYAML  outputFormat = "yaml"
)

// must implement this interface for cmd.Flags().VarP
var _ pflag.Value = (*outputFormat)(nil)

func (o *outputFormat) Set(s string) error {
	switch s {
	case string(outputTable), string(outputJSON), string(outputYAML):
		*o = outputFormat(s)
		return nil
	default:
		return fmt.Errorf("invalid output format: %s", s)
	}
}

func (o *outputFormat) String() string {
	return string(*o)
}

func (o *outputFormat) Type() string {
	return "outputFormat"
}

var rootCmd = NewZarfCommand()

func preRun(cmd *cobra.Command, _ []string) error {
	// If --insecure was provided, set --insecure-skip-tls-verify and --plain-http to match
	if config.CommonOptions.Insecure {
		config.CommonOptions.InsecureSkipTLSVerify = true
		config.CommonOptions.PlainHTTP = true
	}

	// Skip for vendor only commands
	if checkVendorOnlyFromPath(cmd) {
		return nil
	}

	// Configure logger and add it to cmd context. We flip NoColor because setLogger wants "isColor"
	l, err := setupLogger(LogLevelCLI, LogFormat, !IsColorDisabled)
	if err != nil {
		return err
	}
	ctx := logger.WithContext(cmd.Context(), l)
	cmd.SetContext(ctx)

	// if --no-color is set, disable PTerm color in message prints
	if IsColorDisabled {
		pterm.DisableColor()
	}

	// Print out config location
	err = PrintViperConfigUsed(cmd.Context())
	if err != nil {
		return err
	}
	return nil
}

func run(cmd *cobra.Command, _ []string) {
	err := cmd.Help()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
	}
}

// NewZarfCommand creates the `zarf` command and its nested children.
func NewZarfCommand() *cobra.Command {
	rootCmd := &cobra.Command{
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

	// Add the tools commands
	// IMPORTANT: we need to make sure the tools command are added first
	// to ensure the config defaulting doesn't kick in, and inject values
	// into zart tools update-creds command
	// see https://github.com/zarf-dev/zarf/pull/3340#discussion_r1889221826
	rootCmd.AddCommand(newToolsCommand())

	// TODO(soltysh): consider adding command groups
	rootCmd.AddCommand(newConnectCommand())
	rootCmd.AddCommand(sayCommand())
	rootCmd.AddCommand(newDestroyCommand())
	rootCmd.AddCommand(newDevCommand())
	rootCmd.AddCommand(newInitCommand())
	rootCmd.AddCommand(newInternalCommand(rootCmd))
	rootCmd.AddCommand(newPackageCommand())

	rootCmd.AddCommand(newVersionCommand())

	return rootCmd
}

// Execute is the entrypoint for the CLI.
func Execute(ctx context.Context) {
	cmd, err := rootCmd.ExecuteContextC(ctx)
	if err == nil {
		return
	}

	// Check if we need to use the default err printer
	defaultPrintCmds := []string{"helm", "yq", "kubectl"}
	comps := strings.Split(cmd.CommandPath(), " ")
	if len(comps) > 1 && comps[1] == "tools" && slices.Contains(defaultPrintCmds, comps[2]) {
		cmd.PrintErrln(cmd.ErrPrefix(), err.Error())
		os.Exit(1)
	}

	// TODO(mkcp): Remove message on logger release
	errParagraph := message.Paragraph(err.Error())
	pterm.Error.Println(errParagraph)

	// NOTE(mkcp): The default logger is set with user flags downstream in rootCmd's preRun func, so we don't have
	// access to it on Execute's ctx.
	logger.Default().Error(err.Error())
	os.Exit(1)
}

func init() {
	// Skip for vendor-only commands
	if checkVendorOnlyFromArgs() {
		return
	}

	vpr := getViper()

	// Logs
	rootCmd.PersistentFlags().StringVarP(&LogLevelCLI, "log-level", "l", vpr.GetString(VLogLevel), lang.RootCmdFlagLogLevel)
	rootCmd.PersistentFlags().StringVar(&LogFormat, "log-format", vpr.GetString(VLogFormat), "Select a logging format. Defaults to 'console'. Valid options are: 'console', 'json', 'dev'.")
	rootCmd.PersistentFlags().BoolVar(&IsColorDisabled, "no-color", vpr.GetBool(VNoColor), "Disable terminal color codes in logging and stdout prints.")

	// Core functionality
	rootCmd.PersistentFlags().StringVarP(&config.CLIArch, "architecture", "a", vpr.GetString(VArchitecture), lang.RootCmdFlagArch)
	rootCmd.PersistentFlags().StringVar(&config.CommonOptions.CachePath, "zarf-cache", vpr.GetString(VZarfCache), lang.RootCmdFlagCachePath)
	rootCmd.PersistentFlags().StringVar(&config.CommonOptions.TempDirectory, "tmpdir", vpr.GetString(VTmpDir), lang.RootCmdFlagTempDir)

	// Security
	rootCmd.PersistentFlags().BoolVar(&config.CommonOptions.Insecure, "insecure", vpr.GetBool(VInsecure), lang.RootCmdFlagInsecure)
	rootCmd.PersistentFlags().BoolVar(&config.CommonOptions.PlainHTTP, "plain-http", vpr.GetBool(VPlainHTTP), lang.RootCmdFlagPlainHTTP)
	rootCmd.PersistentFlags().BoolVar(&config.CommonOptions.InsecureSkipTLSVerify, "insecure-skip-tls-verify", vpr.GetBool(VInsecureSkipTLSVerify), lang.RootCmdFlagInsecureSkipTLSVerify)
	_ = rootCmd.PersistentFlags().MarkDeprecated("insecure", "please use --plain-http, --insecure-skip-tls-verify, or --skip-signature-validation instead.")
}

// setupLogger handles creating a logger and setting it as the global default.
func setupLogger(level, format string, isColor bool) (*slog.Logger, error) {
	// If we didn't get a level from config, fallback to "info"
	if level == "" {
		level = "info"
	}
	sLevel, err := logger.ParseLevel(level)
	if err != nil {
		return nil, err
	}
	cfg := logger.Config{
		Level:       sLevel,
		Format:      logger.Format(format),
		Destination: logger.DestinationDefault,
		Color:       logger.Color(isColor),
	}
	l, err := logger.New(cfg)
	if err != nil {
		return nil, err
	}
	logger.SetDefault(l)
	l.Debug("logger successfully initialized", "cfg", cfg)
	return l, nil
}
