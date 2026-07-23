package clio

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/spf13/cobra"

	"github.com/anchore/fangs"
	"github.com/anchore/go-homedir"
	"github.com/anchore/go-logger/adapter/redact"
)

func ConfigCommand(app Application, opts *ConfigCommandConfig) *cobra.Command {
	if opts == nil {
		opts = DefaultConfigCommandConfig()
	}

	id := app.ID()
	internalApp := extractInternalApp(app)
	if internalApp == nil {
		return &cobra.Command{
			RunE: func(_ *cobra.Command, _ []string) error {
				return fmt.Errorf("unable to extract internal application, provided: %v", app)
			},
		}
	}

	cmd := &cobra.Command{
		Use:   "config",
		Short: fmt.Sprintf("show the %s configuration", id.Name),
		RunE: func(cmd *cobra.Command, _ []string) error {
			allConfigs := allCommandConfigs(internalApp)
			var err error
			if opts.LoadConfig {
				err = loadAllConfigs(cmd, internalApp.setupConfig.FangsConfig, allConfigs)
			}
			summary := summarizeConfig(cmd, internalApp.setupConfig.FangsConfig, opts.makeFilters(internalApp.state.RedactStore), allConfigs)
			_, writeErr := os.Stdout.WriteString(summary)
			if writeErr != nil {
				writeErr = fmt.Errorf("an error occurred writing configuration summary: %w", writeErr)
				err = errors.Join(err, writeErr)
			}
			if err != nil {
				// space before the error display
				_, _ = os.Stderr.WriteString("\n")
			}
			return err
		},
	}

	cmd.Flags().BoolVarP(&opts.LoadConfig, "load", "", opts.LoadConfig, fmt.Sprintf("load and validate the %s configuration", id.Name))

	if opts.IncludeLocationsSubcommand {
		// sub-command to print expanded configuration file search locations
		cmd.AddCommand(summarizeLocationsCommand(internalApp))
	}

	return cmd
}

type valueFilterFunc func(string) string

type ConfigCommandConfig struct {
	LoadConfig                 bool
	IncludeLocationsSubcommand bool
	ReplaceHomeDirWithTilde    bool
}

func DefaultConfigCommandConfig() *ConfigCommandConfig {
	return &ConfigCommandConfig{
		IncludeLocationsSubcommand: true,
		ReplaceHomeDirWithTilde:    true,
	}
}

// WithIncludeLocationsSubcommand true will include a `config locations` subcommand which lists each location that will
// be used to locate configuration files based on the configured environment
func (c *ConfigCommandConfig) WithIncludeLocationsSubcommand(include bool) *ConfigCommandConfig {
	c.IncludeLocationsSubcommand = include
	return c
}

// WithReplaceHomeDirWithTilde adds a value filter function which replaces matching home directory values in strings
// starting with the user's home directory to make configurations more portable. Note: this does not apply to the
// locations subcommand, only the config command itself
func (c *ConfigCommandConfig) WithReplaceHomeDirWithTilde(replace bool) *ConfigCommandConfig {
	c.ReplaceHomeDirWithTilde = replace
	return c
}

func (c *ConfigCommandConfig) makeFilters(redactStore redact.Store) (filter valueFilterFunc) {
	if redactStore != nil {
		filter = chainFilterFuncs(redactStore.RedactString, filter)
	}
	if c.ReplaceHomeDirWithTilde {
		userHome, _ := homedir.Dir()
		if userHome != "" {
			filter = chainFilterFuncs(filter, func(s string) string {
				// make any defaults based on the user's home directory more portable
				if strings.HasPrefix(s, userHome) {
					s = strings.ReplaceAll(s, userHome, "~")
				}
				return s
			})
		}
	}
	return filter
}

func chainFilterFuncs(f1, f2 valueFilterFunc) valueFilterFunc {
	if f1 == nil {
		return f2
	}
	if f2 == nil {
		return f1
	}
	return func(s string) string {
		s = f1(s)
		s = f2(s)
		return s
	}
}

func extractInternalApp(app Application) *application {
	if a, ok := app.(*application); ok {
		return a
	}
	return nil
}

func allCommandConfigs(internalApp *application) []any {
	return append([]any{&internalApp.state.Config, internalApp}, internalApp.state.Config.FromCommands...)
}

func loadAllConfigs(cmd *cobra.Command, fangsCfg fangs.Config, allConfigs []any) error {
	var errs []error
	for _, cfg := range allConfigs {
		// load each config individually, as there may be conflicting names / types that will cause
		// viper to fail to read them all and panic
		if err := fangs.Load(fangsCfg, cmd, cfg); err != nil {
			t := reflect.TypeOf(cfg)
			for t.Kind() == reflect.Pointer {
				t = t.Elem()
			}
			errs = appendConfigLoadError(errs, t, err)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("error(s) occurred loading configuration: %w", errors.Join(errs...))
}

func summarizeConfig(commandWithRootParent *cobra.Command, fangsCfg fangs.Config, redact func(string) string, allConfigs []any) string {
	summary := fangs.SummarizeCommand(fangsCfg, commandWithRootParent, redact, allConfigs...)
	summary = strings.TrimSpace(summary) + "\n"
	return summary
}

func summarizeLocationsCommand(internalApp *application) *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:   "locations",
		Short: fmt.Sprintf("shows all locations and the order in which %s will look for a configuration file", internalApp.ID().Name),
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			suffix := ".yaml"
			if all {
				suffix = ""
			}
			summary := summarizeLocations(internalApp.setupConfig.FangsConfig, suffix)
			_, err := os.Stdout.WriteString(summary)
			return err
		},
	}

	cmd.Flags().BoolVarP(&all, "all", "", all, "include every file extension supported")

	return cmd
}

func summarizeLocations(fangsCfg fangs.Config, onlySuffix string) string {
	var out strings.Builder
	for _, f := range fangs.SummarizeLocations(fangsCfg) {
		if onlySuffix != "" && !strings.HasSuffix(f, onlySuffix) {
			continue
		}
		out.WriteString(f + "\n")
	}
	return out.String()
}

// appendConfigLoadError appends errors including originating struct, but deduplicates identical errors that occur across multiple load calls
func appendConfigLoadError(errs []error, t reflect.Type, err error) []error {
	if err == nil {
		return errs
	}
	msg := err.Error()
	// remove configuration object source when we get the same error from multiple sources
	for i, e := range errs {
		// already have this error, don't append
		if e.Error() == msg {
			return errs
		}
		// if we have an identical wrapped error, this occurred when loading multiple configurations so just show the error
		if e, ok := e.(interface{ Unwrap() error }); ok {
			if e.Unwrap().Error() == msg {
				errs[i] = err
				return errs
			}
		}
	}
	return append(errs, fmt.Errorf("error loading config '%s.%s': %w", t.PkgPath(), t.Name(), err))
}
