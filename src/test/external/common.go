// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package external provides a test for interacting with external resources
package external

import (
	"path"
	"path/filepath"
	"testing"

	"github.com/defenseunicorns/zarf/src/pkg/utils/exec"
	"github.com/defenseunicorns/zarf/src/test"
	"github.com/otiai10/copy"
	"github.com/stretchr/testify/require"
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
