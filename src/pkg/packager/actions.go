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
	"runtime/debug"
	"strings"
	"time"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/packager/template"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/exec"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
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
	var (
		ctx        context.Context
		cancel     context.CancelFunc
		cmdEscaped string
		out        string
		err        error
		vars       map[string]*utils.TextTemplate

		cmd = action.Cmd
	)

	// If the action is a wait, convert it to a command.
	if action.Wait != nil {
		// If the wait has no timeout, set a default of 5 minutes.
		if action.MaxTotalSeconds == nil {
			fiveMin := 300
			action.MaxTotalSeconds = &fiveMin
		}

		// Convert the wait to a command.
		if cmd, err = convertWaitToCmd(*action.Wait, action.MaxTotalSeconds); err != nil {
			return err
		}

		// Mute the output becuase it will be noisy.
		t := true
		action.Mute = &t

		// Set the max retries to 0.
		z := 0
		action.MaxRetries = &z

		// Not used for wait actions.
		d := ""
		action.Dir = &d
		action.Env = []string{}
		action.SetVariables = []types.ZarfComponentActionSetVariable{}
	}

	if action.Description != "" {
		cmdEscaped = action.Description
	} else {
		cmdEscaped = message.Truncate(cmd, 60, false)
	}

	spinner := message.NewProgressSpinner("Running \"%s\"", cmdEscaped)
	// Persist the spinner output so it doesn't get overwritten by the command output.
	spinner.EnablePreserveWrites()

	// If the value template is not nil, get the variables for the action.
	// No special variables or deprecations will be used in the action.
	// Reload the variables each time in case they have been changed by a previous action.
	if valueTemplate != nil {
		vars, _ = valueTemplate.GetVariables(types.ZarfComponent{})
	}

	cfg := actionGetCfg(defaultCfg, action, vars)

	if cmd, err = actionCmdMutation(cmd, cfg.Shell); err != nil {
		spinner.Errorf(err, "Error mutating command: %s", cmdEscaped)
	}

	duration := time.Duration(cfg.MaxTotalSeconds) * time.Second
	timeout := time.After(duration)

	// Keep trying until the max retries is reached.
retryCmd:
	for remaining := cfg.MaxRetries + 1; remaining > 0; remaining-- {

		// Perform the action run.
		tryCmd := func(ctx context.Context) error {
			// Try running the command and continue the retry loop if it fails.
			if out, err = actionRun(ctx, cfg, cmd, cfg.Shell, spinner); err != nil {
				return err
			}

			out = strings.TrimSpace(out)

			// If an output variable is defined, set it.
			for _, v := range action.SetVariables {
				p.setVariableInConfig(v.Name, out, v.Sensitive, v.AutoIndent, v.Type)
				if err := p.checkVariablePattern(v.Name, v.Pattern); err != nil {
					message.WarnErr(err, err.Error())
					return err
				}
			}

			// If the action has a wait, change the spinner message to reflect that on success.
			if action.Wait != nil {
				spinner.Successf("Wait for \"%s\" succeeded", cmdEscaped)
			} else {
				spinner.Successf("Completed \"%s\"", cmdEscaped)
			}

			// If the command ran successfully, continue to the next action.
			return nil
		}

		// If no timeout is set, run the command and return or continue retrying.
		if cfg.MaxTotalSeconds < 1 {
			spinner.Updatef("Waiting for \"%s\" (no timeout)", cmdEscaped)
			if err := tryCmd(context.TODO()); err != nil {
				continue retryCmd
			}

			return nil
		}

		// Run the command on repeat until success or timeout.
		spinner.Updatef("Waiting for \"%s\" (timeout: %ds)", cmdEscaped, cfg.MaxTotalSeconds)
		select {
		// On timeout break the loop to abort.
		case <-timeout:
			break retryCmd

		// Otherwise, try running the command.
		default:
			ctx, cancel = context.WithTimeout(context.Background(), duration)
			defer cancel()
			if err := tryCmd(ctx); err != nil {
				continue retryCmd
			}

			return nil
		}
	}

	select {
	case <-timeout:
		// If we reached this point, the timeout was reached.
		return fmt.Errorf("command \"%s\" timed out after %d seconds", cmdEscaped, cfg.MaxTotalSeconds)

	default:
		// If we reached this point, the retry limit was reached.
		return fmt.Errorf("command \"%s\" failed after %d retries", cmdEscaped, cfg.MaxRetries)
	}
}

// convertWaitToCmd will return the wait command if it exists, otherwise it will return the original command.
func convertWaitToCmd(wait types.ZarfComponentActionWait, timeout *int) (string, error) {
	// Build the timeout string.
	timeoutString := fmt.Sprintf("--timeout %ds", *timeout)

	// If the action has a wait, build a cmd from that instead.
	cluster := wait.Cluster
	if cluster != nil {
		ns := cluster.Namespace
		if ns != "" {
			ns = fmt.Sprintf("-n %s", ns)
		}

		// Build a call to the zarf tools wait-for command.
		return fmt.Sprintf("./zarf tools wait-for %s %s %s %s %s",
			cluster.Kind, cluster.Identifier, cluster.Condition, ns, timeoutString), nil
	}

	network := wait.Network
	if network != nil {
		// Make sure the protocol is lower case.
		network.Protocol = strings.ToLower(network.Protocol)

		// If the protocol is http and no code is set, default to 200.
		if strings.HasPrefix(network.Protocol, "http") && network.Code == 0 {
			network.Code = 200
		}

		// Build a call to the zarf tools wait-for command.
		return fmt.Sprintf("./zarf tools wait-for %s %s %d %s",
			network.Protocol, network.Address, network.Code, timeoutString), nil
	}

	return "", fmt.Errorf("wait action is missing a cluster or network")
}

// Perform some basic string mutations to make commands more useful.
func actionCmdMutation(cmd string, shellPref types.ZarfComponentActionShell) (string, error) {
	binaryPath, err := os.Executable()
	if err != nil {
		return cmd, err
	}

	// If zarf is used as a library, os.Executable() will return the path to the binary that called it.
	//
	// To verify this, we can check the build info to see if the main module path of the current executable
	// matches Zarf's main module path.
	//
	// The likelyhood of this being a false positive are extremely low.
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return cmd, fmt.Errorf("could not read build info")
	}
	isZarf := bi.Main.Path == "github.com/"+config.GithubProject
	if !isZarf {
		binaryPath = "zarf"
	}

	// Try to patch the zarf binary path in case the name isn't exactly "./zarf".
	cmd = strings.ReplaceAll(cmd, "./zarf ", binaryPath+" ")

	// Make commands 'more' compatible with Windows OS PowerShell
	if runtime.GOOS == "windows" && (exec.IsPowershell(shellPref.Windows) || shellPref.Windows == "") {
		// Replace "touch" with "New-Item" on Windows as it's a common command, but not POSIX so not aliased by M$.
		// See https://mathieubuisson.github.io/powershell-linux-bash/ &
		// http://web.cs.ucla.edu/~miryung/teaching/EE461L-Spring2012/labs/posix.html for more details.
		cmd = regexp.MustCompile(`^touch `).ReplaceAllString(cmd, `New-Item `)

		// Convert any ${ZARF_VAR_*} or $ZARF_VAR_* to ${env:ZARF_VAR_*} or $env:ZARF_VAR_* respectively (also TF_VAR_*).
		// https://regex101.com/r/xk1rkw/1
		envVarRegex := regexp.MustCompile(`(?P<envIndicator>\${?(?P<varName>(ZARF|TF)_VAR_([a-zA-Z0-9_-])+)}?)`)
		get, err := helpers.MatchRegex(envVarRegex, cmd)
		if err == nil {
			newCmd := strings.ReplaceAll(cmd, get("envIndicator"), fmt.Sprintf("$Env:%s", get("varName")))
			message.Debugf("Converted command \"%s\" to \"%s\" t", cmd, newCmd)
			cmd = newCmd
		}
	}

	return cmd, nil
}

// Merge the ActionSet defaults with the action config.
func actionGetCfg(cfg types.ZarfComponentActionDefaults, a types.ZarfComponentAction, vars map[string]*utils.TextTemplate) types.ZarfComponentActionDefaults {
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

	if a.Shell != nil {
		cfg.Shell = *a.Shell
	}

	// Add variables to the environment.
	for k, v := range vars {
		// Remove # from env variable name.
		k = strings.ReplaceAll(k, "#", "")
		// Make terraform variables available to the action as TF_VAR_lowercase_name.
		k1 := strings.ReplaceAll(strings.ToLower(k), "zarf_var", "TF_VAR")
		cfg.Env = append(cfg.Env, fmt.Sprintf("%s=%s", k, v.Value))
		cfg.Env = append(cfg.Env, fmt.Sprintf("%s=%s", k1, v.Value))
	}

	return cfg
}

func actionRun(ctx context.Context, cfg types.ZarfComponentActionDefaults, cmd string, shellPref types.ZarfComponentActionShell, spinner *message.Spinner) (string, error) {
	shell, shellArgs := exec.GetOSShell(shellPref)

	message.Debugf("Running command in %s: %s", shell, cmd)

	execCfg := exec.Config{
		Env: cfg.Env,
		Dir: cfg.Dir,
	}

	if !cfg.Mute {
		execCfg.Stdout = spinner
		execCfg.Stderr = spinner
	}

	out, errOut, err := exec.CmdWithContext(ctx, execCfg, shell, shellArgs, cmd)
	// Dump final complete output (respect mute to prevent sensitive values from hitting the logs).
	if !cfg.Mute {
		message.Debug(cmd, out, errOut)
	}

	return out, err
}
