// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExternalDataInjection(t *testing.T) {
	t.Log("E2E: External Data injection")

	tmpdir := t.TempDir()

	image := "alpine:latest"

	podYaml := fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: external-data-pod
  namespace: external-data-test
  labels:
    app: external-data-test
spec:
  containers:
    - name: alpine
      image: %s
      command: ["/bin/sh", "-c", "while true; do sleep 3600; done"]
      volumeMounts:
        - name: data
          mountPath: /data
  volumes:
    - name: data
      emptyDir: {}
`, image)

	err := os.WriteFile(filepath.Join(tmpdir, "pod.yaml"), []byte(podYaml), 0644)
	require.NoError(t, err)

	zarfYaml := fmt.Sprintf(`kind: ZarfPackageConfig
metadata:
  name: external-data
  version: 0.0.1

components:
  - name: data-pod
    required: true
    manifests:
      - name: pod
        namespace: external-data-test
        files:
          - pod.yaml
    images:
      - %s

  - name: inject-data
    required: true
    dataInjections:
      - source: "###ZARF_VAR_EXT_DATA###"
        type: external
        target:
          namespace: external-data-test
          selector: app=external-data-test
          container: alpine
          path: /data
`, image)

	err = os.WriteFile(filepath.Join(tmpdir, "zarf.yaml"), []byte(zarfYaml), 0644)
	require.NoError(t, err)

	// Create data to inject
	dataDir := filepath.Join(tmpdir, "my-data")
	err = os.Mkdir(dataDir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dataDir, "test.txt"), []byte("hello external world"), 0644)
	require.NoError(t, err)

	// Create package
	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", tmpdir, "-o", tmpdir, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	packageName := fmt.Sprintf("zarf-package-external-data-%s-0.0.1.tar.zst", e2e.Arch)
	packagePath := filepath.Join(tmpdir, packageName)

	// Deploy package with variable
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", packagePath, "--confirm", "--set", fmt.Sprintf("EXT_DATA=%s", dataDir))
	require.NoError(t, err, stdOut, stdErr)

	// Verify injection
	stdOut, stdErr, err = e2e.Kubectl(t, "-n", "external-data-test", "exec", "external-data-pod", "--", "cat", "/data/test.txt")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdOut, "hello external world")

	// Cleanup
	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", packagePath, "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}

