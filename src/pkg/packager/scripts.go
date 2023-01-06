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
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

// Run scripts that a component has provided.
func (p *Packager) runComponentScripts(scripts []string, componentScript types.ZarfComponentScripts) error {
	for _, script := range scripts {
		if err := p.loopScriptUntilSuccess(script, componentScript); err != nil {
			return err
		}
	}

	return nil
}

func (p *Packager) loopScriptUntilSuccess(script string, scripts types.ZarfComponentScripts) error {
	spinner := message.NewProgressSpinner("Waiting for command \"%s\"", script)
	defer spinner.Success()

	var ctx context.Context
	var cancel context.CancelFunc

	// Default timeout is 5 minutes
	if scripts.TimeoutSeconds < 1 {
		scripts.TimeoutSeconds = 300
	}

	duration := time.Duration(scripts.TimeoutSeconds) * time.Second
	timeout := time.After(duration)

	script, err := p.scriptMutation(script)
	if err != nil {
		spinner.Errorf(err, "Error mutating script: %s", script)
	}

	spinner.Updatef("Waiting for command \"%s\" (timeout: %d seconds)", script, scripts.TimeoutSeconds)

	for {
		select {
		// On timeout abort
		case <-timeout:
			cancel()
			return fmt.Errorf("script \"%s\" timed out", script)

		// Otherwise try running the script
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

			output, errOut, err := utils.ExecCommandWithContext(ctx, scripts.ShowOutput, shell, shellArgs, script)

			if err != nil {
				message.Debug(err, output, errOut)
				// If retry, let the script run again
				if scripts.Retry {
					continue
				}
				// Otherwise, fail
				return fmt.Errorf("script \"%s\" failed: %w", script, err)
			}

			// Dump the script output in debug if output not already streamed
			if !scripts.ShowOutput {
				message.Debug(output, errOut)
			}

			// Close the function now that we are done
			return nil
		}
	}
}

// Perform some basic string mutations to make scripts more useful.
func (p *Packager) scriptMutation(script string) (string, error) {

	binaryPath, err := os.Executable()
	if err != nil {
		return script, err
	}

	// Try to patch the zarf binary path in case the name isn't exactly "./zarf"
	script = strings.ReplaceAll(script, "./zarf ", binaryPath+" ")

	// Replace "touch" with "New-Item" on Windows as it's a common command, but not POSIX so not aliases by M$
	// See https://mathieubuisson.github.io/powershell-linux-bash/ &
	// http://web.cs.ucla.edu/~miryung/teaching/EE461L-Spring2012/labs/posix.html for more details
	if runtime.GOOS == "windows" {
		script = regexp.MustCompile(`^touch `).ReplaceAllString(script, `New-Item `)
	}

	return script, nil
}
