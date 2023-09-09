// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package external provides a test for interacting with external resources
package external

import (
	"context"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/defenseunicorns/zarf/src/pkg/utils/exec"
	"github.com/defenseunicorns/zarf/src/test"
)

var zarfBinPath = path.Join("../../../build", test.GetCLIName())

func verifyKubectlWaitSuccess(t *testing.T, timeoutMinutes time.Duration, args []string, onTimeout string) bool {
	return verifyWaitSuccess(t, timeoutMinutes, "kubectl", args, "condition met", onTimeout)
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
