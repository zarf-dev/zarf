// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package external provides a test for interacting with external resources
package external

import (
	"context"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/otiai10/copy"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/utils/exec"
	"github.com/zarf-dev/zarf/src/test"
)

var zarfBinPath = path.Join("../../../build", test.GetCLIName())

func createPodInfoPackageWithInsecureSources(t *testing.T, temp string) {
	err := copy.Copy("../../../examples/podinfo-flux", temp)
	require.NoError(t, err)
	// This is done because while .spec.insecure is auto set to true for internal registries by the agent
	// it is not for external registries, however since we are using an insecure external registry, we still need it
	err = exec.CmdWithPrint(zarfBinPath, "tools", "yq", "eval", ".spec.insecure = true", "-i", filepath.Join(temp, "helm", "podinfo-source.yaml"))
	require.NoError(t, err, "unable to yq edit helm source")
	err = exec.CmdWithPrint(zarfBinPath, "tools", "yq", "eval", ".spec.insecure = true", "-i", filepath.Join(temp, "oci", "podinfo-source.yaml"))
	require.NoError(t, err, "unable to yq edit oci source")
	exec.CmdWithPrint(zarfBinPath, "package", "create", temp, "--confirm", "--output", temp)
}

func verifyWaitSuccess(t *testing.T, timeoutMinutes time.Duration, cmd string, args []string, condition string, onTimeout string) bool {
	timeout := time.After(timeoutMinutes * time.Minute)
	for {
		// delay check 3 seconds
		time.Sleep(3 * time.Second)
		select {
		// on timeout abort
		case <-timeout:
			t.Error(onTimeout)

			return false

			// after delay, try running
		default:
			// Check information from the given command
			stdOut, _, err := exec.CmdWithContext(context.TODO(), exec.PrintCfg(), cmd, args...)
			// Log error
			if err != nil {
				t.Log(string(stdOut), err)
			}
			if strings.Contains(string(stdOut), condition) {
				return true
			}
		}
	}
}
