// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying zarf packages
package packager

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils/exec"
	"github.com/defenseunicorns/zarf/src/types"
)

// Run commands that a component has provided
func (p *Packager) runComponentActions(actionSet types.ZarfComponentActionSet, actions []types.ZarfComponentAction) error {
	for _, a := range actions {
		spinner := message.NewProgressSpinner("Running command \"%s\"", a.Cmd)
		defer spinner.Success()

		var (
			ctx    context.Context
			cancel context.CancelFunc
			cmd    string
			err    error
		)

		cfg := actionGetCfg(actionSet, a)
		duration := time.Duration(cfg.MaxSeconds) * time.Second
		timeout := time.After(duration)

		if cmd, err = actionCmdMutation(a.Cmd); err != nil {
			spinner.Errorf(err, "Error mutating command: %s", cmd)
		}

		if cfg.MaxSeconds > 0 {
			spinner.Updatef("Waiting for command \"%s\" (timeout: %d seconds)", cmd, cfg.MaxSeconds)
		} else {
			spinner.Updatef("Waiting for command \"%s\" (no timeout)", cmd)
		}

		for {
			select {
			// On timeout abort
			case <-timeout:
				cancel()
				return fmt.Errorf("command \"%s\" timed out", cmd)

			// Otherwise try running the command
			default:
				ctx, cancel = context.WithTimeout(context.Background(), duration)
				defer cancel()

				var shell string
				var shellArgs string

				if runtime.GOOS == "windows" {
					shell = "powershell"
					shellArgs = "-Command"
				} else {
					shell = "sh"
					shellArgs = "-c"
				}

				execCfg := exec.Config{
					Print: !cfg.Mute,
					Env:   cfg.Env,
					Dir:   cfg.Dir,
				}
				output, errOut, err := exec.CmdWithContext(ctx, execCfg, shell, shellArgs, cmd)

				if err != nil {
					message.Debug(err, output, errOut)
					// If retry, let the command run again
					if cfg.Retry {
						continue
					}
					// Otherwise, fail
					return fmt.Errorf("command \"%s\" failed: %w", cmd, err)
				}

				// Dump the command output in debug if output not already streamed
				if cfg.Mute {
					message.Debug(output, errOut)
				}

				// Close the function now that we are done
				return nil
			}
		}
	}

	return nil
}

// Perform some basic string mutations to make commands more useful
func actionCmdMutation(cmd string) (string, error) {
	binaryPath, err := os.Executable()
	if err != nil {
		return cmd, err
	}

	// Try to patch the zarf binary path in case the name isn't exactly "./zarf"
	cmd = strings.ReplaceAll(cmd, "./zarf ", binaryPath+" ")

	// Replace "touch" with "New-Item" on Windows as it's a common command, but not POSIX so not aliases by M$
	// See https://mathieubuisson.github.io/powershell-linux-bash/ &
	// http://web.cs.ucla.edu/~miryung/teaching/EE461L-Spring2012/labs/posix.html for more details
	if runtime.GOOS == "windows" {
		cmd = regexp.MustCompile(`^touch `).ReplaceAllString(cmd, `New-Item `)
	}

	return cmd, nil
}

// Merge the actionset defaults with the action config
func actionGetCfg(actionSet types.ZarfComponentActionSet, a types.ZarfComponentAction) types.ZarfComponentActionDefaults {
	cfg := actionSet.Defaults

	if !a.Mute {
		cfg.Mute = a.Mute
	}

	// Default is no timeout, but add a timeout if one is provided
	if a.MaxSeconds > 0 {
		cfg.MaxSeconds = a.MaxSeconds
	}

	if a.Retry {
		cfg.Retry = a.Retry
	}

	if a.Dir != "" {
		cfg.Dir = a.Dir
	}

	if len(a.Env) > 0 {
		cfg.Env = append(cfg.Env, a.Env...)
	}

	return cfg
}
