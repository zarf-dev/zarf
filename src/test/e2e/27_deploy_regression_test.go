// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/pkg/archive"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestGHCRDeploy(t *testing.T) {
	t.Log("E2E: GHCR OCI deploy")

	var sha string
	// shas for package published 2023-08-08T22:13:51Z
	switch e2e.Arch {
	case "arm64":
		sha = "d4f656981241366a82ef3ed2e175802043a3c5615b72cd819dd94ada27708263"
	case "amd64":
		sha = "6032b1d1029d00932fd44e3a4ac93a5ee62f0732d47b022e821c8688fc6c3c55"
	}

	// Test with command from https://docs.zarf.dev/getting-started/install/
	stdOut, stdErr, err := e2e.Zarf(t, "package", "deploy", fmt.Sprintf("oci://ghcr.io/zarf-dev/packages/dos-games:1.2.0@sha256:%s", sha), "--key=https://zarf.dev/cosign.pub", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", "dos-games", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}

func TestDeployArchitectureCheckOverride(t *testing.T) {
	for _, kind := range []api.PackageKind{api.ZarfPackageConfig, api.ZarfInitConfig} {
		t.Run(string(kind), func(t *testing.T) {
			dir := t.TempDir()
			checksumsPath := filepath.Join(dir, layout.Checksums)
			require.NoError(t, os.WriteFile(checksumsPath, nil, 0o600))
			definitionPath := filepath.Join(dir, layout.ZarfYAML)
			require.NoError(t, layout.WritePackageDefinition(definitionPath, api.Package{
				Kind:       kind,
				Metadata:   api.PackageMetadata{Name: "architecture-override", Architecture: "amd64"},
				Build:      api.BuildData{AggregateChecksum: fmt.Sprintf("%x", sha256.Sum256(nil))},
				Components: []api.Component{{Name: "application"}},
			}))
			pkgPath := filepath.Join(t.TempDir(), "package.tar")
			require.NoError(t, archive.Compress(t.Context(), []string{definitionPath, checksumsPath}, pkgPath, archive.CompressOpts{}))
			stdOut, stdErr, err := e2e.Zarf(t, "package", "deploy", pkgPath, "--skip-architecture-check", "--confirm")
			if kind == api.ZarfInitConfig {
				require.Error(t, err, stdOut, stdErr)
				require.Contains(t, stdErr, "--skip-architecture-check is not supported for init packages")
				return
			}
			require.NoError(t, err, stdOut, stdErr)
			require.NotContains(t, stdErr, "Architecture mismatch allowed.", "packages without images do not need emulation")
		})
	}
}

func TestDeployArchitectureCheckOverrideMismatch(t *testing.T) {
	const stateLookupReached = "architecture override test reached state lookup"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var body string
		switch r.URL.Path {
		case "/version":
			body = `{"gitVersion":"v1.34.0"}`
		case "/api/v1/nodes":
			body = `{"apiVersion":"v1","kind":"NodeList","items":[{"metadata":{"name":"arm64-node"},"status":{"nodeInfo":{"architecture":"arm64"}}}]}`
		case "/api/v1/pods":
			body = `{"apiVersion":"v1","kind":"PodList","items":[{"metadata":{"name":"running-pod"},"status":{"phase":"Running"}}]}`
		case "/api/v1/namespaces/zarf/secrets/zarf-state":
			// Stop after compatibility validation, before pushing or running images.
			w.WriteHeader(http.StatusForbidden)
			body = fmt.Sprintf(`{"apiVersion":"v1","kind":"Status","status":"Failure","reason":"Forbidden","code":403,"message":%q}`, stateLookupReached)
		default:
			http.NotFound(w, r)
			return
		}
		if _, err := fmt.Fprint(w, body); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	kubeconfigPath := filepath.Join(t.TempDir(), "kubeconfig")
	require.NoError(t, clientcmd.WriteToFile(clientcmdapi.Config{
		Clusters:       map[string]*clientcmdapi.Cluster{"test": {Server: server.URL}},
		Contexts:       map[string]*clientcmdapi.Context{"test": {Cluster: "test"}},
		CurrentContext: "test",
	}, kubeconfigPath))
	t.Setenv("KUBECONFIG", kubeconfigPath)

	dir := t.TempDir()
	checksumsPath := filepath.Join(dir, layout.Checksums)
	require.NoError(t, os.WriteFile(checksumsPath, nil, 0o600))
	definitionPath := filepath.Join(dir, layout.ZarfYAML)
	require.NoError(t, layout.WritePackageDefinition(definitionPath, api.Package{
		Kind:     api.ZarfPackageConfig,
		Metadata: api.PackageMetadata{Name: "architecture-override", Architecture: "amd64"},
		Build:    api.BuildData{AggregateChecksum: fmt.Sprintf("%x", sha256.Sum256(nil))},
		Components: []api.Component{{
			Name:   "application",
			Images: []api.Image{{Name: "example.com/application:latest", Source: api.ImageSourceRegistryDaemonFallback}},
		}},
	}))
	pkgPath := filepath.Join(t.TempDir(), "package.tar")
	require.NoError(t, archive.Compress(t.Context(), []string{definitionPath, checksumsPath}, pkgPath, archive.CompressOpts{}))

	stdOut, stdErr, err := e2e.Zarf(t, "package", "deploy", pkgPath, "--confirm")
	require.Error(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "this package architecture is amd64, but the target cluster only has the arm64 architecture(s)")
	require.NotContains(t, stdErr, stateLookupReached)
	require.NotContains(t, stdErr, "Architecture mismatch allowed.")

	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", pkgPath, "--skip-architecture-check", "--confirm")
	require.Error(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, stateLookupReached, "the override must allow deployment to proceed past compatibility validation")
	require.Equal(t, 1, strings.Count(stdErr, "Architecture mismatch allowed."))
	require.Contains(t, stdErr, "packageArchitecture=amd64")
	require.Contains(t, stdErr, "nodeArchitectures=[arm64]")
	require.NotContains(t, stdErr, "this package architecture is amd64")
}
