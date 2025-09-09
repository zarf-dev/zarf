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
	"strconv"
	"strings"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/internal/feature"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/types"
)

var (
	// Default global config for the packager
	pkgConfig = types.PackagerConfig{}
	// Features is a string map of feature names to enabled state.
	// Example: "foo=true,bar=false,baz=true"
	features map[string]string
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
	// Configure user defined Features
	err := setupFeatures(features)
	if err != nil {
		return err
	}
	// Implement "axolotl-mode"
	if feature.IsEnabled(feature.AxolotlMode) {
		if _, err = fmt.Fprintln(os.Stderr, logo()); err != nil {
			return err
		}
	}

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

	// Print enabled features once we have a logger available
	l.Debug("User-configured features:", "features", flattenUserFeatures())

	// if --no-color is set, disable PTerm color in message prints
	if IsColorDisabled {
		pterm.DisableColor()
	}

	// Print out config location
	err = PrintViperConfigUsed(cmd.Context())
	if err != nil {
		return err
	}

	l.Debug("using temporary directory", "tmpDir", config.CommonOptions.TempDirectory)
	return nil
}

func setupFeatures(m map[string]string) error {
	fs, err := mapToFeatures(m)
	if err != nil {
		return err
	}

	err = feature.Set(fs)
	if err != nil {
		return err
	}

	return nil
}

// parseFeatures take an unstructured string from a viper source (cli flag, env var, disk config) and parses it into
// feature.Feature structs.
func mapToFeatures(m map[string]string) ([]feature.Feature, error) {
	// No features given, exit
	if len(m) == 0 {
		return []feature.Feature{}, nil
	}

	s := make([]feature.Feature, 0)

	// Handle pairs
	for k, v := range m {
		// Parse value into bool
		b, err := strconv.ParseBool(v)
		if err != nil {
			return []feature.Feature{}, fmt.Errorf("unable to parse feature value: %s into bool for key: %s", v, k)
		}

		// Append to feature set
		s = append(s, feature.Feature{
			Name:    feature.Name(k),
			Enabled: feature.Enabled(b),
		})
	}

	return s, nil
}

func flattenUserFeatures() string {
	fs := feature.AllUser()
	ss := make([]string, 0)
	for _, f := range fs {
		ss = append(ss, f.String())
	}
	return strings.Join(ss, ",")
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

	// NOTE(mkcp): This line must be run with the unconfigured default logger because user flags are set downstream
	// in rootCmd's preRun func.
	logger.Default().Error(err.Error())
	os.Exit(1)
}

func init() {
	var showNoProgressDeprecation bool
	// Skip for vendor-only commands
	if checkVendorOnlyFromArgs() {
		return
	}

	vpr := getViper()

	// Features
	rootCmd.PersistentFlags().StringToStringVar(&features, "features", vpr.GetStringMapString(VFeatures), "[ALPHA] Provide a comma-separated list of feature names to bools to enable or disable. Ex. --features \"foo=true,bar=false,baz=true\"")

	// Logs
	rootCmd.PersistentFlags().StringVarP(&LogLevelCLI, "log-level", "l", vpr.GetString(VLogLevel), lang.RootCmdFlagLogLevel)
	rootCmd.PersistentFlags().StringVar(&LogFormat, "log-format", vpr.GetString(VLogFormat), "Select a logging format. Defaults to 'console'. Valid options are: 'console', 'json', 'dev'.")
	rootCmd.PersistentFlags().BoolVar(&IsColorDisabled, "no-color", vpr.GetBool(VNoColor), "Disable terminal color codes in logging and stdout prints.")
	rootCmd.PersistentFlags().BoolVar(&showNoProgressDeprecation, "no-progress", v.GetBool("no_progress"), "Disable fancy UI progress bars, spinners, logos, etc")
	_ = rootCmd.PersistentFlags().MarkDeprecated("no-progress", "Progress bars and spinners were removed with --log-format=legacy, this flag will be removed in a future version of Zarf.")

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
