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
	"github.com/spf13/viper"
	"helm.sh/helm/v4/pkg/kube"

	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/feature"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/ocischeme"
)

var (
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

var rootCmd, rootCmdError = newZarfCommand()

func preRun(cmd *cobra.Command, _ []string) error {
	// This ensures the field manager is set to Zarf during any Helm SDK actions
	kube.ManagedFieldsManager = cluster.FieldManagerName
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
	// One negotiator per CLI invocation; decisions never expire (TTL 0) since Zarf is a short-lived process
	ctx = ocischeme.WithNegotiator(ctx, ocischeme.New(ocischeme.Options{}))
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
	// Library callers can supply their own arguments with SetArgs; startup
	// errors from the host process are only relevant to Execute.
	cmd, _ := newZarfCommand() //nolint:errcheck // Cobra validates library callers' supplied arguments.
	return cmd
}

func newZarfCommand() (*cobra.Command, error) {
	rootCmd := &cobra.Command{
		Use:          "zarf COMMAND",
		Short:        lang.RootCmdShort,
		Long:         lang.RootCmdLong,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		// We use silence errors so we can print out errors in the Zarf log format
		SilenceErrors:     true,
		PersistentPreRunE: preRun,
		Run:               run,
	}

	invocation, err := parseVendorInvocation(os.Args, vendorCmds)
	if invocation != nil {
		var options rootOptions
		flags := newRootFlagSet(getViper(), &options)
		help := flags.BoolP("help", "h", false, "help for zarf")
		err = flags.Parse(invocation.flags)
		if err == nil {
			options.apply()
			_, err = setupLogger(options.logLevel, options.logFormat, !options.noColor)
			if options.noColor {
				pterm.DisableColor()
			}
		}
		if err == nil {
			if *help {
				invocation.args = append(invocation.args, "--help")
				invocation.toolArgs = append(invocation.toolArgs, "--help")
			}
			rootCmd.SetArgs(invocation.args)
		}
	}

	var toolArgs []string
	if invocation != nil && err == nil {
		toolArgs = invocation.toolArgs
	}

	// Add the tools commands
	// IMPORTANT: we need to make sure the tools command are added first
	// to ensure the config defaulting doesn't kick in, and inject values
	// into zart tools update-creds command
	// see https://github.com/zarf-dev/zarf/pull/3340#discussion_r1889221826
	rootCmd.AddCommand(newToolsCommand(toolArgs))

	// TODO(soltysh): consider adding command groups
	rootCmd.AddCommand(newConnectCommand())
	rootCmd.AddCommand(sayCommand())
	rootCmd.AddCommand(newDestroyCommand())
	rootCmd.AddCommand(newDevCommand())
	rootCmd.AddCommand(newInitCommand())
	rootCmd.AddCommand(newInternalCommand(rootCmd))
	rootCmd.AddCommand(newComponentCommand())
	rootCmd.AddCommand(newPackageCommand())

	rootCmd.AddCommand(newVersionCommand())

	// Setup flags - skip for vendor-only commands
	if invocation == nil || err != nil {
		setupRootFlags(rootCmd)
	}

	setupGroupedFlagUsage(rootCmd)

	return rootCmd, err
}

type rootOptions struct {
	features              map[string]string
	logLevel              string
	logFormat             string
	noColor               bool
	noProgress            bool
	architecture          string
	cachePath             string
	tmpDir                string
	plainHTTP             bool
	insecureSkipTLSVerify bool
}

func (o *rootOptions) apply() {
	features = o.features
	LogLevelCLI = o.logLevel
	LogFormat = o.logFormat
	IsColorDisabled = o.noColor
	config.CLIArch = o.architecture
	config.CommonOptions.CachePath = o.cachePath
	config.CommonOptions.TempDirectory = o.tmpDir
	plainHTTP = o.plainHTTP
	insecureSkipTLSVerify = o.insecureSkipTLSVerify
}

// setupRootFlags sets up the persistent flags for the root command.
func setupRootFlags(rootCmd *cobra.Command) {
	var options rootOptions
	rootCmd.PersistentFlags().AddFlagSet(newRootFlagSet(getViper(), &options))
	options.apply()
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		options.apply()
		return preRun(cmd, args)
	}
}

// newRootFlagSet also supplies the definitions used to find embedded tool arguments,
// without loading configuration or changing global options.
func newRootFlagSet(vpr *viper.Viper, o *rootOptions) *pflag.FlagSet {
	flags := pflag.NewFlagSet("zarf", pflag.ContinueOnError)

	// Features
	flags.StringToStringVar(&o.features, "features", vpr.GetStringMapString(VFeatures), "Provide a comma-separated list of feature names to bools to enable or disable. Ex. --features \"foo=true,bar=false,baz=true\"")

	// Logs
	flags.StringVarP(&o.logLevel, "log-level", "l", vpr.GetString(VLogLevel), lang.RootCmdFlagLogLevel)
	flags.StringVar(&o.logFormat, "log-format", vpr.GetString(VLogFormat), "Select a logging format. Defaults to 'console'. Valid options are: 'console', 'json', 'dev'.")
	flags.BoolVar(&o.noColor, "no-color", vpr.GetBool(VNoColor), "Disable terminal color codes in logging and stdout prints.")
	flags.BoolVar(&o.noProgress, "no-progress", vpr.GetBool("no_progress"), "Disable fancy UI progress bars, spinners, logos, etc")
	_ = flags.MarkDeprecated("no-progress", "Progress bars and spinners were removed with --log-format=legacy, this flag will be removed in a future version of Zarf.")

	// Core functionality
	flags.StringVarP(&o.architecture, "architecture", "a", vpr.GetString(VArchitecture), lang.RootCmdFlagArch)
	cachePath := vpr.GetString(VCache)
	if cachePath == "" {
		cachePath = vpr.GetString(VZarfCache)
	}
	if cachePath == "" {
		cachePath = config.ZarfDefaultCachePath
	}
	flags.StringVar(&o.cachePath, "zarf-cache", cachePath, lang.RootCmdFlagCachePath)
	_ = flags.MarkDeprecated("zarf-cache", "use --cache instead")
	flags.StringVar(&o.cachePath, "cache", cachePath, lang.RootCmdFlagCachePath)
	flags.StringVar(&o.tmpDir, "tmpdir", vpr.GetString(VTmpDir), lang.RootCmdFlagTempDir)

	// Security
	flags.BoolVar(&o.plainHTTP, "plain-http", vpr.GetBool(VPlainHTTP), lang.RootCmdFlagPlainHTTP)
	flags.BoolVar(&o.insecureSkipTLSVerify, "insecure-skip-tls-verify", vpr.GetBool(VInsecureSkipTLSVerify), lang.RootCmdFlagInsecureSkipTLSVerify)
	return flags
}

// Execute is the entrypoint for the CLI.
func Execute(ctx context.Context) error {
	if rootCmdError != nil {
		// Prefix errors must be reported before Cobra merges the tool's flags.
		logger.Default().Error(rootCmdError.Error())
		return rootCmdError
	}
	cmd, err := rootCmd.ExecuteContextC(ctx)
	if err == nil {
		return nil
	}

	// Check if we need to use the default err printer
	defaultPrintCmds := []string{"helm", "yq", "kubectl"}
	comps := strings.Split(cmd.CommandPath(), " ")
	if len(comps) > 2 && comps[1] == "tools" && slices.Contains(defaultPrintCmds, comps[2]) {
		cmd.PrintErrln(cmd.ErrPrefix(), err.Error())
		os.Exit(1)
	}

	// Use default logger in case there was an error prior to the logger being setup
	logger.Default().Error(err.Error())
	return err
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
