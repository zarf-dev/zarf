// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestHelmAdoptionAgentLabel verifies that --take-ownership handles
// the zarf.dev/agent:ignore label correctly depending on whether the package
// defines the label in a namespace Helm template.
func TestHelmAdoptionAgentLabel(t *testing.T) {
	t.Log("E2E: Helm adopt namespace agent label handling")

	tmpdir := t.TempDir()

	c, err := cluster.New(t.Context())
	require.NoError(t, err)

	t.Run("changes agent ignore label to mutate label when namespace is not explicitly defined", func(t *testing.T) {
		t.Parallel()
		noTemplateSrc := filepath.Join("src", "test", "packages", "49-adopt-no-ns-template")
		_, stdErr, err := e2e.Zarf(t, "package", "create", noTemplateSrc, "-o", tmpdir, "--skip-sbom", "--confirm")
		require.NoError(t, err, stdErr)
		noTemplatePkg := filepath.Join(tmpdir, fmt.Sprintf("zarf-package-adopt-no-ns-template-%s-0.1.0.tar.zst", e2e.Arch))

		const ns = "adopt-no-ns-template"
		_, _, err = e2e.Kubectl(t, "create", "namespace", ns)
		require.NoError(t, err)
		_, _, err = e2e.Kubectl(t, "label", "namespace", ns, "zarf.dev/agent=ignore")
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _, err := e2e.Zarf(t, "package", "remove", "adopt-no-ns-template", "--confirm")
			require.NoError(t, err)
			_, _, err = e2e.Kubectl(t, "delete", "namespace", ns, "--ignore-not-found", "--grace-period=0")
			require.NoError(t, err)
		})

		_, stdErr, err = e2e.Zarf(t, "package", "deploy", noTemplatePkg, "--confirm", "--take-ownership")
		require.NoError(t, err, stdErr)

		namespace, err := c.Clientset.CoreV1().Namespaces().Get(t.Context(), ns, metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, "mutate", namespace.Labels[cluster.AgentLabel])
	})

	t.Run("keeps agent ignore label when explicitly defined by the package", func(t *testing.T) {
		t.Parallel()
		withTemplateSrc := filepath.Join("src", "test", "packages", "49-adopt-ns-with-agent-ignore")
		_, stdErr, err := e2e.Zarf(t, "package", "create", withTemplateSrc, "-o", tmpdir, "--skip-sbom", "--confirm")
		require.NoError(t, err, stdErr)
		withTemplatePkg := filepath.Join(tmpdir, fmt.Sprintf("zarf-package-adopt-ns-with-agent-ignore-%s-0.1.0.tar.zst", e2e.Arch))

		const ns = "adopt-ns-with-agent-ignore"
		_, _, err = e2e.Kubectl(t, "create", "namespace", ns)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _, err := e2e.Zarf(t, "package", "remove", "adopt-ns-with-agent-ignore", "--confirm")
			require.NoError(t, err)
			_, _, err = e2e.Kubectl(t, "delete", "namespace", ns, "--ignore-not-found", "--grace-period=0")
			require.NoError(t, err)
		})

		_, stdErr, err = e2e.Zarf(t, "package", "deploy", withTemplatePkg, "--confirm", "--take-ownership")
		require.NoError(t, err, stdErr)

		namespace, err := c.Clientset.CoreV1().Namespaces().Get(t.Context(), ns, metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, "ignore", namespace.Labels[cluster.AgentLabel])
	})
}
