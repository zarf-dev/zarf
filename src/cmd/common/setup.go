// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package common handles command configuration across all commands
package common

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/pterm/pterm"

	"github.com/zarf-dev/zarf/src/pkg/message"
)

// SetupCLI sets up the CLI logging
func SetupCLI(logLevel string, skipLogFile, noColor bool) error {
	if noColor {
		message.DisableColor()
	}

	printViperConfigUsed()

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
