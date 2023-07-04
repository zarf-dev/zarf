// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func publish(t *testing.T, path string, reg string) {
	cmd := strings.Split(fmt.Sprintf("package publish %s oci://%s", path, reg), " ")
	_, _, err := e2e.Zarf(cmd...)
	require.NoError(t, err)
}

func TestBundle(t *testing.T) {
	e2e.SetupDockerRegistry(t, 888)
	e2e.SetupDockerRegistry(t, 889)

	ver := "0.0.1"
	arch := e2e.Arch
	pkg := fmt.Sprintf("build/zarf-package-dos-games-%s-%s.tar.zst", arch, ver)
	publish(t, pkg, "localhost:888")

	pkg = fmt.Sprintf("build/zarf-package-remote-manifests-%s-%s.tar.zst", arch, ver)
	publish(t, pkg, "localhost:889")

	dir := "src/test/packages/60-bundle"
	cmd := strings.Split(fmt.Sprintf("bundle create %s -o oci://%s --confirm", dir, "localhost:888"), " ")
	_, _, err := e2e.Zarf(cmd...)
	require.NoError(t, err)
}
