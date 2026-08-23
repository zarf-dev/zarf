# CLIO

An easy way to bootstrap your application with batteries included.

## Status

***Consider this project to be in alpha. The API is not stable and may change at any time.***

## What is included?
- Pairs well with [cobra](github.com/spf13/cobra) and [viper](github.com/spf13/viper) via [fangs](github.com/anchore/fangs), covering CLI arg parsing and config file + env var loading.
- Provides an event bus via [partybus](github.com/wagoodman/go-partybus), enabling visibility deep in your execution stack as to what is happening.
- Provides a logger via the [logger interface](github.com/anchore/go-logger), allowing you to swap out for any concrete logger you'd like.
- Supplies a redactor object that can be used to remove sensitive output before it's exposed (in the log or elsewhere).
- Defines a generic UI interface that adapts well to TUI frameworks such as [bubbletea](github.com/charmbracelet/bubbletea).

## Example

Here's a basic example of how to use clio + cobra to get a fully functional CLI application going:

```go
package main

import (
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/wagoodman/go-partybus"
	"github.com/anchore/clio"
	"github.com/anchore/fangs"
)

// Define your per-command or entire application config as a struct
type MyCommandConfig struct {
	TimestampServer string `mapstructure:"timestamp-server"`
	// ...
}

// ... add cobra flags just as you are used to doing in any other cobra application
func (c *MyCommandConfig) AddFlags(flags fangs.FlagSet) {
	flags.StringVarP(
		&c.TimestampServer, "timestamp-server", "",
		"URL to a timestamp server to use for timestamping the signature",
	)
	// ...
}

func MyCommand(app clio.Application) *cobra.Command {
	cfg := &MyCommandConfig{
		TimestampServer: "https://somewhere.out/there", // a default value
	}

	return app.SetupCommand(&cobra.Command{
		Use:     "my-command",
		PreRunE: app.Setup(cfg),
		RunE: func(cmd *cobra.Command, args []string) error {
			// perform command functions here
			return nil
		},
	}, cfg)
}

func main() {
	cfg := clio.NewSetupConfig(clio.Identification{
		Name: "awesome",
		Version: "v1.0.0",
    })

	app := clio.New(*cfg)
	
	root := app.SetupRootCommand(&cobra.Command{})

	root.AddCommand(MyCommand(app))

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
```