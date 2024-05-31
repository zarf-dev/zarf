// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package common handles command configuration across all commands
package common

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/pterm/pterm"
)

// LogLevelCLI holds the log level as input from a command
var LogLevelCLI string

// SetupCLI sets up the CLI logging, interrupt functions, and more
func SetupCLI() {
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
	if LogLevelCLI != "" {
		if lvl, ok := match[LogLevelCLI]; ok {
			message.SetLogLevel(lvl)
			message.Debug("Log level set to " + LogLevelCLI)
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
		ts := time.Now().Format("2006-01-02-15-04-05")

		f, err := os.CreateTemp("", fmt.Sprintf("zarf-%s-*.log", ts))
		if err != nil {
			message.WarnErr(err, "Error creating a log file in a temporary directory")
			return
		}
		logFile, err := message.UseLogFile(f)
		if err != nil {
			message.WarnErr(err, "Error saving a log file to a temporary directory")
			return
		}

		pterm.SetDefaultOutput(io.MultiWriter(os.Stderr, logFile))
		message.Notef("Saving log file to %s", f.Name())
	}
}
