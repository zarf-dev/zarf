// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package actions contains functions for running component actions within Zarf packages.
package actions

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/variables"
	"github.com/defenseunicorns/zarf/src/types"
)

type runOptions struct {
	action types.ZarfComponentAction
	vc     *variables.VariableConfig
}

func Run(ctx context.Context, actions []types.ZarfComponentAction, actionPhase types.ActionPhase, vc *variables.VariableConfig) error {
	for _, action := range actions {
		if action.Phase() == actionPhase {
			err := run(ctx, runOptions{action, vc})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func run(ctx context.Context, opts runOptions) (err error) {
	action := opts.action
	vc := opts.vc

	if action.Cmd == "" && action.Wait == nil {
		return errors.New("invalid action. The command must be a non-empty string or a wait action must be used")
	}

	var cmdargs []string
	switch runtime.GOOS {
	case "windows":
		cmdargs = []string{"cmd", "/C"}
	default:
		cmdargs = []string{"/bin/sh", "-c", "-e"}
	}

	if len(action.Interpreter) > 0 {
		cmdargs = action.Interpreter
	}

	if action.Wait != nil {
		action.Cmd, err = convertWaitToCmd(action.Wait)
		if err != nil {
			return err
		}
	}

	// Patch the zarf binary path
	zarfCommand, err := utils.GetFinalExecutableCommand()
	if err != nil {
		return err
	}
	action.Cmd = strings.ReplaceAll(action.Cmd, "./zarf ", zarfCommand+" ")

	cmdargs = append(cmdargs, action.Cmd)

	cmdEnv := os.Environ()
	for k, v := range action.Env {
		cmdEnv = append(cmdEnv, fmt.Sprintf("%s=%s", k, v))
	}

	if vc != nil {
		for k, v := range vc.GetAllTemplates() {
			// Remove # from env variable name.
			k = strings.ReplaceAll(k, "#", "")
			// Make terraform variables available to the action as TF_VAR_lowercase_name.
			k1 := strings.ReplaceAll(strings.ToLower(k), "zarf_var", "TF_VAR")
			cmdEnv = append(cmdEnv, fmt.Sprintf("%s=%s", k, v.Value))
			cmdEnv = append(cmdEnv, fmt.Sprintf("%s=%s", k1, v.Value))
		}
	}

	cmd := exec.CommandContext(ctx, cmdargs[0], cmdargs[1:]...)
	absPath, err := filepath.Abs(action.Dir)
	if err != nil {
		return err
	}
	cmd.Dir = absPath
	cmd.Env = cmdEnv

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	stdoutMulti := io.MultiWriter(os.Stdout, &stdoutBuf)
	stderrMulti := io.MultiWriter(os.Stderr, &stderrBuf)
	cmd.Stdout = stdoutMulti
	cmd.Stderr = stderrMulti

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("%s: %w", stderrBuf.String(), err)
	}

	for _, v := range action.SetVariables {
		vc.SetVariable(v.Name, strings.TrimSpace(stdoutBuf.String()), v.Sensitive, v.AutoIndent, v.Type)
		if err := vc.CheckVariablePattern(v.Name, v.Pattern); err != nil {
			return err
		}
	}

	return nil
}

// convertWaitToCmd will return the wait command if it exists, otherwise it will return the original command.
func convertWaitToCmd(wait *types.ZarfComponentActionWait) (string, error) {
	cluster := wait.Cluster
	network := wait.Network

	if cluster == nil && network == nil {
		return "", errors.New("wait action is missing a cluster or network")
	}

	if cluster != nil {
		ns := cluster.Namespace
		if ns != "" {
			ns = fmt.Sprintf("-n %s", ns)
		}
		return fmt.Sprintf("./zarf tools wait-for %s %s %s %s",
			cluster.Kind, cluster.Identifier, cluster.Condition, ns), nil
	}

	if network != nil {
		network.Protocol = strings.ToLower(network.Protocol)
		if strings.HasPrefix(network.Protocol, "http") && network.Code == 0 {
			network.Code = 200
		}
		return fmt.Sprintf("./zarf tools wait-for %s %s %d",
			network.Protocol, network.Address, network.Code), nil
	}

	return "", nil
}
