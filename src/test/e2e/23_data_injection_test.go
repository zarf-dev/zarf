// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/test"
)

func TestDataInjection(t *testing.T) {
	t.Log("E2E: Data injection")

	tmpdir := t.TempDir()

	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", "examples/kiwix", "-o", tmpdir, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	ctx := logger.WithContext(context.Background(), test.GetLogger(t))
	packageName := fmt.Sprintf("zarf-package-kiwix-%s-3.5.0.tar", e2e.Arch)
	path := filepath.Join(tmpdir, packageName)

	sbomPath := filepath.Join(tmpdir, ".sbom-location")

	// Repeat the injection action 3 times to ensure the data injection is idempotent and doesn't fail to perform an upgrade
	for i := 0; i < 3; i++ {
		runDataInjection(t, path)
	}

	// Verify the file and injection marker were created
	runningKiwixPod, _, err := e2e.Kubectl(t, "--namespace=kiwix", "get", "pods", "--selector=app=kiwix-serve", "--field-selector=status.phase=Running", "--output=jsonpath={.items[0].metadata.name}")
	require.NoError(t, err)
	stdOut, stdErr, err = e2e.Kubectl(t, "--namespace=kiwix", "logs", runningKiwixPod, "--tail=5", "-c=kiwix-serve")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdOut, "devops.stackexchange.com_en_all_2023-05.zim")
	require.Contains(t, stdOut, ".zarf-injection-")

	// need target to equal svc that we are trying to connect to call checkForZarfConnectLabel
	c, err := cluster.NewCluster()
	require.NoError(t, err)
	tunnel, err := c.Connect(ctx, "kiwix")
	require.NoError(t, err)
	defer tunnel.Close()

	// Ensure connection
	resp, err := http.Get(tunnel.HTTPEndpoint())
	require.NoError(t, err, resp)
	require.Equal(t, 200, resp.StatusCode)

	// Remove the data injection example
	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Ensure that the `requirements.txt` file is discovered correctly
	stdOut, stdErr, err = e2e.Zarf(t, "package", "inspect", "sbom", path, "--output", sbomPath)
	require.NoError(t, err, stdOut, stdErr)
	require.FileExists(t, filepath.Join(sbomPath, "kiwix", "compare.html"), "A compare.html file should have been made")

	require.FileExists(t, filepath.Join(sbomPath, "kiwix", "sbom-viewer-zarf-component-kiwix-serve.html"), "The data-injection component should have an SBOM viewer")
	require.FileExists(t, filepath.Join(sbomPath, "kiwix", "zarf-component-kiwix-serve.json"), "The data-injection component should have an SBOM json")
}

func runDataInjection(t *testing.T, path string) {
	// Deploy the data injection example
	stdOut, stdErr, err := e2e.Zarf(t, "package", "deploy", path, "--confirm", "--timeout", "5m")
	require.NoError(t, err, stdOut, stdErr)
}
