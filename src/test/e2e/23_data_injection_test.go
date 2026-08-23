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

	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", "src/test/packages/23-data-injections", "-o", tmpdir, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	ctx := logger.WithContext(context.Background(), test.GetLogger(t))
	packageName := fmt.Sprintf("zarf-package-data-injection-%s-1.0.0.tar", e2e.Arch)
	path := filepath.Join(tmpdir, packageName)

	sbomPath := filepath.Join(tmpdir, ".sbom-location")

	// Repeat the injection action 3 times to ensure the data injection is idempotent and doesn't fail to perform an upgrade
	for i := 0; i < 3; i++ {
		runDataInjection(t, path)
	}

	// Verify the file and injection marker were created
	runningPod, _, err := e2e.Kubectl(t, "--namespace=data-injection", "get", "pods", "--selector=app=file-server", "--field-selector=status.phase=Running", "--output=jsonpath={.items[0].metadata.name}")
	require.NoError(t, err)
	stdOut, stdErr, err = e2e.Kubectl(t, "--namespace=data-injection", "logs", runningPod, "--tail=50", "-c=file-server")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdOut, "hello.txt")
	require.Contains(t, stdOut, ".zarf-injection-")

	// need target to equal svc that we are trying to connect to call checkForZarfConnectLabel
	c, err := cluster.New(ctx)
	require.NoError(t, err)
	tunnel, err := c.Connect(ctx, "file-server")
	require.NoError(t, err)
	defer tunnel.Close()

	endpoints := tunnel.HTTPEndpoints()
	require.Len(t, endpoints, 1)

	// Ensure connection
	resp, err := http.Get(endpoints[0] + "/hello.txt")
	require.NoError(t, err, resp)
	require.Equal(t, 200, resp.StatusCode)

	// Remove the data injection example
	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Zarf(t, "package", "inspect", "sbom", path, "--output", sbomPath)
	require.NoError(t, err, stdOut, stdErr)

	require.FileExists(t, filepath.Join(sbomPath, "data-injection", "sbom-viewer-zarf-component-file-server.html"), "The data-injection component should have an SBOM viewer")
	require.FileExists(t, filepath.Join(sbomPath, "data-injection", "zarf-component-file-server.json"), "The data-injection component should have an SBOM json")
}

func runDataInjection(t *testing.T, path string) {
	// Deploy the data injection example
	stdOut, stdErr, err := e2e.Zarf(t, "package", "deploy", path, "--confirm", "--timeout", "5m")
	require.NoError(t, err, stdOut, stdErr)
}
