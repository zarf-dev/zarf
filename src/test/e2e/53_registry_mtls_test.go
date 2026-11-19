// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"github.com/zarf-dev/zarf/src/types"
)

// FIXME: we don't need this everywhere, one e2e test per source and destination should be sufficient
func TestRegistryMTLS(t *testing.T) {
	ctx := testutil.TestContext(t)
	source, sourceTLS := testutil.SetupInMemoryRegistryMTLS(ctx, t)
	destination, destinationTLS := testutil.SetupInMemoryRegistryMTLS(ctx, t)
	configPath, err := filepath.Abs("hack/empty-config.toml")
	require.NoError(t, err)
	t.Setenv("ZARF_CONFIG", configPath)
	// This journey exercises registry access without depending on a cluster.
	t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "missing-kubeconfig"))

	tlsFlags := func(role string, opts types.RegistryTLSOptions) []string {
		return []string{
			"--" + role + "-registry-tls-ca=" + opts.CAFile,
			"--" + role + "-registry-tls-cert=" + opts.CertFile,
			"--" + role + "-registry-tls-key=" + opts.KeyFile,
		}
	}
	sourceFlags := tlsFlags("source", sourceTLS)
	destinationFlags := tlsFlags("destination", destinationTLS)
	cache := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		args = append(args, "--cache", cache)
		stdout, stderr, err := e2e.Zarf(t, args...)
		require.NoError(t, err, stdout, stderr)
		return stdout
	}

	// Package resources use the existing container registry configuration.
	imageRegistry := testutil.SetupInMemoryRegistryDynamic(ctx, t)
	imageRepo := testutil.NewRepo(t, imageRegistry+"/images/fixture")
	imageDesc := testutil.PushSinglePlatformImage(ctx, t, imageRepo, e2e.Arch)
	require.NoError(t, imageRepo.Tag(ctx, imageDesc, "1"))

	packageDir := t.TempDir()
	chartPath, err := filepath.Abs("src/test/packages/15-helm-charts/chart")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(packageDir, "values.yaml"), []byte("message: mtls-package\n"), 0o600))
	definition := fmt.Sprintf(`kind: ZarfPackageConfig
metadata:
  name: mtls
  version: 1.0.0
components:
  - name: resources
    required: true
    images:
      - %s/images/fixture:1
    charts:
      - name: fixture
        version: 0.1.0
        namespace: default
        localPath: %q
        valuesFiles:
          - values.yaml
`, imageRegistry, chartPath)
	require.NoError(t, os.WriteFile(filepath.Join(packageDir, "zarf.yaml"), []byte(definition), 0o600))

	// Creation publishes directly to a Zarf package registry using mTLS.
	args := []string{"package", "create", packageDir, "--output", "oci://" + destination + "/created", "--skip-sbom", "--confirm"}
	run(append(args, destinationFlags...)...)
	createdRef := "oci://" + destination + "/created/mtls:1.0.0"
	run(append([]string{"package", "inspect", "definition", createdRef}, tlsFlags("source", destinationTLS)...)...)

	// The publish/push alias can copy directly between two independently authenticated registries.
	args = []string{"package", "push", createdRef, "oci://" + source + "/packages", "--plain-http"}
	args = append(args, tlsFlags("source", destinationTLS)...)
	run(append(args, tlsFlags("destination", sourceTLS)...)...)
	sourceRef := "oci://" + source + "/packages/mtls:1.0.0"

	for _, inspect := range []string{"definition", "images", "digest", "values-files", "manifests"} {
		run(append([]string{"package", "inspect", inspect, sourceRef}, sourceFlags...)...)
	}
	pullDir := t.TempDir()
	run(append([]string{"package", "pull", sourceRef, "--output-directory", pullDir}, sourceFlags...)...)
	archive := filepath.Join(pullDir, fmt.Sprintf("zarf-package-mtls-%s-1.0.0.tar.zst", e2e.Arch))
	require.FileExists(t, archive)

	// Sign reads using the source identity and publishes with the destination identity.
	args = []string{"package", "sign", sourceRef, "--output", "oci://" + destination + "/signed", "--signing-key", "src/test/packages/zarf-test.prv-key", "--confirm"}
	args = append(args, sourceFlags...)
	run(append(args, destinationFlags...)...)
	signedRef := "oci://" + destination + "/signed/mtls:1.0.0"
	run(append([]string{"package", "verify", signedRef, "--key", "src/test/packages/zarf-test.pub"}, tlsFlags("source", destinationTLS)...)...)

	// Local archives also publish with destination credentials.
	run(append([]string{"package", "publish", archive, "oci://" + destination + "/archives"}, destinationFlags...)...)

	// Remote imports use source credentials even when the resulting package is published elsewhere.
	run(append([]string{"package", "publish", packageDir, "oci://" + source + "/skeletons"}, tlsFlags("destination", sourceTLS)...)...)
	importDir := t.TempDir()
	importDefinition := fmt.Sprintf(`kind: ZarfPackageConfig
metadata:
  name: imported-mtls
  version: 1.0.0
components:
  - name: resources
    import:
      url: oci://%s/skeletons/mtls:1.0.0
`, source)
	require.NoError(t, os.WriteFile(filepath.Join(importDir, "zarf.yaml"), []byte(importDefinition), 0o600))
	args = []string{"package", "create", importDir, "--output", "oci://" + destination + "/imports", "--skip-sbom", "--confirm"}
	args = append(args, sourceFlags...)
	run(append(args, destinationFlags...)...)

	mirrorRegistry := testutil.SetupInMemoryRegistryDynamic(ctx, t)
	args = []string{"package", "mirror-resources", sourceRef, "--registry-url", mirrorRegistry, "--images", "--no-img-checksum", "--confirm"}
	run(append(args, sourceFlags...)...)
	mirroredRepo := testutil.NewRepo(t, mirrorRegistry+"/images/fixture")
	mirroredDesc, err := mirroredRepo.Resolve(ctx, "1")
	require.NoError(t, err)
	require.Equal(t, imageDesc.Digest, mirroredDesc.Digest)

	// Init authenticates its package read before validating the package kind.
	_, stderr, err := e2e.Zarf(t, append([]string{"init", sourceRef, "--cache", cache}, sourceFlags...)...)
	require.Error(t, err)
	require.Contains(t, stderr, `zarf init can only deploy packages of kind "ZarfInitConfig"`)

	// Local actions exercise deploy and remove from an mTLS package source without a cluster.
	localDir := t.TempDir()
	localDefinition := `kind: ZarfPackageConfig
metadata:
  name: local-mtls
  version: 1.0.0
components:
  - name: local
    required: true
    actions:
      onDeploy:
        after:
          - cmd: echo mtls-deploy-action
      onRemove:
        after:
          - cmd: echo mtls-remove-action
`
	require.NoError(t, os.WriteFile(filepath.Join(localDir, "zarf.yaml"), []byte(localDefinition), 0o600))
	run(append([]string{"package", "create", localDir, "--output", "oci://" + source + "/local", "--skip-sbom", "--confirm"}, tlsFlags("destination", sourceTLS)...)...)
	localRef := "oci://" + source + "/local/local-mtls:1.0.0"
	stdout := run(append([]string{"package", "deploy", localRef, "--confirm"}, sourceFlags...)...)
	require.Contains(t, stdout, "mtls-deploy-action")
	stdout = run(append([]string{"package", "remove", localRef, "--confirm"}, sourceFlags...)...)
	require.Contains(t, stdout, "mtls-remove-action")

	t.Run("certificate from environment and key from flag", func(t *testing.T) {
		t.Setenv("ZARF_PACKAGE_PULL_SOURCE_REGISTRY_TLS_CERT", sourceTLS.CertFile)
		stdout, stderr, err := e2e.Zarf(t, "package", "pull", sourceRef, "--output-directory", t.TempDir(), "--cache", cache,
			"--source-registry-tls-ca", sourceTLS.CAFile, "--source-registry-tls-key", sourceTLS.KeyFile)
		require.NoError(t, err, stdout, stderr)
	})

	// Incomplete pairs fail before attempting registry access.
	_, stderr, err = e2e.Zarf(t, "package", "pull", sourceRef, "--source-registry-tls-cert", sourceTLS.CertFile)
	require.Error(t, err)
	require.Contains(t, stderr, "--source-registry-tls-cert and --source-registry-tls-key must be provided together")
}
