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
	"github.com/defenseunicorns/zarf/src/test"
	"github.com/stretchr/testify/require"
)

type zarfCommandResult struct {
	stdOut string
	stdErr string
	err    error
}

func zarfCommandWStruct(e2e test.ZarfE2ETest, path string) (result zarfCommandResult) {
	result.stdOut, result.stdErr, result.err = e2e.Zarf("package", "deploy", path, "--confirm")
	return result
}

func TestWait(t *testing.T) {
	t.Log("E2E: Helm Wait")
	e2e.SetupWithCluster(t)
	defer e2e.Teardown(t)

	// Create the package.
	stdOut, stdErr, err := e2e.ZarfWithConfirm("package", "create", "src/test/packages/28-helm-no-wait/", "-o=build")
	require.NoError(t, err, stdOut, stdErr)

	path := fmt.Sprintf("build/zarf-package-helm-no-wait-%s.tar.zst", e2e.Arch)

	zarfChannel := make(chan zarfCommandResult, 1)
	go func() {
		zarfChannel <- zarfCommandWStruct(e2e, path)
	}()

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

	stdOut, stdErr, err = e2e.Zarf("package", "remove", "helm-no-wait", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}
