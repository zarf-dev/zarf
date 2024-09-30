// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"time"

	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/test"
)

type zarfCommandResult struct {
	stdOut string
	stdErr string
	err    error
}

func zarfCommandWStruct(t *testing.T, e2e test.ZarfE2ETest, path string) zarfCommandResult {
	result := zarfCommandResult{}
	result.stdOut, result.stdErr, result.err = e2e.Zarf(t, "package", "deploy", path, "--confirm")
	return result
}

func TestNoWait(t *testing.T) {
	t.Log("E2E: Helm Wait")

	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", "src/test/packages/28-helm-no-wait", "-o=build", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	path := fmt.Sprintf("build/zarf-package-helm-no-wait-%s.tar.zst", e2e.Arch)

	zarfChannel := make(chan zarfCommandResult, 1)
	go func() {
		zarfChannel <- zarfCommandWStruct(t, e2e, path)
	}()

	stdOut = ""
	stdErr = ""
	err = nil

	select {
	case res := <-zarfChannel:
		stdOut = res.stdOut
		stdErr = res.stdErr
		err = res.err
	case <-time.After(30 * time.Second):
		t.Error("Timeout waiting for zarf deploy (it tried to wait)")
		t.Log("Removing hanging namespace...")
		_, _, _ = e2e.Kubectl(t, "delete", "namespace", "no-wait", "--force=true", "--wait=false", "--grace-period=0") // TODO(mkcp): intentionally ignored, mark nolint
	}
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", "helm-no-wait", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}
