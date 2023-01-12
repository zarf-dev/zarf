// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying zarf packages.
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

// Run commands that a component has provided.
func (p *Packager) runComponentActions(actionSet types.ZarfComponentActionSet, actions []types.ZarfComponentAction) error {
ACTION:
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

		if cmd, err = actionCmdMutation(a.Cmd); err != nil {
			spinner.Errorf(err, "Error mutating command: %s", cmd)
		}

		duration := time.Duration(cfg.MaxTotalSeconds) * time.Second
		timeout := time.After(duration)

		// Keep trying until the max retries is reached.
		for remaining := cfg.MaxRetries + 1; remaining > 0; remaining-- {

			// If no timeout is set, run the command and return.
			if cfg.MaxTotalSeconds < 1 {
				spinner.Updatef("Waiting for command \"%s\" (no timeout)", cmd)

				// Try running the command and continue the retry loop if it fails.
				if err := actionRun(context.TODO(), cfg, cmd); err != nil {
					message.Debugf("command \"%s\" failed: %s", cmd, err.Error())
					continue
				}

				// If the command ran successfully, continue to the next action.
				continue ACTION
			}

			spinner.Updatef("Waiting for command \"%s\" (timeout: %d seconds)", cmd, cfg.MaxTotalSeconds)

			select {
			// On timeout abort.
			case <-timeout:
				cancel()
				return fmt.Errorf("command \"%s\" timed out", cmd)

			// Otherwise, try running the command.
			default:
				ctx, cancel = context.WithTimeout(context.Background(), duration)
				defer cancel()

				// Try running the command and continue the retry loop if it fails.
				if err := actionRun(ctx, cfg, cmd); err != nil {
					message.Debug(err)
					continue
				}

				// If the command ran successfully, continue to the next action.
				continue ACTION
			}
		}

		// If we've reached this point, the retry limit has been reached.
		return fmt.Errorf("command \"%s\" failed after %d retries", cmd, cfg.MaxRetries)
	}

	// If we've reached this point, all actions have been run successfully.
	return nil
}

// Perform some basic string mutations to make commands more useful.
func actionCmdMutation(cmd string) (string, error) {
	binaryPath, err := os.Executable()
	if err != nil {
		return cmd, err
	}

	// Try to patch the zarf binary path in case the name isn't exactly "./zarf".
	cmd = strings.ReplaceAll(cmd, "./zarf ", binaryPath+" ")

	// Replace "touch" with "New-Item" on Windows as it's a common command, but not POSIX so not aliases by M$.
	// See https://mathieubuisson.github.io/powershell-linux-bash/ &
	// http://web.cs.ucla.edu/~miryung/teaching/EE461L-Spring2012/labs/posix.html for more details.
	if runtime.GOOS == "windows" {
		cmd = regexp.MustCompile(`^touch `).ReplaceAllString(cmd, `New-Item `)
	}

	return cmd, nil
}

// Merge the actionset defaults with the action config.
func actionGetCfg(actionSet types.ZarfComponentActionSet, a types.ZarfComponentAction) types.ZarfComponentActionDefaults {
	cfg := actionSet.Defaults

	if !a.Mute {
		cfg.Mute = a.Mute
	}

	// Default is no timeout, but add a timeout if one is provided.
	if a.MaxTotalSeconds > 0 {
		cfg.MaxTotalSeconds = a.MaxTotalSeconds
	}

	if a.MaxRetries > 0 {
		cfg.MaxRetries = a.MaxRetries
	}

	if a.Dir != "" {
		cfg.Dir = a.Dir
	}

	if len(a.Env) > 0 {
		cfg.Env = append(cfg.Env, a.Env...)
	}

	return cfg
}

func actionRun(ctx context.Context, cfg types.ZarfComponentActionDefaults, cmd string) error {
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
	// Dump the command output in debug if output not already streamed.
	if cfg.Mute {
		message.Debug(output, errOut)
	}

	return err
}
