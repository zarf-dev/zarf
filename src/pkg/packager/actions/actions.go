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

	"github.com/avast/retry-go/v4"
	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/internal/packager/template"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/utils/exec"
	"github.com/zarf-dev/zarf/src/pkg/variables"
)

// Run runs all provided actions.
func Run(ctx context.Context, defaultCfg v1alpha1.ZarfComponentActionDefaults, actions []v1alpha1.ZarfComponentAction, variableConfig *variables.VariableConfig) error {
	if variableConfig == nil {
		variableConfig = template.GetZarfVariableConfig()
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
	var (
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

	spinner := message.NewProgressSpinner("Running \"%s\"", cmdEscaped)
	// Persist the spinner output so it doesn't get overwritten by the command output.
	spinner.EnablePreserveWrites()

	actionDefaults := actionGetCfg(ctx, defaultCfg, action, variableConfig.GetAllTemplates())

	if cmd, err = actionCmdMutation(ctx, cmd, actionDefaults.Shell); err != nil {
		spinner.Errorf(err, "Error mutating command: %s", cmdEscaped)
	}

	// Keep trying until the max retries or timeout is reached.
	actionCtx := ctx
	if actionDefaults.MaxTotalSeconds > 0 {
		var actionCancel context.CancelFunc
		actionCtx, actionCancel = context.WithTimeout(ctx, time.Duration(actionDefaults.MaxTotalSeconds)*time.Second)
		defer actionCancel()
	}
	err = retry.Do(func() error {
		if out, err = actionRun(actionCtx, actionDefaults, cmd, actionDefaults.Shell, spinner); err != nil {
			return err
		}
		out = strings.TrimSpace(out)

		// If an output variable is defined, set it.
		for _, v := range action.SetVariables {
			variableConfig.SetVariable(v.Name, out, v.Sensitive, v.AutoIndent, v.Type)
			if err := variableConfig.CheckVariablePattern(v.Name, v.Pattern); err != nil {
				return err
			}
		}

		// If the action has a wait, change the spinner message to reflect that on success.
		if action.Wait != nil {
			spinner.Successf("Wait for \"%s\" succeeded", cmdEscaped)
		} else {
			spinner.Successf("Completed \"%s\"", cmdEscaped)
		}

		return nil
	},
		retry.Context(actionCtx),
		retry.Attempts(uint(actionDefaults.MaxRetries)),
		retry.DelayType(retry.FixedDelay),
		retry.Delay(100*time.Millisecond),
		retry.OnRetry(func(uint, error) {
			timeoutStr := ""
			if actionDefaults.MaxTotalSeconds > 0 {
				timeoutStr = fmt.Sprintf("(timeout: %ds)", actionDefaults.MaxTotalSeconds)
			}
			spinner.Updatef("Waiting for \"%s\" %s", cmdEscaped, timeoutStr)
		}),
	)
	if actionCtx.Err() != nil {
		return fmt.Errorf("command %q timed out after %d seconds: %w", cmdEscaped, actionDefaults.MaxTotalSeconds, err)
	}
	if err != nil {
		return fmt.Errorf("command %q failed after %d retries: %w", cmdEscaped, actionDefaults.MaxRetries, err)
	}
	return nil
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
func actionCmdMutation(_ context.Context, cmd string, shellPref v1alpha1.Shell) (string, error) {
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
			message.Debugf("Converted command \"%s\" to \"%s\" t", cmd, newCmd)
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

func actionRun(ctx context.Context, cfg v1alpha1.ZarfComponentActionDefaults, cmd string, shellPref v1alpha1.Shell, spinner *message.Spinner) (string, error) {
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

	out, errOut, err := exec.CmdWithContext(ctx, execCfg, shell, append(shellArgs, cmd)...)
	// Dump final complete output (respect mute to prevent sensitive values from hitting the logs).
	if !cfg.Mute {
		message.Debug(cmd, out, errOut)
	}

	return out, err
}
