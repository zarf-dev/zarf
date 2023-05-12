// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/defenseunicorns/zarf/src/pkg/utils/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDataInjection(t *testing.T) {
	t.Log("E2E: Data injection")
	e2e.SetupWithCluster(t)
	defer e2e.Teardown(t)

	path := fmt.Sprintf("build/zarf-package-data-injection-demo-%s.tar", e2e.Arch)

	sbomPath := filepath.Join(os.TempDir(), ".sbom-location")

	e2e.CleanFiles(sbomPath)

	// Repeat the injection action 3 times to ensure the data injection is idempotent and doesn't fail to perform an upgrade
	for i := 0; i < 3; i++ {
		runDataInjection(t, path)
	}

	// Verify the file and injection marker were created
	stdOut, stdErr, err := exec.CmdWithContext(context.TODO(), exec.PrintCfg(), "kubectl", "--namespace=demo", "logs", "--tail=5", "--selector=app=data-injection", "-c=data-injection")
	require.NoError(t, err, stdOut, stdErr)
	assert.Contains(t, stdOut, "this-is-an-example-file.txt")
	assert.Contains(t, stdOut, ".zarf-injection-")

	stdOut, stdErr, err = e2e.ExecZarfCommand("package", "remove", "data-injection-demo", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Ensure that the `requirements.txt` file is discovered correctly
	stdOut, stdErr, err = e2e.ExecZarfCommand("package", "inspect", path, "--sbom-out", sbomPath)
	require.NoError(t, err, stdOut, stdErr)
	_, err = os.ReadFile(filepath.Join(sbomPath, "data-injection-demo", "compare.html"))
	require.NoError(t, err)
	_, err = os.ReadFile(filepath.Join(sbomPath, "data-injection-demo", "sbom-viewer-zarf-component-with-init-container.html"))
	require.NoError(t, err)
	withInitContainerJson, err := os.ReadFile(filepath.Join(sbomPath, "data-injection-demo", "zarf-component-with-init-container.json"))
	require.NoError(t, err)
	require.Contains(t, string(withInitContainerJson), "pytz")

	e2e.CleanFiles(sbomPath)
}

func runDataInjection(t *testing.T, path string) {
	// Limit this deploy to 5 minutes
	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Minute)
	defer cancel()

	// Deploy the data injection example
	stdOut, stdErr, err := exec.CmdWithContext(ctx, exec.PrintCfg(), e2e.ZarfBinPath, "package", "deploy", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}
