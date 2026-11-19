// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHelmPostRenderersDeploy(t *testing.T) {
	t.Log("E2E: Deploy Helm charts with post-renderers")
	tmpdir := t.TempDir()
	fixture := filepath.Join("src", "test", "packages", "16-helm-post-renderers")

	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", fixture, "-o", tmpdir, "--skip-sbom", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	packagePath := filepath.Join(tmpdir, fmt.Sprintf("zarf-package-helm-post-renderers-%s.tar.zst", e2e.Arch))

	const nohooksNS, separateNS = "post-render-nohooks", "post-render-separate"
	// Zarf leaves the namespaces it creates behind on remove.
	t.Cleanup(func() {
		_, _, err := e2e.Kubectl(t, "delete", "namespace", nohooksNS, separateNS, "--ignore-not-found")
		require.NoError(t, err)
	})

	// The package requires v1.1.0, which development builds of the CLI may not report yet.
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", packagePath, "--confirm", "--skip-version-check", "--set-values", "team=from-deploy")
	require.NoError(t, err, stdOut, stdErr)
	deployed := true
	t.Cleanup(func() {
		if deployed {
			stdOut, stdErr, err := e2e.Zarf(t, "package", "remove", "helm-post-renderers", "--confirm", "--skip-version-check")
			require.NoError(t, err, stdOut, stdErr)
		}
	})

	requireJSONPath := func(t *testing.T, namespace, resource, path, expected string) {
		t.Helper()
		out, stdErr, err := e2e.Kubectl(t, "get", resource, "-n", namespace, "-o", "jsonpath="+path)
		require.NoError(t, err, stdErr)
		require.Equal(t, expected, out, "%s/%s %s", namespace, resource, path)
	}

	// Strategic-merge and JSON 6902 patches, and the image retarget, reach the live Deployment.
	requireJSONPath(t, nohooksNS, "deployment/app", `{.metadata.labels.post-render\.zarf\.dev/smp}`, "patched")
	requireJSONPath(t, nohooksNS, "deployment/app", `{.metadata.annotations.post-render\.zarf\.dev/team}`, "from-deploy")
	requireJSONPath(t, nohooksNS, "deployment/app", "{.spec.template.spec.securityContext.seccompProfile.type}", "RuntimeDefault")
	requireJSONPath(t, nohooksNS, "deployment/app", `{.spec.template.spec.containers[0].env[?(@.name=="PATCHED_BY")].value}`, "json6902")
	requireJSONPath(t, nohooksNS, "deployment/app", "{.status.readyReplicas}", "1")
	// The agent rewrites the image to the Zarf registry, so check the retargeted tag survived that rewrite.
	image, stdErr, err := e2e.Kubectl(t, "get", "deployment/app", "-n", nohooksNS, "-o", "jsonpath={.spec.template.spec.containers[0].image}")
	require.NoError(t, err, stdErr)
	require.Contains(t, image, "zarf-dev/images/alpine:3.21.3")

	requireHooks := func(t *testing.T) {
		t.Helper()
		// nohooks: the hook Job runs, but without the patch.
		requireJSONPath(t, nohooksNS, "job/app-hook", "{.status.succeeded}", "1")
		requireJSONPath(t, nohooksNS, "job/app-hook", `{.metadata.labels.post-render\.zarf\.dev/hook}`, "")
		// separate: the hook Job is patched, and it only succeeds because its image was retargeted.
		requireJSONPath(t, separateNS, "job/app-hook", "{.status.succeeded}", "1")
		requireJSONPath(t, separateNS, "job/app-hook", `{.metadata.labels.post-render\.zarf\.dev/hook}`, "patched")
		requireJSONPath(t, separateNS, "deployment/app", `{.metadata.labels.post-render\.zarf\.dev/smp}`, "patched")
	}
	requireHooks(t)

	// Upgrading with a new value re-renders the templated post-renderer.
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", packagePath, "--confirm", "--skip-version-check", "--set-values", "team=from-upgrade")
	require.NoError(t, err, stdOut, stdErr)
	requireJSONPath(t, nohooksNS, "deployment/app", `{.metadata.annotations.post-render\.zarf\.dev/team}`, "from-upgrade")
	requireJSONPath(t, nohooksNS, "deployment/app", `{.metadata.labels.post-render\.zarf\.dev/smp}`, "patched")
	requireHooks(t)
	stdOut, stdErr, err = e2e.Zarf(t, "tools", "helm", "history", "nohooks-app", "-n", nohooksNS)
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdOut, "Upgrade complete")

	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", "helm-post-renderers", "--confirm", "--skip-version-check")
	require.NoError(t, err, stdOut, stdErr)
	deployed = false
	_, _, err = e2e.Kubectl(t, "get", "deployment/app", "-n", nohooksNS)
	require.Error(t, err)
}
