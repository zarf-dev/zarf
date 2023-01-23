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

	"github.com/defenseunicorns/zarf/src/internal/packager/template"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils/exec"
	"github.com/defenseunicorns/zarf/src/types"
)

func (p *Packager) runActions(defaultCfg types.ZarfComponentActionDefaults, actions []types.ZarfComponentAction, valueTemplate *template.Values) error {
	for _, a := range actions {
		if err := p.runAction(defaultCfg, a, valueTemplate); err != nil {
			return err
		}
	}
	return nil
}

// Run commands that a component has provided.
func (p *Packager) runAction(defaultCfg types.ZarfComponentActionDefaults, action types.ZarfComponentAction, valueTemplate *template.Values) error {
	spinner := message.NewProgressSpinner("Running command \"%s\"", action.Cmd)
	defer spinner.Success()

	var (
		ctx    context.Context
		cancel context.CancelFunc
		cmd    string
		out    string
		err    error
		vars   map[string]string
	)

	// If the value template is not nil, get the variables for the action.
	// No special variables or deprecations will be used in the action.
	// Reload the variables each time in case they have been changed by a previous action.
	if valueTemplate != nil {
		vars, _ = valueTemplate.GetVariables(types.ZarfComponent{})
	}

	cfg := actionGetCfg(defaultCfg, action, vars)

	if cmd, err = actionCmdMutation(action.Cmd); err != nil {
		spinner.Errorf(err, "Error mutating command: %s", cmd)
	}

	duration := time.Duration(cfg.MaxTotalSeconds) * time.Second
	timeout := time.After(duration)

	// Keep trying until the max retries is reached.
	for remaining := cfg.MaxRetries + 1; remaining > 0; remaining-- {

		// If no timeout is set, run the command and return or continue retrying.
		if cfg.MaxTotalSeconds < 1 {
			spinner.Updatef("Waiting for command \"%s\" (no timeout)", cmd)

			// Try running the command and continue the retry loop if it fails.
			if out, err = actionRun(context.TODO(), cfg, cmd); err != nil {
				message.Debugf("command \"%s\" failed: %s", cmd, err.Error())
				continue
			}

			// If an output variable is defined, set it.
			if action.SetVariable != "" {
				p.setVariable(action.SetVariable, out)
			}

			// If the command ran successfully, continue to the next action.
			return nil
		}

		// Run the command on repeat until success or timeout.
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
			if out, err = actionRun(ctx, cfg, cmd); err != nil {
				message.Debug(err)
				continue
			}

			// If an output variable is defined, set it.
			if action.SetVariable != "" {
				p.setVariable(action.SetVariable, out)
			}

			// If the command ran successfully, continue to the next action.
			return nil
		}
	}

	// If we've reached this point, the retry limit has been reached.
	return fmt.Errorf("command \"%s\" failed after %d retries", cmd, cfg.MaxRetries)
}

// Perform some basic string mutations to make commands more useful.
func actionCmdMutation(cmd string) (string, error) {
	binaryPath, err := os.Executable()
	if err != nil {
		return cmd, err
	}

	// Try to patch the zarf binary path in case the name isn't exactly "./zarf".
	cmd = strings.ReplaceAll(cmd, "./zarf ", binaryPath+" ")

	// Make commands 'more' compatible with Windows OS PowerShell
	if runtime.GOOS == "windows" {
		// Replace "touch" with "New-Item" on Windows as it's a common command, but not POSIX so not aliased by M$.
		// See https://mathieubuisson.github.io/powershell-linux-bash/ &
		// http://web.cs.ucla.edu/~miryung/teaching/EE461L-Spring2012/labs/posix.html for more details.
		cmd = regexp.MustCompile(`^touch `).ReplaceAllString(cmd, `New-Item `)

		// Convert any ${ZARF_VAR_*} or $ZARF_VAR_* to $env:ZARF_VAR_*
		// https://regex101.com/r/BdtUfO/1
		envVarRegex := regexp.MustCompile(`(?P<envIndicator>\${?(?P<varName>ZARF_VAR_([a-zA-Z0-9_-])+)}?)`)
		matches := envVarRegex.FindStringSubmatch(cmd)
		matchIndex := envVarRegex.SubexpIndex
		if len(matches) > 0 {
			newCmd := strings.ReplaceAll(cmd, matches[matchIndex("envIndicator")], fmt.Sprintf("$Env:%s", matches[matchIndex("varName")]))
			message.Debugf("Converted command \"%s\" to \"%s\" t", cmd, newCmd)
			cmd = newCmd
		}
	}

	return cmd, nil
}

// Merge the ActionSet defaults with the action config.
func actionGetCfg(cfg types.ZarfComponentActionDefaults, a types.ZarfComponentAction, vars map[string]string) types.ZarfComponentActionDefaults {
	if a.Mute != nil {
		cfg.Mute = *a.Mute
	}

	// Default is no timeout, but add a timeout if one is provided.
	if a.MaxTotalSeconds != nil {
		cfg.MaxTotalSeconds = *a.MaxTotalSeconds
	}

	if a.MaxRetries != nil {
		cfg.MaxRetries = *a.MaxRetries
	}

	if a.Dir != nil {
		cfg.Dir = *a.Dir
	}

	if len(a.Env) > 0 {
		cfg.Env = append(cfg.Env, a.Env...)
	}

	// Add variables to the environment.
	for k, v := range vars {
		// Remove # from env variable name.
		k = strings.ReplaceAll(k, "#", "")
		// Make terraform variables available to the action as TF_VAR_lowercase_name.
		k1 := strings.ReplaceAll(strings.ToLower(k), "zarf_var", "TF_VAR")
		cfg.Env = append(cfg.Env, fmt.Sprintf("%s=%s", k, v))
		cfg.Env = append(cfg.Env, fmt.Sprintf("%s=%s", k1, v))
	}

	return cfg
}

func actionRun(ctx context.Context, cfg types.ZarfComponentActionDefaults, cmd string) (string, error) {
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

	return output, err
}
