// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/defenseunicorns/zarf/src/types"
	"github.com/stretchr/testify/require"
)

// TestMismatchedVersions ensures that zarf produces a warning
// when the initialized version of Zarf doesn't match the current CLI
func TestMismatchedVersions(t *testing.T) {
	t.Log("E2E: Mismatched versions")
	e2e.SetupWithCluster(t)

	var (
		expectedWarningMessage = "Potential Breaking Changes"
	)

	// Get the current init package secret
	initPkg := types.DeployedPackage{}
	base64Pkg, _, err := e2e.Kubectl("get", "secret", "zarf-package-init", "-n", "zarf", "-o", "jsonpath={.data.data}")
	require.NoError(t, err)
	jsonPkg, err := base64.StdEncoding.DecodeString(base64Pkg)
	require.NoError(t, err)
	fmt.Println(string(jsonPkg))
	err = json.Unmarshal(jsonPkg, &initPkg)
	require.NoError(t, err)

	// Edit the build data to trigger the breaking change check
	initPkg.Data.Build.Version = "v0.25.0"

	// Delete the package secret
	_, _, err = e2e.Kubectl("delete", "secret", "zarf-package-init", "-n", "zarf")
	require.NoError(t, err)

	// Create a new secret with the modified data
	jsonPkgModified, err := json.Marshal(initPkg)
	require.NoError(t, err)
	_, _, err = e2e.Kubectl("create", "secret", "generic", "zarf-package-init", "-n", "zarf", fmt.Sprintf("--from-literal=data=%s", string(jsonPkgModified)))
	require.NoError(t, err)

	path := filepath.Join("build", fmt.Sprintf("zarf-package-dos-games-%s-1.0.0.tar.zst", e2e.Arch))

	// Deploy the games package
	stdOut, stdErr, err := e2e.Zarf("package", "deploy", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, expectedWarningMessage)

	// Remove the games package
	stdOut, stdErr, err = e2e.Zarf("package", "remove", "dos-games", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Reset the package secret
	_, _, err = e2e.Kubectl("delete", "secret", "zarf-package-init", "-n", "zarf")
	require.NoError(t, err)
	_, _, err = e2e.Kubectl("create", "secret", "generic", "zarf-package-init", "-n", "zarf", fmt.Sprintf("--from-literal=data=%s", string(jsonPkg)))
	require.NoError(t, err)
}
