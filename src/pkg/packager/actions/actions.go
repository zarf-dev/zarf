// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package actions contains functions for running component actions within Zarf packages.
package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/goccy/go-yaml"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	ptmpl "github.com/zarf-dev/zarf/src/internal/packager/template"
	"github.com/zarf-dev/zarf/src/internal/template"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/utils/exec"
	"github.com/zarf-dev/zarf/src/pkg/value"
	"github.com/zarf-dev/zarf/src/pkg/variables"
	"github.com/zarf-dev/zarf/src/pkg/wait"
)

// Run runs all provided actions.
func Run(ctx context.Context, basePath string, defaultCfg v1alpha1.ZarfComponentActionDefaults, actions []v1alpha1.ZarfComponentAction, variableConfig *variables.VariableConfig, values value.Values) error {
	if variableConfig == nil {
		variableConfig = ptmpl.GetZarfVariableConfig(ctx, false)
	}

	for _, a := range actions {
		if err := runAction(ctx, basePath, defaultCfg, a, variableConfig, values); err != nil {
			return err
		}
	}
	return nil
}

// Run commands that a component has provided.
func runAction(ctx context.Context, basePath string, defaultCfg v1alpha1.ZarfComponentActionDefaults, action v1alpha1.ZarfComponentAction, variableConfig *variables.VariableConfig, values value.Values) error {
	var cmdEscaped string
	var err error
	cmd := action.Cmd
	l := logger.From(ctx)
	start := time.Now()

	if action.Wait != nil {
		err := runWaitAction(ctx, action)
		if err != nil {
			return err
		}
		l.Debug("wait action succeeded", "duration", time.Since(start))
		return nil
	}

	tmplObjs := template.NewObjects(values).
		WithConstants(variableConfig.GetConstants()).
		WithVariables(variableConfig.GetSetVariableMap())

	if action.Description != "" {
		cmdEscaped = action.Description
	} else {
		cmdEscaped = helpers.Truncate(cmd, 60, false)
	}

	// Apply go-templates in cmds if templating is enabled
	if action.ShouldTemplate() {
		cmd, err = template.Apply(ctx, cmd, tmplObjs)
		if err != nil {
			return fmt.Errorf("could not template cmd %s: %w", cmdEscaped, err)
		}
	}

	l.Info("running command", "cmd", cmdEscaped)

	actionDefaults := actionGetCfg(ctx, defaultCfg, action, variableConfig.GetAllTemplates())
	actionDefaults.Dir = filepath.Join(basePath, actionDefaults.Dir)

	if cmd, err = actionCmdMutation(ctx, cmd, actionDefaults.Shell, runtime.GOOS); err != nil {
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
			stdout, _, err := actionRun(ctx, actionDefaults, cmd)
			if err != nil {
				return err
			}
			l.Info("action succeeded", "cmd", cmdEscaped)

			outTrimmed := strings.TrimSpace(stdout)

			// If an output variable is defined, set it.
			for _, v := range action.SetVariables {
				variableConfig.SetVariable(v.Name, outTrimmed, v.Sensitive, v.AutoIndent, v.Type)
				if err := variableConfig.CheckVariablePattern(v.Name, v.Pattern); err != nil {
					return err
				}
			}

			// If an output value is defined, parse the result and set it to values map.
			for _, v := range action.SetValues {
				if err := parseAndSetValue(outTrimmed, v, values); err != nil {
					return err
				}
			}

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
		l.Info("waiting for action", "cmd", cmdEscaped, "timeout", fmt.Sprintf("%d seconds", actionDefaults.MaxTotalSeconds))
		select {
		// On timeout break the loop to abort.
		case <-timeout:
			break retryCmd

		// Otherwise, try running the command.
		default:
			ctx, cancel := context.WithTimeout(ctx, duration)
			defer cancel()
			if err := tryCmd(ctx); err != nil {
				l.Warn("action failed", "cmd", cmdEscaped, "err", err.Error())
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

func runWaitAction(ctx context.Context, action v1alpha1.ZarfComponentAction) error {
	waitCfg := action.Wait

	timeout := 5 * time.Minute
	if action.MaxTotalSeconds != nil && *action.MaxTotalSeconds > 0 {
		timeout = time.Duration(*action.MaxTotalSeconds) * time.Second
	}

	if waitCfg.Cluster != nil {
		return runWaitClusterAction(ctx, waitCfg.Cluster, timeout)
	} else if waitCfg.Network != nil {
		return runWaitNetworkAction(ctx, waitCfg.Network, timeout)
	}
	return fmt.Errorf("wait action is missing a cluster or network")
}

func runWaitClusterAction(ctx context.Context, cluster *v1alpha1.ZarfComponentActionWaitCluster, timeout time.Duration) error {
	l := logger.From(ctx)

	kind := cluster.Kind
	identifier := cluster.Name
	condition := cluster.Condition
	namespace := cluster.Namespace

	desc := fmt.Sprintf("wait for %s/%s", kind, identifier)
	if condition != "" {
		desc = fmt.Sprintf("%s to be %s", desc, condition)
	}
	l.Info("running wait action", "description", desc)

	return wait.ForResource(ctx, namespace, condition, kind, identifier, timeout)
}

func runWaitNetworkAction(ctx context.Context, network *v1alpha1.ZarfComponentActionWaitNetwork, timeout time.Duration) error {
	l := logger.From(ctx)

	kind := strings.ToLower(network.Protocol)
	identifier := network.Address
	var condition string
	if strings.HasPrefix(kind, "http") && network.Code == 0 {
		condition = "200"
	} else if network.Code != 0 {
		condition = strconv.Itoa(network.Code)
	}

	desc := fmt.Sprintf("wait for %s/%s", kind, identifier)
	if condition != "" {
		desc = fmt.Sprintf("%s to be %s", desc, condition)
	}
	l.Info("running wait action", "description", desc)

	return wait.ForNetwork(ctx, kind, identifier, condition, timeout)
}

// Perform some basic string mutations to make commands more useful.
func actionCmdMutation(ctx context.Context, cmd string, shellPref v1alpha1.Shell, goos string) (string, error) {
	zarfCommand, err := utils.GetFinalExecutableCommand()
	if err != nil {
		return cmd, err
	}

	// Try to patch the zarf binary path in case the name isn't exactly "./zarf".
	cmd = strings.ReplaceAll(cmd, "./zarf ", zarfCommand+" ")

	// Make commands 'more' compatible with Windows OS PowerShell
	if goos == "windows" && (exec.IsPowershell(shellPref.Windows) || shellPref.Windows == "") {
		// Replace "touch" with "New-Item" on Windows as it's a common command, but not POSIX so not aliased by M$.
		// See https://mathieubuisson.github.io/powershell-linux-bash/ &
		// http://web.cs.ucla.edu/~miryung/teaching/EE461L-Spring2012/labs/posix.html for more details.
		cmd = regexp.MustCompile(`^touch `).ReplaceAllString(cmd, `New-Item `)

		// Convert any ${ZARF_VAR_*} or $ZARF_VAR_* to ${env:ZARF_VAR_*} or $env:ZARF_VAR_* respectively
		// (also TF_VAR_* and ZARF_CONST_).
		// https://regex101.com/r/xk1rkw/1
		envVarRegex := regexp.MustCompile(`(?P<envIndicator>\${?(?P<varName>(ZARF|TF)_(VAR|CONST)_([a-zA-Z0-9_-])+)}?)`)
		getFunctions := MatchAllRegex(envVarRegex, cmd)

		newCmd := cmd
		for _, get := range getFunctions {
			newCmd = strings.ReplaceAll(newCmd, get("envIndicator"), fmt.Sprintf("$Env:%s", get("varName")))
		}
		if newCmd != cmd {
			logger.From(ctx).Debug("converted command", "cmd", cmd, "newCmd", newCmd)
		}
		cmd = newCmd
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
	start := time.Now()
	shell, shellArgs := exec.GetOSShell(cfg.Shell)

	l.Debug("running command", "shell", shell, "cmd", cmd)

	execCfg := exec.Config{
		Env:   cfg.Env,
		Dir:   cfg.Dir,
		Print: !cfg.Mute,
	}

	stdout, stderr, err := exec.CmdWithContext(ctx, execCfg, shell, append(shellArgs, cmd)...)
	// Dump final complete output (respect mute to prevent sensitive values from hitting the logs).
	if !cfg.Mute {
		l.Debug("command complete", "stdout", stdout, "stderr", stderr, "duration", time.Since(start))
	}
	return stdout, stderr, err
}

// MatchAllRegex wraps a get function around each substring match, returning all matches.
func MatchAllRegex(regex *regexp.Regexp, str string) []func(string) string {
	// Validate the string.
	matches := regex.FindAllStringSubmatch(str, -1)

	// Parse the string into its components.
	var funcs []func(string) string
	for _, match := range matches {
		funcs = append(funcs, func(name string) string {
			return match[regex.SubexpIndex(name)]
		})
	}
	return funcs
}

// parseAndSetValue parses the output string according to the setValue type and sets it in the values map.
func parseAndSetValue(output string, setValue v1alpha1.SetValue, values value.Values) error {
	var val any
	switch setValue.Type {
	case v1alpha1.SetValueYAML:
		var parsed any
		if err := yaml.Unmarshal([]byte(output), &parsed); err != nil {
			return fmt.Errorf("failed to parse YAML output for setValue %q: %w", setValue.Key, err)
		}
		val = parsed
	case v1alpha1.SetValueJSON:
		var parsed any
		if err := json.Unmarshal([]byte(output), &parsed); err != nil {
			return fmt.Errorf("failed to parse JSON output for setValue %q: %w", setValue.Key, err)
		}
		val = parsed
	case v1alpha1.SetValueString, "":
		// Empty Type behaves as v1alpha1.SetValueString
		val = output
	default:
		return fmt.Errorf("unknown setValue type %q for key %q", setValue.Type, setValue.Key)
	}
	return values.Set(value.Path(setValue.Key), val)
}
