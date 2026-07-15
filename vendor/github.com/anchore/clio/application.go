package clio

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/gookit/color"
	"github.com/pborman/indent"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.yaml.in/yaml/v3"

	"github.com/anchore/fangs"
	"github.com/anchore/go-logger"
	"github.com/anchore/go-logger/adapter/redact"
)

type Initializer func(*State) error

type PostRun func(*State, error)

type MapExitCode func(error) int

type postConstruct func(*application)

type Application interface {
	ID() Identification
	AddFlags(flags *pflag.FlagSet, cfgs ...any)
	SetupCommand(cmd *cobra.Command, cfgs ...any) *cobra.Command
	SetupRootCommand(cmd *cobra.Command, cfgs ...any) *cobra.Command
	Run()
}

type application struct {
	root            *cobra.Command
	setupConfig     SetupConfig `yaml:"-" mapstructure:"-"`
	state           State       `yaml:"-" mapstructure:"-"`
	resourcesLoaded bool
}

var _ interface {
	Application
	fangs.PostLoader
} = (*application)(nil)

func New(cfg SetupConfig) Application {
	return &application{
		setupConfig: cfg,
		state: State{
			RedactStore: redact.NewStore(),
		},
	}
}

func (a *application) ID() Identification {
	return a.setupConfig.ID
}

// State returns all application configuration and resources to be either used or replaced by the caller. Note: this is only valid after the application has been setup (cobra PreRunE has run).
func (a *application) State() *State {
	return &a.state
}

// TODO: configs of any doesn't lean into the type system enough. Consider a more specific type.

func (a *application) Setup(cfgs ...any) func(_ *cobra.Command, _ []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		// allow for the all configuration to be loaded first, then allow for the application
		// PostLoad() to run, allowing the setup of resources (logger, bus, ui, etc.) and run user initializers
		// as early as possible before the final configuration is logged. This allows for a couple of things:
		// 1. user initializers to account for taking action before logging the final configuration (such as log redactions).
		// 2. other user-facing PostLoad() functions to be able to use the logger, bus, etc. as early as possible. (though it's up to the caller on how these objects are made accessible)
		allConfigs, err := a.loadConfigs(cmd, cfgs...)
		if err != nil {
			return err
		}

		// show the app version and configuration...
		logVersion(a.setupConfig, a.state.Logger)

		logConfiguration(a.state.Logger, allConfigs...)

		return nil
	}
}

func (a *application) loadConfigs(cmd *cobra.Command, cfgs ...any) ([]any, error) {
	allConfigs := []any{
		&a.state.Config, // 1. process the core application configurations first (logging and development)
		a,               // 2. enables application.PostLoad() to be called, initializing all state (bus, logger, ui, etc.)
	}
	allConfigs = append(allConfigs, cfgs...) // 3. allow for all other configs to be loaded + call PostLoad()

	if err := fangs.Load(a.setupConfig.FangsConfig, cmd, allConfigs...); err != nil {
		return nil, fmt.Errorf("invalid application config: %v", err)
	}
	return allConfigs, nil
}

func (a *application) PostLoad() error {
	if err := a.state.setup(a.setupConfig); err != nil {
		return err
	}
	return a.runInitializers()
}

func (a *application) runInitializers() error {
	for _, init := range a.setupConfig.Initializers {
		if err := init(&a.state); err != nil {
			return err
		}
	}
	a.resourcesLoaded = true
	return nil
}

func (a *application) runPostRuns(err error) {
	for _, postRun := range a.setupConfig.postRuns {
		a.runPostRun(postRun, err)
	}
}

func (a *application) runPostRun(fn PostRun, err error) {
	defer func() {
		// handle panics in each postRun -- the app may already be in a panicking situation,
		// this recover should not affect the original panic, as it is being run in a
		// different call stack from the original panic, but the original panic should
		// return without confusing things when a postRun also fails by panic
		if v := recover(); v != nil {
			a.state.Logger.Debugf("panic while calling postRun: %v", v)
		}
	}()
	fn(&a.state, err)
}

func (a *application) WrapRunE(fn func(cmd *cobra.Command, args []string) error) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		wrapper := func(cmd *cobra.Command, args []string) (err error) {
			defer func() {
				// when the worker has completed (or errored) we want to exit the event loop gracefully
				if a.state.Bus != nil {
					a.state.Bus.Publish(ExitEvent(false))
				}
			}()
			defer func() {
				a.runPostRuns(err)
			}()
			err = fn(cmd, args)
			return
		}

		return a.execute(cmd.Context(), async(cmd, args, wrapper))
	}
}

func (a *application) execute(ctx context.Context, errs <-chan error) error {
	if a.state.Config.Dev != nil {
		if profiler := parseProfile(a.state.Config.Dev.Profile); profiler != nil {
			defer profiler()()
		}
	}

	return eventloop(
		ctx,
		a.state.Logger.Nested("component", "eventloop"),
		a.state.Subscription,
		errs,
		a.state.UI,
	)
}

func logVersion(cfg SetupConfig, log logger.Logger) {
	if cfg.ID.Version == "" {
		log.Infof(cfg.ID.Name)
		return
	}
	log.Infof(
		"%s version: %+v",
		cfg.ID.Name,
		cfg.ID.Version,
	)
}

func logConfiguration(log logger.Logger, cfgs ...any) {
	var sb strings.Builder

	for _, cfg := range cfgs {
		if cfg == nil {
			continue
		}

		var str string
		if stringer, ok := cfg.(fmt.Stringer); ok {
			str = stringer.String()
		} else {
			// yaml is pretty human friendly (at least when compared to json)
			cfgBytes, err := yaml.Marshal(cfg)
			if err != nil {
				str = fmt.Sprintf("%+v", err)
			} else {
				str = string(cfgBytes)
			}
		}

		str = strings.TrimSpace(str)

		if str != "" && str != "{}" {
			sb.WriteString(str + "\n")
		}
	}

	content := sb.String()

	if content != "" {
		formatted := color.Magenta.Sprint(indent.String("  ", strings.TrimSpace(content)))
		log.Debugf("config:\n%+v", formatted)
	} else {
		log.Debug("config: (none)")
	}
}

func (a *application) AddFlags(flags *pflag.FlagSet, cfgs ...any) {
	fangs.AddFlags(a.setupConfig.FangsConfig.Logger, flags, cfgs...)
	a.state.Config.FromCommands = append(a.state.Config.FromCommands, cfgs...)
}

func (a *application) SetupCommand(cmd *cobra.Command, cfgs ...any) *cobra.Command {
	return a.setupCommand(cmd, cmd.Flags(), &cmd.PreRunE, cfgs...)
}

func (a *application) Run() {
	if a.root == nil {
		panic(errors.New(setupRootCommandNotCalledError))
	}

	// drive application control from a single context which can be cancelled (notifying the event loop to stop)
	ctx, cancel := context.WithCancel(context.Background())
	a.root.SetContext(ctx)

	// note: it is important to always do signal handling from the main package. In this way if quill is used
	// as a lib a refactor would not need to be done (since anything from the main package cannot be imported this
	// nicely enforces this constraint)
	signals := make(chan os.Signal, 10) // Note: A buffered channel is recommended for this; see https://golang.org/pkg/os/signal/#Notify
	signal.Notify(signals, os.Interrupt)

	var exitCode int

	defer func() {
		if exitCode != 0 {
			os.Exit(exitCode)
		}
	}()

	defer func() {
		signal.Stop(signals)
		cancel()
	}()

	go func() {
		select {
		case <-signals: // first signal, cancel context
			a.state.Logger.Trace("signal interrupt, stop requested")
			cancel()
		case <-ctx.Done():
		}
		<-signals // second signal, hard exit
		a.state.Logger.Trace("signal interrupt, killing")
		exitCode = 1
	}()

	if err := a.root.Execute(); err != nil {
		a.handleExitError(err, os.Stderr)

		exitCode = 1
		if a.setupConfig.mapExitCode != nil {
			exitCode = a.setupConfig.mapExitCode(err)
		}
	}
}

func (a application) handleExitError(err error, stderr io.Writer) {
	msg := color.Red.Render(strings.TrimSpace(err.Error()))

	hasLogger := a.state.Logger != nil
	shouldLog := hasLogger && a.resourcesLoaded
	shouldPrint := !hasLogger || !a.resourcesLoaded

	if shouldLog {
		a.state.Logger.Error(msg)
	}

	if shouldPrint {
		fmt.Fprintln(stderr, msg)
	}
}

func (a *application) SetupRootCommand(cmd *cobra.Command, cfgs ...any) *cobra.Command {
	a.root = cmd
	return a.setupRootCommand(cmd, cfgs...)
}

func (a *application) setupRootCommand(cmd *cobra.Command, cfgs ...any) *cobra.Command {
	if !strings.HasPrefix(cmd.Use, a.setupConfig.ID.Name) {
		cmd.Use = a.setupConfig.ID.Name
	}

	cmd.Version = a.setupConfig.ID.Version

	cmd.SetVersionTemplate(fmt.Sprintf("%s {{.Version}}\n", a.setupConfig.ID.Name))

	// make a copy of the default configs
	a.state.Config.Log = cp(a.setupConfig.DefaultLoggingConfig)
	a.state.Config.Dev = cp(a.setupConfig.DefaultDevelopmentConfig)

	for _, pc := range a.setupConfig.postConstructs {
		pc(a)
	}

	return a.setupCommand(cmd, cmd.Flags(), &cmd.PreRunE, cfgs...)
}

func cp[T any](value *T) *T {
	if value == nil {
		return nil
	}

	t := *value
	return &t
}

func (a *application) setupCommand(cmd *cobra.Command, flags *pflag.FlagSet, fn *func(cmd *cobra.Command, args []string) error, cfgs ...any) *cobra.Command {
	original := *fn
	*fn = func(cmd *cobra.Command, args []string) error {
		err := a.Setup(cfgs...)(cmd, args)
		if err != nil {
			return err
		}
		if original != nil {
			return original(cmd, args)
		}
		return nil
	}

	if cmd.RunE != nil {
		cmd.RunE = a.WrapRunE(cmd.RunE)
	}

	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	a.state.Config.FromCommands = append(a.state.Config.FromCommands, cfgs...)

	fangs.AddFlags(a.setupConfig.FangsConfig.Logger, flags, cfgs...)

	return cmd
}

func async(cmd *cobra.Command, args []string, f func(cmd *cobra.Command, args []string) error) <-chan error {
	errs := make(chan error)
	go func() {
		defer close(errs)
		if err := f(cmd, args); err != nil {
			errs <- err
		}
	}()
	return errs
}

const setupRootCommandNotCalledError = "SetupRootCommand() must be called with the root command"
