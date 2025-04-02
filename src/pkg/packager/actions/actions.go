// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package actions contains functions for running component actions within Zarf packages.
package actions

import (
	"context"
	"fmt"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/internal/packager/template"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/utils/exec"
	"github.com/zarf-dev/zarf/src/pkg/variables"
)

// Run runs all provided actions.
func Run(ctx context.Context, defaultCfg v1alpha1.ZarfComponentActionDefaults, actions []v1alpha1.ZarfComponentAction, variableConfig *variables.VariableConfig) error {
	// TODO(mkcp): Remove interactive on logger release
	if variableConfig == nil {
		variableConfig = template.GetZarfVariableConfig(ctx)
	}

	for _, a := range actions {
		if err := runAction(ctx, defaultCfg, a, variableConfig); err != nil {
			return err
		}
	}
	return nil
}

// Run commands that a component has provided.
func runAction(ctx context.Context, defaultCfg v1alpha1.ZarfComponentActionDefaults, action v1alpha1.ZarfComponentAction, variableConfig *variables.VariableConfig) error {
	var cmdEscaped string
	var err error
	cmd := action.Cmd
	l := logger.From(ctx)
	start := time.Now()

	// If the action is a wait, convert it to a command.
	if action.Wait != nil {
		// If the wait has no timeout, set a default of 5 minutes.
		if action.MaxTotalSeconds == nil {
			fiveMin := 300
			action.MaxTotalSeconds = &fiveMin
		}

		// Convert the wait to a command.
		if cmd, err = convertWaitToCmd(ctx, *action.Wait, action.MaxTotalSeconds); err != nil {
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
		action.SetVariables = []v1alpha1.Variable{}
	}

	if action.Description != "" {
		cmdEscaped = action.Description
	} else {
		cmdEscaped = helpers.Truncate(cmd, 60, false)
	}

	l.Info("running command", "cmd", cmdEscaped)

	actionDefaults := actionGetCfg(ctx, defaultCfg, action, variableConfig.GetAllTemplates())

	if cmd, err = actionCmdMutation(ctx, cmd, actionDefaults.Shell); err != nil {
		l.Error("error mutating command", "cmd", cmdEscaped, "err", err.Error())
	}

	duration := time.Duration(actionDefaults.MaxTotalSeconds) * time.Second
	timeout := time.After(duration)

	// Keep trying until the max retries is reached.
	// TODO: Refactor using go-retry
retryCmd:
	for remaining := actionDefaults.MaxRetries + 1; remaining > 0; remaining-- {
		// Perform the action run.
		tryCmd := func(ctx context.Context) error {
			// Try running the command and continue the retry loop if it fails.
			stdout, stderr, err := actionRun(ctx, actionDefaults, cmd)
			if err != nil {
				if !actionDefaults.Mute {
					l.Warn("action failed", "cmd", cmdEscaped, "stdout", stdout, "stderr", stderr)
				}
				return err
			}
			if !actionDefaults.Mute {
				l.Info("action succeeded", "cmd", cmdEscaped, "stdout", stdout, "stderr", stderr)
			}

			outTrimmed := strings.TrimSpace(stdout)

			// If an output variable is defined, set it.
			for _, v := range action.SetVariables {
				variableConfig.SetVariable(v.Name, outTrimmed, v.Sensitive, v.AutoIndent, v.Type)
				if err := variableConfig.CheckVariablePattern(v.Name, v.Pattern); err != nil {
					return err
				}
			}

			// If the action has a wait, change the spinner message to reflect that on success.
			if action.Wait != nil {
				l.Debug("wait for action succeeded", "cmd", cmdEscaped, "duration", time.Since(start))
				return nil
			}

			l.Debug("completed action", "cmd", cmdEscaped, "duration", time.Since(start))

			// If the command ran successfully, continue to the next action.
			return nil
		}

		// If no timeout is set, run the command and return or continue retrying.
		if actionDefaults.MaxTotalSeconds < 1 {
			l.Info("waiting for action (no timeout)", "cmd", cmdEscaped)
			if err := tryCmd(ctx); err != nil {
				continue retryCmd
			}

			return nil
		}

		// Run the command on repeat until success or timeout.
		l.Info("waiting for action", "cmd", cmdEscaped, "timeout", actionDefaults.MaxTotalSeconds)
		select {
		// On timeout break the loop to abort.
		case <-timeout:
			break retryCmd

		// Otherwise, try running the command.
		default:
			ctx, cancel := context.WithTimeout(ctx, duration)
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
		if actionDefaults.MaxTotalSeconds < 1 {
			return fmt.Errorf("command %q failed after %d retries", cmdEscaped, actionDefaults.MaxRetries)
		} else {
			return fmt.Errorf("command %q timed out after %d seconds", cmdEscaped, actionDefaults.MaxTotalSeconds)
		}
	default:
		// If we reached this point, the retry limit was reached.
		return fmt.Errorf("command %q failed after %d retries", cmdEscaped, actionDefaults.MaxRetries)
	}
}

// convertWaitToCmd will return the wait command if it exists, otherwise it will return the original command.
func convertWaitToCmd(_ context.Context, wait v1alpha1.ZarfComponentActionWait, timeout *int) (string, error) {
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
			cluster.Kind, cluster.Name, cluster.Condition, ns, timeoutString), nil
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
func actionCmdMutation(ctx context.Context, cmd string, shellPref v1alpha1.Shell) (string, error) {
	zarfCommand, err := utils.GetFinalExecutableCommand()
	if err != nil {
		return cmd, err
	}

	// Try to patch the zarf binary path in case the name isn't exactly "./zarf".
	cmd = strings.ReplaceAll(cmd, "./zarf ", zarfCommand+" ")

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
			logger.From(ctx).Debug("converted command", "cmd", cmd, "newCmd", newCmd)
			cmd = newCmd
		}
	}

	return cmd, nil
}

// Merge the ActionSet defaults with the action config.
func actionGetCfg(_ context.Context, cfg v1alpha1.ZarfComponentActionDefaults, a v1alpha1.ZarfComponentAction, vars map[string]*variables.TextTemplate) v1alpha1.ZarfComponentActionDefaults {
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

func actionRun(ctx context.Context, cfg v1alpha1.ZarfComponentActionDefaults, cmd string) (string, string, error) {
	l := logger.From(ctx)
	shell, shellArgs := exec.GetOSShell(cfg.Shell)

	l.Debug("running command", "shell", shell, "cmd", cmd)

	execCfg := exec.Config{
		Env: cfg.Env,
		Dir: cfg.Dir,
	}

	stdout, stderr, err := exec.CmdWithContext(ctx, execCfg, shell, append(shellArgs, cmd)...)
	// Dump final complete output (respect mute to prevent sensitive values from hitting the logs).
	if !cfg.Mute {
		l.Debug("action complete", "cmd", cmd, "stdout", stdout, "stderr", stderr)
	}
	return stdout, stderr, err
}
