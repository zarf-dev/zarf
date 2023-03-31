// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	// "context"
	"fmt"
	"os/exec"
	"time"

	// "os/exec"
	"testing"

	// "github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type zarfCommandResult struct {
	stdOut string
	stdErr string
	err    error
}

func zarfCommandWStruct(e2e ZarfE2ETest, path string) (result zarfCommandResult) {
	result.stdOut, result.stdErr, result.err = e2e.execZarfCommand("package", "deploy", path, "--confirm")
	return result
}

func TestWait(t *testing.T) {
	t.Log("E2E: Helm Wait")
	e2e.setupWithCluster(t)
	defer e2e.teardown(t)

	path := fmt.Sprintf("build/zarf-package-test-helm-wait-%s.tar.zst", e2e.arch)

	zarfChannel := make(chan zarfCommandResult, 1)
	go func() {
		zarfChannel <- zarfCommandWStruct(e2e, path)
	}()

	var stdOut string
	var stdErr string
	var err error

	select {
	case res := <-zarfChannel:
		stdOut = res.stdOut
		stdErr = res.stdErr
		err = res.err
	case <-time.After(30 * time.Second):
		t.Error("Timeout waiting for zarf deploy (it tried to wait)")
		t.Log("Removing hanging namespace...")
		kubectlOut, err := exec.Command("kubectl", "delete", "namespace", "no-wait", "--force=true", "--wait=false", "--grace-period=0").Output()
		if err != nil {
			t.Log(kubectlOut)
		} else {
			panic(err)
		}
	}
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.execZarfCommand("package", "remove", "test-helm-wait", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}
