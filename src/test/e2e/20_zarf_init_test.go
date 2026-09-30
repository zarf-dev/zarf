// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/pki"
	"github.com/zarf-dev/zarf/src/pkg/state"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

func TestInitGitTLSChartCreatesSecret(t *testing.T) {
	render := func(t *testing.T, valuesFile string) corev1.Secret {
		t.Helper()
		args := []string{"tools", "helm", "template", "zarf-git-server-tls", "packages/gitea/tls-chart", "--namespace", "zarf"}
		if valuesFile != "" {
			args = append(args, "-f", valuesFile)
		}
		command := exec.Command(e2e.ZarfBinPath, args...)
		stdout, err := command.Output()
		require.NoError(t, err)
		secret := corev1.Secret{}
		require.NoError(t, yaml.Unmarshal(stdout, &secret))
		require.Equal(t, state.GitServerTLSSecret, secret.Name)
		return secret
	}

	t.Run("generated certificate covers Git clients", func(t *testing.T) {
		secret := render(t, "")
		_, err := tls.X509KeyPair(secret.Data[state.GitServerTLSCertKey], secret.Data[state.GitServerTLSKey])
		require.NoError(t, err)
		cert, err := pki.ParseCertFromPEM(secret.Data[state.GitServerTLSCertKey])
		require.NoError(t, err)
		roots := x509.NewCertPool()
		require.True(t, roots.AppendCertsFromPEM(secret.Data[state.GitServerTLSCAKey]))
		_, err = cert.Verify(x509.VerifyOptions{Roots: roots, DNSName: state.ZarfInClusterGitServiceHost})
		require.NoError(t, err)
		for _, host := range append(append([]string{}, state.ZarfGitServerTLSHosts...), "127.0.0.1", "::1") {
			require.NoError(t, cert.VerifyHostname(host))
		}
	})

	t.Run("user certificate is stored in the Secret", func(t *testing.T) {
		certs, err := pki.GeneratePKI(state.ZarfInClusterGitServiceHost, state.ZarfGitServerTLSHosts...)
		require.NoError(t, err)
		values, err := yaml.Marshal(map[string]any{"certificates": map[string]any{
			"required": true, "ca": string(certs.CA), "cert": string(certs.Cert), "key": string(certs.Key),
		}})
		require.NoError(t, err)
		valuesFile := filepath.Join(t.TempDir(), "git-tls-values.yaml")
		require.NoError(t, os.WriteFile(valuesFile, values, 0600))
		secret := render(t, valuesFile)
		require.Equal(t, certs.CA, secret.Data[state.GitServerTLSCAKey])
		require.Equal(t, certs.Cert, secret.Data[state.GitServerTLSCertKey])
		require.Equal(t, certs.Key, secret.Data[state.GitServerTLSKey])
	})

	t.Run("incomplete certificate fails chart rendering", func(t *testing.T) {
		valuesFile := filepath.Join(t.TempDir(), "partial-values.yaml")
		require.NoError(t, os.WriteFile(valuesFile, []byte("certificates:\n  required: true\n  ca: incomplete\n"), 0600))
		command := exec.Command(e2e.ZarfBinPath, "tools", "helm", "template", "zarf-git-server-tls", "packages/gitea/tls-chart", "-f", valuesFile)
		output, err := command.CombinedOutput()
		require.Error(t, err)
		require.Contains(t, string(output), "GIT_SERVER_TLS_CA, GIT_SERVER_TLS_CERT, and GIT_SERVER_TLS_KEY must be provided together")
	})
}

func TestZarfInit(t *testing.T) {
	t.Log("E2E: Zarf init")

	initComponents := "git-server"
	if e2e.ApplianceMode {
		initComponents = "k3s,git-server"
	}

	initPackageVersion := e2e.GetZarfVersion(t)

	var (
		mismatchedArch        = e2e.GetMismatchedArch()
		mismatchedInitPackage = fmt.Sprintf("zarf-init-%s-%s.tar.zst", mismatchedArch, initPackageVersion)
		expectedErrorMessage  = "unable to run component before action: command \"Check that the host architecture matches the package architecture\""
	)
	t.Cleanup(func() {
		e2e.CleanFiles(t, mismatchedInitPackage)
	})

	if runtime.GOOS == "linux" {
		// Build init package with different arch than the cluster arch.
		stdOut, stdErr, err := e2e.Zarf(t, "package", "create", "src/test/packages/20-mismatched-arch-init", "--architecture", mismatchedArch, "--confirm")
		require.NoError(t, err, stdOut, stdErr)

		// Check that `zarf init` returns an error because of the mismatched architectures.
		// We need to use the --architecture flag here to force zarf to find the package.
		_, stdErr, err = e2e.Zarf(t, "init", "--architecture", mismatchedArch, "--components=k3s", "--confirm")
		require.Error(t, err, stdErr)
		require.Contains(t, stdErr, expectedErrorMessage)
	}

	if !e2e.ApplianceMode {
		// throw a pending pod into the cluster to ensure we can properly ignore them when selecting images
		_, _, err := e2e.Kubectl(t, "apply", "-f", "https://raw.githubusercontent.com/kubernetes/website/main/content/en/examples/pods/pod-with-node-affinity.yaml")
		require.NoError(t, err)
	}

	// Check for any old secrets to ensure that they don't get saved in the init log
	oldState := state.State{}
	base64State, _, err := e2e.Kubectl(t, "get", "secret", "zarf-state", "-n", "zarf", "-o", "jsonpath={.data.state}")
	if err == nil {
		oldStateJSON, err := base64.StdEncoding.DecodeString(base64State)
		require.NoError(t, err)
		err = json.Unmarshal(oldStateJSON, &oldState)
		require.NoError(t, err)
	}

	// run `zarf init`
	_, _, err = e2e.Zarf(t, "init", "--components="+initComponents, "--nodeport", "31337", "--injector-port", "31888", "--confirm")
	require.NoError(t, err)

	// Verify that any state secrets were not included in the log
	s := state.State{}
	base64State, _, err = e2e.Kubectl(t, "get", "secret", "zarf-state", "-n", "zarf", "-o", "jsonpath={.data.state}")
	require.NoError(t, err)
	stateJSON, err := base64.StdEncoding.DecodeString(base64State)
	require.NoError(t, err)
	err = json.Unmarshal(stateJSON, &s)
	require.NoError(t, err)
	if !oldState.ArtifactServer.IsConfigured() {
		require.False(t, s.ArtifactServer.IsConfigured(), "artifact server should be disabled by default")
	}

	if e2e.ApplianceMode {
		// make sure that we upgraded `k3s` correctly and are running the correct version - this should match that found in `packages/distros/k3s`
		kubeletVersion, _, err := e2e.Kubectl(t, "get", "nodes", "-o", "jsonpath={.items[0].status.nodeInfo.kubeletVersion}")
		require.NoError(t, err)
		require.Contains(t, kubeletVersion, "v1.34.3+k3s1")
	}

	// Check that the registry is running on the correct NodePort
	stdOut, _, err := e2e.Kubectl(t, "get", "service", "-n", "zarf", "zarf-docker-registry", "-o=jsonpath='{.spec.ports[*].nodePort}'")
	require.NoError(t, err)
	require.Contains(t, stdOut, "31337")

	// Verify that we save the injector port
	require.Equal(t, 31888, s.InjectorInfo.Port)

	// Check that the registry is running with the correct scale down policy
	stdOut, _, err = e2e.Kubectl(t, "get", "hpa", "-n", "zarf", "zarf-docker-registry", "-o=jsonpath='{.spec.behavior.scaleDown.selectPolicy}'")
	require.NoError(t, err)
	require.Contains(t, stdOut, "Min")

	verifyZarfNamespaceLabels(t)
	verifyZarfSecretLabels(t)
	verifyZarfPodLabels(t)
	verifyZarfServiceLabels(t)

	// Opting into the deprecated artifact server creates its state and Gitea token.
	_, _, err = e2e.Zarf(t, "init", "--components="+initComponents, "--features=artifact-server=true", "--confirm")
	require.NoError(t, err)
	base64State, _, err = e2e.Kubectl(t, "get", "secret", "zarf-state", "-n", "zarf", "-o", "jsonpath={.data.state}")
	require.NoError(t, err)
	stateJSON, err = base64.StdEncoding.DecodeString(base64State)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(stateJSON, &s))
	require.True(t, s.ArtifactServer.IsInternal())
	require.NotEmpty(t, s.ArtifactServer.PushToken)

	// Special sizing-hacking for reducing resources where Kind + CI eats a lot of free cycles (ignore errors)
	_, _, _ = e2e.Kubectl(t, "scale", "deploy", "-n", "kube-system", "coredns", "--replicas=1") //nolint:errcheck
	_, _, _ = e2e.Kubectl(t, "scale", "deploy", "-n", "zarf", "agent-hook", "--replicas=1")     //nolint:errcheck

	// Zarf should fail since registry credentials are changing on a subsequent init
	_, _, err = e2e.Zarf(t, "init", "--components="+initComponents, "--registry-push-password", "new-password", "--confirm")
	require.Error(t, err)

	if s.GitServer.IsInternal() && !s.GitServer.TLSMode.Enabled() {
		verifyGitTLSRotation(t)
	}
}

func verifyGitTLSRotation(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		_, stderr, err := e2e.Zarf(t, "tools", "update-creds", "git", "--git-tls-mode=disabled", "--confirm")
		require.NoError(t, err, stderr)
	})
	_, stderr, err := e2e.Zarf(t, "tools", "update-creds", "git", "--git-tls-mode=tls-enabled", "--confirm")
	require.NoError(t, err, stderr)

	getSecret := func(name string) corev1.Secret {
		t.Helper()
		output, stderr, err := e2e.Kubectl(t, "get", "secret", name, "-n", "zarf", "-o=json")
		require.NoError(t, err, stderr)
		secret := corev1.Secret{}
		require.NoError(t, json.Unmarshal([]byte(output), &secret))
		return secret
	}
	getGiteaDeployment := func() appsv1.Deployment {
		t.Helper()
		output, stderr, err := e2e.Kubectl(t, "get", "deployment", "zarf-gitea", "-n", "zarf", "-o=json")
		require.NoError(t, err, stderr)
		deployment := appsv1.Deployment{}
		require.NoError(t, json.Unmarshal([]byte(output), &deployment))
		return deployment
	}

	previous := getSecret(state.GitServerTLSSecret)
	previousDigest := fmt.Sprintf("%x", sha256.Sum256(previous.Data[state.GitServerTLSCertKey]))
	require.Equal(t, previousDigest, getGiteaDeployment().Spec.Template.Annotations["zarf.dev/git-tls-sha256"])

	_, stderr, err = e2e.Zarf(t, "tools", "update-creds", "git", "--rotate-tls", "--confirm")
	require.NoError(t, err, stderr)
	rotated := getSecret(state.GitServerTLSSecret)
	require.NotEqual(t, previous.Data[state.GitServerTLSCAKey], rotated.Data[state.GitServerTLSCAKey])
	require.NotEqual(t, previous.Data[state.GitServerTLSCertKey], rotated.Data[state.GitServerTLSCertKey])
	rotatedDigest := fmt.Sprintf("%x", sha256.Sum256(rotated.Data[state.GitServerTLSCertKey]))
	require.Equal(t, rotatedDigest, getGiteaDeployment().Spec.Template.Annotations["zarf.dev/git-tls-sha256"])
	require.Equal(t, rotated.Data[state.GitServerTLSCAKey], getSecret(config.ZarfGitServerSecretName).Data[state.GitServerTLSCAKey])
}

func verifyZarfNamespaceLabels(t *testing.T) {
	t.Helper()

	expectedLabels := `'{"app.kubernetes.io/managed-by":"zarf","kubernetes.io/metadata.name":"zarf","zarf.dev/agent":"mutate"}'`
	actualLabels, _, err := e2e.Kubectl(t, "get", "ns", "zarf", "-o=jsonpath='{.metadata.labels}'")
	require.NoError(t, err)
	require.Equal(t, expectedLabels, actualLabels)
}

func verifyZarfSecretLabels(t *testing.T) {
	t.Helper()

	// zarf state
	expectedLabels := `'{"app.kubernetes.io/managed-by":"zarf"}'`
	actualLabels, _, err := e2e.Kubectl(t, "get", "-n=zarf", "secret", "zarf-state", "-o=jsonpath='{.metadata.labels}'")
	require.NoError(t, err)
	require.Equal(t, expectedLabels, actualLabels)

	// init package secret
	expectedLabels = `'{"app.kubernetes.io/managed-by":"zarf","package-deploy-info":"init"}'`
	actualLabels, _, err = e2e.Kubectl(t, "get", "-n=zarf", "secret", "zarf-package-init", "-o=jsonpath='{.metadata.labels}'")
	require.NoError(t, err)
	require.Equal(t, expectedLabels, actualLabels)

	// registry
	expectedLabels = `'{"app.kubernetes.io/managed-by":"zarf"}'`
	actualLabels, _, err = e2e.Kubectl(t, "get", "-n=zarf", "secret", "private-registry", "-o=jsonpath='{.metadata.labels}'")
	require.NoError(t, err)
	require.Equal(t, expectedLabels, actualLabels)

	// agent hook TLS
	//
	// this secret does not have the managed by zarf label
	// because it is deployed as a helm chart rather than generated in Go code. It does get the zarf.dev/package label added
	// as part of the post-renderer.
	expectedLabels = `'{"app.kubernetes.io/managed-by":"Helm","zarf.dev/package":"init"}'`
	actualLabels, _, err = e2e.Kubectl(t, "get", "-n=zarf", "secret", "agent-hook-tls", "-o=jsonpath='{.metadata.labels}'")
	require.NoError(t, err)
	require.Equal(t, expectedLabels, actualLabels)

	// git server
	expectedLabels = `'{"app.kubernetes.io/managed-by":"zarf"}'`
	actualLabels, _, err = e2e.Kubectl(t, "get", "-n=zarf", "secret", "private-git-server", "-o=jsonpath='{.metadata.labels}'")
	require.NoError(t, err)
	require.Equal(t, expectedLabels, actualLabels)
}

func verifyZarfPodLabels(t *testing.T) {
	t.Helper()

	// registry
	podHash, _, err := e2e.Kubectl(t, "get", "-n=zarf", "--selector=app=docker-registry", "pods", `-o=jsonpath="{.items[0].metadata.labels['pod-template-hash']}"`)
	require.NoError(t, err)
	expectedLabels := fmt.Sprintf(`'{"app":"docker-registry","pod-template-hash":%s,"release":"zarf-docker-registry","zarf.dev/agent":"ignore","zarf.dev/package":"init"}'`, podHash)
	actualLabels, _, err := e2e.Kubectl(t, "get", "-n=zarf", "--selector=app=docker-registry", "pods", "-o=jsonpath='{.items[0].metadata.labels}'")
	require.NoError(t, err)
	require.Equal(t, expectedLabels, actualLabels)

	// agent
	podHash, _, err = e2e.Kubectl(t, "get", "-n=zarf", "--selector=app=agent-hook", "pods", `-o=jsonpath="{.items[0].metadata.labels['pod-template-hash']}"`)
	require.NoError(t, err)
	expectedLabels = fmt.Sprintf(`'{"app":"agent-hook","pod-template-hash":%s,"zarf.dev/agent":"ignore","zarf.dev/package":"init"}'`, podHash)
	actualLabels, _, err = e2e.Kubectl(t, "get", "-n=zarf", "--selector=app=agent-hook", "pods", "-o=jsonpath='{.items[0].metadata.labels}'")
	require.NoError(t, err)
	require.Equal(t, expectedLabels, actualLabels)

	// git server
	patchedLabel := `"zarf-agent":"patched","zarf.dev/package":"init"`
	actualLabels, _, err = e2e.Kubectl(t, "get", "-n=zarf", "--selector=app.kubernetes.io/instance=zarf-gitea  ", "pods", "-o=jsonpath='{.items[0].metadata.labels}'")
	require.NoError(t, err)
	require.Contains(t, actualLabels, patchedLabel)
}

func verifyZarfServiceLabels(t *testing.T) {
	t.Helper()

	// registry
	expectedLabels := `'{"app.kubernetes.io/managed-by":"Helm","zarf.dev/connect-name":"registry","zarf.dev/package":"init"}'`
	actualLabels, _, err := e2e.Kubectl(t, "get", "-n=zarf", "service", "zarf-connect-registry", "-o=jsonpath='{.metadata.labels}'")
	require.NoError(t, err)
	require.Equal(t, expectedLabels, actualLabels)

	// git server
	expectedLabels = `'{"app.kubernetes.io/managed-by":"Helm","zarf.dev/connect-name":"git","zarf.dev/package":"init"}'`
	actualLabels, _, err = e2e.Kubectl(t, "get", "-n=zarf", "service", "zarf-connect-git", "-o=jsonpath='{.metadata.labels}'")
	require.NoError(t, err)
	require.Equal(t, expectedLabels, actualLabels)
}
