// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package actions contains functions for running commands and tasks
package actions

import (
	"context"
	"fmt"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/defenseunicorns/zarf/src/pkg/utils/exec"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/pkg/variables"
)

// Spinner includes various messaging functions that are used in actions
// TODO: Make this a real interface for progress
type Spinner interface {
	Errorf(err error, format string, a ...any)
	Successf(format string, a ...any)
	Updatef(format string, a ...any)
	Write(p []byte) (n int, err error)
}

// ActionRunner contains the configuration for running actions
type ActionRunner struct {
	commandName string
	commandPath string
	waitCommand string
	logger      func(payload ...any)
}

// New creates a new ActionConfig
func New(commandName string, commandPath string, waitCommand string, logger func(payload ...any)) *ActionRunner {
	return &ActionRunner{
		commandName,
		commandPath,
		waitCommand,
		logger,
	}
}

// RunAction runs an action per the ActionConfig.
func (ac *ActionRunner) RunAction(defaultCfg ActionDefaults, action Action, variableConfig *variables.VariableConfig, spinner Spinner) error {
	var (
		ctx        context.Context
		cancel     context.CancelFunc
		cmdEscaped string
		out        string
		err        error

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
		if cmd, err = ac.ConvertWaitToCmd(*action.Wait, action.MaxTotalSeconds); err != nil {
			return err
		}

		// Mute the output because it will be noisy.
		t := true
		action.Mute = &t

		// Set the max retries to 0.
		z := 0
		action.MaxRetries = &z

		// Not used for wait actions.
		d := ""
		action.Dir = &d
		action.Env = []string{}
		action.SetVariables = []ActionSetVariable{}
	}

	if action.Description != "" {
		cmdEscaped = action.Description
	} else {
		cmdEscaped = helpers.Truncate(cmd, 60, false)
	}

	cfg := ac.actionMergeDefaults(defaultCfg, action, variableConfig.GetAllTemplates())

	if cmd, err = ac.actionCmdMutation(cmd, cfg.Shell); err != nil {
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
			if out, err = ac.execAction(ctx, cfg, cmd, cfg.Shell, spinner); err != nil {
				return err
			}

			out = strings.TrimSpace(out)

			// If an output variable is defined, set it.
			for _, v := range action.SetVariables {
				variableConfig.SetVariableMap.SetVariableInMap(v.Name, out, v.Sensitive, v.AutoIndent, v.Type)
				if err := variableConfig.SetVariableMap.CheckVariablePattern(v.Name, v.Pattern); err != nil {
					// message.WarnErr(err, err.Error())
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
		// If we reached this point, the timeout was reached or command failed with no retries.
		if cfg.MaxTotalSeconds < 1 {
			return fmt.Errorf("command %q failed after %d retries", cmdEscaped, cfg.MaxRetries)
		} else {
			return fmt.Errorf("command %q timed out after %d seconds", cmdEscaped, cfg.MaxTotalSeconds)
		}
	default:
		// If we reached this point, the retry limit was reached.
		return fmt.Errorf("command %q failed after %d retries", cmdEscaped, cfg.MaxRetries)
	}
}

// ConvertWaitToCmd will return the wait command if it exists, otherwise it will return the original command.
func (ac *ActionRunner) ConvertWaitToCmd(wait ActionWait, timeout *int) (string, error) {
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
		return fmt.Sprintf("./%s %s %s %s %s %s", ac.waitCommand,
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
		return fmt.Sprintf("./%s %s %s %d %s", ac.waitCommand,
			network.Protocol, network.Address, network.Code, timeoutString), nil
	}

	return "", fmt.Errorf("wait action is missing a cluster or network")
}

// actionCmdMutation performs some basic string mutations to make commands more useful.
func (ac *ActionRunner) actionCmdMutation(cmd string, shellPref exec.ExecShell) (string, error) {
	// Try to patch the zarf binary path in case the name isn't exactly "./zarf".
	cmd = strings.ReplaceAll(cmd, fmt.Sprintf("./%s ", ac.commandName), ac.commandPath+" ")

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
			ac.logger(fmt.Sprintf("Converted command \"%s\" to \"%s\" t", cmd, newCmd))
			cmd = newCmd
		}
	}

	return cmd, nil
}

// actionMergeDefaults merges the ActionSet defaults with the specified action.
func (ac *ActionRunner) actionMergeDefaults(def ActionDefaults, a Action, vars map[string]*variables.TextTemplate) ActionDefaults {
	if a.Mute != nil {
		def.Mute = *a.Mute
	}

	// Default is no timeout, but add a timeout if one is provided.
	if a.MaxTotalSeconds != nil {
		def.MaxTotalSeconds = *a.MaxTotalSeconds
	}

	if a.MaxRetries != nil {
		def.MaxRetries = *a.MaxRetries
	}

	if a.Dir != nil {
		def.Dir = *a.Dir
	}

	if len(a.Env) > 0 {
		def.Env = append(def.Env, a.Env...)
	}

	if a.Shell != nil {
		def.Shell = *a.Shell
	}

	// Add variables to the environment.
	for k, v := range vars {
		// Remove # from env variable name.
		k = strings.ReplaceAll(k, "#", "")
		// Make terraform variables available to the action as TF_VAR_lowercase_name.
		k1 := strings.ReplaceAll(strings.ToLower(k), fmt.Sprintf("%s_var", ac.commandName), "TF_VAR")
		def.Env = append(def.Env, fmt.Sprintf("%s=%s", k, v.Value))
		def.Env = append(def.Env, fmt.Sprintf("%s=%s", k1, v.Value))
	}

	return def
}

// execAction executes the built command in a shell
func (ac *ActionRunner) execAction(ctx context.Context, cfg ActionDefaults, cmd string, shellPref exec.ExecShell, spinner Spinner) (string, error) {
	shell, shellArgs := exec.GetOSShell(shellPref)

	ac.logger(fmt.Sprintf("Running command in %s: %s", shell, cmd))

	execCfg := exec.Config{
		Env: cfg.Env,
		Dir: cfg.Dir,
	}

	if !cfg.Mute {
		execCfg.Stdout = spinner
		execCfg.Stderr = spinner
	}

	out, errOut, err := exec.CmdWithContext(ctx, execCfg, shell, append(shellArgs, cmd)...)
	// Dump final complete output (respect mute to prevent sensitive values from hitting the logs).
	if !cfg.Mute {
		ac.logger(cmd, out, errOut)
	}

	return out, err
}
