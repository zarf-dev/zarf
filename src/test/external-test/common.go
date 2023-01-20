// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package external_test provides a test for the external init flow.
package external_test

import (
	"os/exec"
	"strings"
	"testing"
	"time"
)

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
			out, err := exec.Command(cmd, args...).Output()
			// Log error
			if err != nil {
				t.Log(string(out), err)
			}
			if strings.Contains(string(out), condition) {
				return true
			}
		}
	}
}
