// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/state"
	appsv1 "k8s.io/api/apps/v1"
)

const (
	registryBackendEnvVar = "ZARF_E2E_REGISTRY_BACKEND"

	minIORegistryBackend = "minio"
	minIONamespace       = "zarf-minio"
	minIOBucket          = "zarf-registry"
	minIOAccessKey       = "minioadmin"
	minIOSecretKey       = "minioadmin"
	minIOEndpoint        = "http://minio.zarf-minio.svc.cluster.local:9000"
)

type minIORegistryConfig struct {
	ServiceAccountName string
	AnnotationKey      string
	AnnotationValue    string
	RootDirectory      string
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

	minIOCfg := setupMinIORegistryBackend(t)

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
	initArgs := []string{"init", "--components=" + initComponents, "--nodeport", "31337", "--injector-port", "31888", "--confirm"}
	_, _, err = e2e.Zarf(t, initArgs...)
	require.NoError(t, err)

	// Verify that any state secrets were not included in the log
	s := state.State{}
	base64State, _, err = e2e.Kubectl(t, "get", "secret", "zarf-state", "-n", "zarf", "-o", "jsonpath={.data.state}")
	require.NoError(t, err)
	stateJSON, err := base64.StdEncoding.DecodeString(base64State)
	require.NoError(t, err)
	err = json.Unmarshal(stateJSON, &s)
	require.NoError(t, err)

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

	if minIOCfg != nil {
		verifyMinIORegistryBackend(t, *minIOCfg)
	}

	verifyZarfNamespaceLabels(t)
	verifyZarfSecretLabels(t)
	verifyZarfPodLabels(t)
	verifyZarfServiceLabels(t)

	// Special sizing-hacking for reducing resources where Kind + CI eats a lot of free cycles (ignore errors)
	_, _, _ = e2e.Kubectl(t, "scale", "deploy", "-n", "kube-system", "coredns", "--replicas=1") //nolint:errcheck
	_, _, _ = e2e.Kubectl(t, "scale", "deploy", "-n", "zarf", "agent-hook", "--replicas=1")     //nolint:errcheck

	// Zarf should fail since registry credentials are changing on a subsequent init
	_, _, err = e2e.Zarf(t, "init", "--components="+initComponents, "--registry-push-password", "new-password", "--confirm")
	require.Error(t, err)
}

func setupMinIORegistryBackend(t *testing.T) *minIORegistryConfig {
	t.Helper()

	backend := strings.ToLower(os.Getenv(registryBackendEnvVar))
	if backend == "" {
		return nil
	}
	require.Equal(t, minIORegistryBackend, backend, "unsupported %s value", registryBackendEnvVar)

	t.Log("E2E: Configuring init registry with a MinIO S3-compatible backend")

	manifestPath := filepath.Join(t.TempDir(), "minio.yaml")
	err := os.WriteFile(manifestPath, []byte(minIOManifest()), 0600)
	require.NoError(t, err)

	stdOut, stdErr, err := e2e.Kubectl(t, "apply", "-f", manifestPath)
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Kubectl(t, "wait", "deployment", "minio", "-n", minIONamespace, "--for=condition=Available", "--timeout=3m")
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Kubectl(t, "wait", "job", "create-zarf-registry-bucket", "-n", minIONamespace, "--for=condition=Complete", "--timeout=2m")
	require.NoError(t, err, stdOut, stdErr)

	cfg := minIORegistryConfig{
		ServiceAccountName: "zarf-registry-minio",
		AnnotationKey:      "minio.zarf.dev/backend",
		AnnotationValue:    "s3",
		RootDirectory:      "/zarf-init-test",
	}

	registryExtraEnvVars := fmt.Sprintf(`- name: REGISTRY_STORAGE
  value: s3
- name: REGISTRY_STORAGE_REDIRECT_DISABLE
  value: "true"
- name: REGISTRY_STORAGE_S3_REGION
  value: us-east-1
- name: REGISTRY_STORAGE_S3_REGIONENDPOINT
  value: %s
- name: REGISTRY_STORAGE_S3_BUCKET
  value: %s
- name: REGISTRY_STORAGE_S3_ROOTDIRECTORY
  value: %s
- name: REGISTRY_STORAGE_S3_ACCESSKEY
  value: %s
- name: REGISTRY_STORAGE_S3_SECRETKEY
  value: %s
- name: REGISTRY_STORAGE_S3_SECURE
  value: "false"
- name: REGISTRY_STORAGE_S3_FORCEPATHSTYLE
  value: "true"
`, minIOEndpoint, minIOBucket, cfg.RootDirectory, minIOAccessKey, minIOSecretKey)

	configPath := filepath.Join(t.TempDir(), "zarf-config.yaml")
	configFile := fmt.Sprintf(`package:
  deploy:
    set:
      REGISTRY_PVC_ENABLED: "false"
      REGISTRY_HPA_MAX: "1"
      REGISTRY_CREATE_SERVICE_ACCOUNT: "true"
      REGISTRY_SERVICE_ACCOUNT_NAME: %q
      REGISTRY_SERVICE_ACCOUNT_ANNOTATIONS: |-
        %s: %q
      REGISTRY_EXTRA_ENVS: |-
%s
`, cfg.ServiceAccountName, cfg.AnnotationKey, cfg.AnnotationValue, indent(registryExtraEnvVars, 8))
	err = os.WriteFile(configPath, []byte(configFile), 0600)
	require.NoError(t, err)

	previousConfig, hadPreviousConfig := os.LookupEnv("ZARF_CONFIG")
	err = os.Setenv("ZARF_CONFIG", configPath)
	require.NoError(t, err)
	t.Cleanup(func() {
		if hadPreviousConfig {
			require.NoError(t, os.Setenv("ZARF_CONFIG", previousConfig))
			return
		}
		require.NoError(t, os.Unsetenv("ZARF_CONFIG"))
	})

	return &cfg
}

func verifyMinIORegistryBackend(t *testing.T, cfg minIORegistryConfig) {
	t.Helper()

	stdOut, stdErr, err := e2e.Kubectl(t, "get", "pvc", "zarf-docker-registry", "-n", "zarf")
	require.Error(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Kubectl(t, "get", "serviceaccount", cfg.ServiceAccountName, "-n", "zarf", "-o=jsonpath={.metadata.annotations.minio\\.zarf\\.dev/backend}")
	require.NoError(t, err, stdOut, stdErr)
	require.Equal(t, cfg.AnnotationValue, stdOut)

	stdOut, stdErr, err = e2e.Kubectl(t, "get", "deployment", "zarf-docker-registry", "-n", "zarf", "-o", "json")
	require.NoError(t, err, stdOut, stdErr)

	deployment := appsv1.Deployment{}
	err = json.Unmarshal([]byte(stdOut), &deployment)
	require.NoError(t, err)
	require.Equal(t, cfg.ServiceAccountName, deployment.Spec.Template.Spec.ServiceAccountName)
	require.Len(t, deployment.Spec.Template.Spec.Containers, 1)

	envVars := map[string]string{}
	for _, envVar := range deployment.Spec.Template.Spec.Containers[0].Env {
		envVars[envVar.Name] = envVar.Value
	}
	require.Equal(t, "s3", envVars["REGISTRY_STORAGE"])
	require.Equal(t, "true", envVars["REGISTRY_STORAGE_REDIRECT_DISABLE"])
	require.Equal(t, "us-east-1", envVars["REGISTRY_STORAGE_S3_REGION"])
	require.Equal(t, minIOEndpoint, envVars["REGISTRY_STORAGE_S3_REGIONENDPOINT"])
	require.Equal(t, minIOBucket, envVars["REGISTRY_STORAGE_S3_BUCKET"])
	require.Equal(t, cfg.RootDirectory, envVars["REGISTRY_STORAGE_S3_ROOTDIRECTORY"])
	require.Equal(t, minIOAccessKey, envVars["REGISTRY_STORAGE_S3_ACCESSKEY"])
	require.Equal(t, minIOSecretKey, envVars["REGISTRY_STORAGE_S3_SECRETKEY"])
	require.Equal(t, "false", envVars["REGISTRY_STORAGE_S3_SECURE"])
	require.Equal(t, "true", envVars["REGISTRY_STORAGE_S3_FORCEPATHSTYLE"])

	stdOut, stdErr, err = e2e.Zarf(t, "tools", "registry", "catalog")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdOut, "zarf-dev/zarf/agent")
	require.Contains(t, stdOut, "library/registry")

	stdOut, stdErr, err = e2e.Kubectl(t, "run", "minio-list-registry", "-n", minIONamespace, "--image=quay.io/minio/mc:RELEASE.2025-08-13T08-35-41Z", "--restart=Never", "--attach", "--rm", "--command", "--", "/bin/sh", "-c", fmt.Sprintf("mc alias set local %s %s %s >/dev/null && mc ls --recursive local/%s%s", minIOEndpoint, minIOAccessKey, minIOSecretKey, minIOBucket, cfg.RootDirectory))
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdOut, "docker/registry")
}

func indent(s string, spaces int) string {
	prefix := strings.Repeat(" ", spaces)
	return prefix + strings.ReplaceAll(strings.TrimRight(s, "\n"), "\n", "\n"+prefix)
}

func minIOManifest() string {
	return fmt.Sprintf(`apiVersion: v1
kind: Namespace
metadata:
  name: %[1]s
  labels:
    zarf.dev/agent: ignore
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: minio
  namespace: %[1]s
spec:
  selector:
    matchLabels:
      app: minio
  template:
    metadata:
      labels:
        app: minio
    spec:
      containers:
        - name: minio
          image: quay.io/minio/minio:RELEASE.2025-09-07T16-13-09Z
          args:
            - server
            - /data
            - --console-address
            - :9001
          env:
            - name: MINIO_ROOT_USER
              value: %[2]s
            - name: MINIO_ROOT_PASSWORD
              value: %[3]s
          ports:
            - name: s3
              containerPort: 9000
            - name: console
              containerPort: 9001
          readinessProbe:
            httpGet:
              path: /minio/health/ready
              port: 9000
            initialDelaySeconds: 5
            periodSeconds: 5
          volumeMounts:
            - name: data
              mountPath: /data
      volumes:
        - name: data
          emptyDir: {}
---
apiVersion: v1
kind: Service
metadata:
  name: minio
  namespace: %[1]s
spec:
  selector:
    app: minio
  ports:
    - name: s3
      port: 9000
      targetPort: 9000
    - name: console
      port: 9001
      targetPort: 9001
---
apiVersion: batch/v1
kind: Job
metadata:
  name: create-zarf-registry-bucket
  namespace: %[1]s
spec:
  backoffLimit: 6
  template:
    spec:
      restartPolicy: OnFailure
      containers:
        - name: mc
          image: quay.io/minio/mc:RELEASE.2025-08-13T08-35-41Z
          command:
            - /bin/sh
            - -c
          args:
            - |
              mc alias set local http://minio.%[1]s.svc.cluster.local:9000 %[2]s %[3]s
              mc mb --ignore-existing local/%[4]s
`, minIONamespace, minIOAccessKey, minIOSecretKey, minIOBucket)
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
