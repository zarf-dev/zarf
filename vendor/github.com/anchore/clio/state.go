package clio

import (
	"fmt"

	"github.com/wagoodman/go-partybus"

	"github.com/anchore/go-logger"
	"github.com/anchore/go-logger/adapter/redact"
)

type State struct {
	Config       Config
	Bus          *partybus.Bus
	Subscription *partybus.Subscription
	Logger       logger.Logger
	RedactStore  redact.Store
	UI           *UICollection
}

type Config struct {
	// Items that end up in the target application configuration
	Log *LoggingConfig     `yaml:"log" json:"log" mapstructure:"log"`
	Dev *DevelopmentConfig `yaml:"dev" json:"dev" mapstructure:"dev"`

	// this is a list of all "config" objects from SetupCommand calls
	FromCommands []any `yaml:"-" json:"-" mapstructure:"-"`
}

func (s *State) setup(cfg SetupConfig) error {
	s.setupBus(cfg.BusConstructor)

	if err := s.setupLogger(cfg.LoggerConstructor); err != nil {
		return fmt.Errorf("unable to setup logger: %w", err)
	}

	if err := s.setupUI(cfg.UIConstructor); err != nil {
		return fmt.Errorf("unable to setup UI: %w", err)
	}
	return nil
}

func (s *State) setupLogger(cx LoggerConstructor) error {
	if cx == nil {
		cx = DefaultLogger
	}

	lgr, err := cx(s.Config, s.RedactStore)
	if err != nil {
		return err
	}

	s.Logger = lgr
	return nil
}

func (s *State) setupBus(cx BusConstructor) {
	if cx == nil {
		cx = newBus
	}
	s.Bus = cx(s.Config)
	if s.Bus != nil {
		s.Subscription = s.Bus.Subscribe()
	}
}

func (s *State) setupUI(cx UIConstructor) error {
	if cx == nil {
		cx = newUI
	}
	var err error
	s.UI, err = cx(s.Config)
	return err
}
