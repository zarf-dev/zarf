// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package external provides a test for interacting with external resources
package external

import (
	"errors"
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

func createPodInfoPackageWithInsecureSources(t *testing.T, packageDir string) {
	temp := t.TempDir()
	err := copy.Copy("../../../examples/podinfo-flux", packageDir)
	require.NoError(t, err)
	// This is done because while .spec.insecure is auto set to true for internal registries by the agent
	// it is not for external registries, however since we are using an insecure external registry, we still need it
	err = exec.CmdWithPrint(zarfBinPath, "tools", "yq", "eval", ".spec.insecure = true", "-i", filepath.Join(packageDir, "helm", "podinfo-source.yaml"))
	require.NoError(t, err, "unable to yq edit helm source")
	err = exec.CmdWithPrint(zarfBinPath, "tools", "yq", "eval", ".spec.insecure = true", "-i", filepath.Join(packageDir, "oci", "podinfo-source.yaml"))
	require.NoError(t, err, "unable to yq edit oci source")
	// avoiding Zarf cache because of flake https://github.com/zarf-dev/zarf/issues/3194
	err = exec.CmdWithPrint(zarfBinPath, "package", "create", packageDir, "--confirm", "--output", packageDir, "--zarf-cache", temp)
	require.NoError(t, err, "unable to create package")
}

func waitForCondition(t *testing.T, timeoutMinutes time.Duration, cmd string, args []string, condition string) error {
	timeout := time.After(timeoutMinutes * time.Minute)
	for {
		// delay check 3 seconds
		time.Sleep(3 * time.Second)
		select {
		// on timeout abort
		case <-timeout:
			return errors.New("timed out waiting for condition")

			// after delay, try running
		default:
			// Check information from the given command
			stdOut, _, err := exec.CmdWithTesting(t, exec.PrintCfg(), cmd, args...)
			// Log error
			if err != nil {
				t.Log(string(stdOut), err)
			}
			if strings.Contains(string(stdOut), condition) {
				return nil
			}
		}
	}
}
