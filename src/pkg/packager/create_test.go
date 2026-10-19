// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	gcrlayout "github.com/google/go-containerregistry/pkg/v1/layout"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/feature"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/signing"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestPackageCreateRejectsV1Beta1WithoutFeature(t *testing.T) {
	ctx := testutil.TestContext(t)
	require.False(t, feature.IsEnabled(feature.V1Beta1))
	packageDir := t.TempDir()
	outputDir := t.TempDir()
	definition := `apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: beta-feature
components:
  - name: empty
    actions:
      onCreate:
        before:
          - cmd: echo created > created.txt
`
	require.NoError(t, os.WriteFile(filepath.Join(packageDir, layout.ZarfYAML), []byte(definition), 0o600))

	packagePath, err := Create(ctx, packageDir, outputDir, CreateOptions{
		CachePath: t.TempDir(),
		SkipSBOM:  true,
	})
	require.ErrorContains(t, err, "--features=v1beta1=true")
	require.Empty(t, packagePath)
	require.NoFileExists(t, filepath.Join(packageDir, "created.txt"))
	entries, err := os.ReadDir(outputDir)
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestCreateArchitectureConcurrentV1Alpha1(t *testing.T) {
	ctx := testutil.TestContext(t)
	registry := testutil.SetupInMemoryRegistryDynamic(ctx, t)
	image := registry + "/images:1"
	testutil.PushMultiArchIndex(ctx, t, registry+"/images", "1", []ocispec.Platform{
		{OS: "linux", Architecture: "amd64"}, {OS: "linux", Architecture: "arm64"},
	})
	source := t.TempDir()
	root := `apiVersion: zarf.dev/v1alpha1
kind: ZarfPackageConfig
metadata:
  name: concurrent
  architecture: amd64
components:
  - name: selected
    description: '###ZARF_PKG_ARCH###'
    import:
      path: imported
`
	require.NoError(t, os.Mkdir(filepath.Join(source, "imported"), 0o700))
	imported := `kind: ZarfPackageConfig
metadata:
  name: imported
  architecture: amd64
components:
`
	for _, arch := range []string{"amd64", "arm64"} {
		imported += fmt.Sprintf(`  - name: selected
    only:
      cluster:
        architecture: %s
    files:
      - source: %s.txt
        target: /payload
    images:
      - %s
`, arch, arch, image)
		require.NoError(t, os.WriteFile(filepath.Join(source, "imported", arch+".txt"), []byte(arch), 0o600))
	}
	require.NoError(t, os.WriteFile(filepath.Join(source, "imported", layout.ZarfYAML), []byte(imported), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(source, layout.ZarfYAML), []byte(root), 0o600))

	for arch, pkg := range assertConcurrentCreateArchitectures(t, source) {
		require.Equal(t, arch, pkg.Components[0].Description)
	}
}

func TestCreateArchitectureConcurrentV1Beta1(t *testing.T) {
	ctx := testutil.TestContext(t)
	// Feature configuration is write-once. Isolate enabling beta from tests that require it disabled.
	const betaTestEnv = "ZARF_TEST_CREATE_ARCHITECTURE_V1BETA1"
	if os.Getenv(betaTestEnv) != "true" {
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCreateArchitectureConcurrentV1Beta1$")
		cmd.Env = append(os.Environ(), betaTestEnv+"=true")
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "%s", output)
		return
	}
	require.NoError(t, feature.Set([]feature.Feature{{Name: feature.V1Beta1, Enabled: true}}))
	registry := testutil.SetupInMemoryRegistryDynamic(ctx, t)
	image := registry + "/images:1"
	testutil.PushMultiArchIndex(ctx, t, registry+"/images", "1", []ocispec.Platform{
		{OS: "linux", Architecture: "amd64"}, {OS: "linux", Architecture: "arm64"},
	})
	source := t.TempDir()
	root := `apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: concurrent
  architecture: amd64
components:
  - name: selected
    import:
      local:
        - path: amd64.yaml
        - path: arm64.yaml
`
	for _, arch := range []string{"amd64", "arm64"} {
		component := fmt.Sprintf(`apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: selected
variant:
  architecture: %s
component:
  files:
    - source: %s.txt
      destination: /payload
  images:
    - name: %s
`, arch, arch, image)
		require.NoError(t, os.WriteFile(filepath.Join(source, arch+".yaml"), []byte(component), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(source, arch+".txt"), []byte(arch), 0o600))
	}
	require.NoError(t, os.WriteFile(filepath.Join(source, layout.ZarfYAML), []byte(root), 0o600))

	assertConcurrentCreateArchitectures(t, source)
}

func assertConcurrentCreateArchitectures(t *testing.T, source string) map[string]api.Package {
	t.Helper()
	previous := config.CLIArch
	config.CLIArch = "amd64"
	t.Cleanup(func() { config.CLIArch = previous })
	ctx := testutil.TestContext(t)
	type result struct {
		arch, path string
		err        error
	}
	results := make(chan result, 2)
	start := make(chan struct{})
	var workers sync.WaitGroup
	for _, arch := range []string{"amd64", "arm64"} {
		opts := CreateOptions{Architecture: arch, CachePath: t.TempDir(), SkipSBOM: true, SetVariables: map[string]string{}, RemoteOptions: defaultTestRemoteOptions()}
		output := t.TempDir()
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			path, err := Create(ctx, source, output, opts)
			results <- result{arch: arch, path: path, err: err}
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	packages := make(map[string]api.Package)
	for result := range results {
		require.NoError(t, result.err)
		require.Equal(t, "zarf-package-concurrent-"+result.arch+".tar.zst", filepath.Base(result.path))
		pkgLayout, err := layout.LoadFromTar(ctx, result.path, layout.PackageLayoutOptions{})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, pkgLayout.Cleanup()) })
		pkg := pkgLayout.Definition()
		require.Equal(t, result.arch, pkg.Metadata.Architecture)
		require.Equal(t, result.arch, pkg.Build.Architecture)
		require.Len(t, pkg.Components, 1)
		filesDir, err := pkgLayout.GetComponentDir(ctx, t.TempDir(), "selected", layout.FilesComponentDir)
		require.NoError(t, err)
		payload, err := os.ReadFile(filepath.Join(filesDir, layout.ComponentFileRelPath(0, "/payload")))
		require.NoError(t, err)
		require.Equal(t, result.arch, string(payload))
		imageLayout, err := gcrlayout.FromPath(pkgLayout.GetImageDirPath())
		require.NoError(t, err)
		index, err := imageLayout.ImageIndex()
		require.NoError(t, err)
		manifest, err := index.IndexManifest()
		require.NoError(t, err)
		require.Len(t, manifest.Manifests, 1)
		selected, err := index.Image(manifest.Manifests[0].Digest)
		require.NoError(t, err)
		imageConfig, err := selected.ConfigFile()
		require.NoError(t, err)
		require.Equal(t, result.arch, imageConfig.Architecture)
		packages[result.arch] = pkg
	}
	require.Equal(t, "amd64", config.CLIArch)
	return packages
}

func TestCreateDifferentialUsesRequestedArchitecture(t *testing.T) {
	previous := config.CLIArch
	config.CLIArch = "amd64"
	t.Cleanup(func() { config.CLIArch = previous })
	ctx := testutil.TestContext(t)
	reg := createRegistry(ctx, t)
	image := reg.Registry + "/images:1"
	testutil.PushMultiArchIndex(ctx, t, reg.Registry+"/images", "1", []ocispec.Platform{
		{OS: "linux", Architecture: "amd64"}, {OS: "linux", Architecture: "arm64"},
	})
	source := t.TempDir()
	baseline := fmt.Sprintf(`kind: ZarfPackageConfig
metadata:
  name: baseline
  version: 0.0.1
components:
  - name: empty
  - name: images
    only:
      cluster:
        architecture: arm64
    images:
      - %s
`, image)
	require.NoError(t, os.WriteFile(filepath.Join(source, layout.ZarfYAML), []byte(baseline), 0o600))
	var ref string
	for _, arch := range []string{"amd64", "arm64"} {
		var err error
		ref, err = Create(ctx, source, "oci://"+reg.String(), CreateOptions{Architecture: arch, CachePath: t.TempDir(), SkipSBOM: true, RemoteOptions: defaultTestRemoteOptions()})
		require.NoError(t, err)
	}
	updated := fmt.Sprintf(`kind: ZarfPackageConfig
metadata:
  name: baseline
  version: 0.0.2
components:
  - name: images
    images:
      - %s
`, image)
	require.NoError(t, os.WriteFile(filepath.Join(source, layout.ZarfYAML), []byte(updated), 0o600))
	path, err := Create(ctx, source, t.TempDir(), CreateOptions{Architecture: "arm64", DifferentialPackagePath: "oci://" + ref, CachePath: t.TempDir(), SkipSBOM: true, RemoteOptions: defaultTestRemoteOptions()})
	require.NoError(t, err)
	pkgLayout, err := layout.LoadFromTar(ctx, path, layout.PackageLayoutOptions{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, pkgLayout.Cleanup()) })
	pkg := pkgLayout.Definition()
	require.Equal(t, "arm64", pkg.Metadata.Architecture)
	require.Equal(t, "arm64", pkg.Build.Architecture)
	require.True(t, pkg.Build.Differential)
	require.Equal(t, "0.0.1", pkg.Build.DifferentialPackageVersion)
	require.Len(t, pkg.Components, 1)
	require.Empty(t, pkg.Components[0].Images, "the arm64 baseline already contains this image; the amd64 baseline does not")
}

func TestPackageCreatePreservesV1alpha1MetadataAnnotationCollisions(t *testing.T) {
	ctx := testutil.TestContext(t)
	packageDir := t.TempDir()
	definition := `kind: ZarfPackageConfig
metadata:
  name: metadata-collisions
  architecture: amd64
  url: field-url
  image: field-image
  authors: field-authors
  documentation: field-documentation
  source: field-source
  vendor: field-vendor
  annotations:
    url: annotation-url
    image: annotation-image
    authors: annotation-authors
    documentation: annotation-documentation
    source: annotation-source
    vendor: annotation-vendor
components:
- name: empty
`
	require.NoError(t, os.WriteFile(filepath.Join(packageDir, layout.ZarfYAML), []byte(definition), 0o600))

	packagePath, err := Create(ctx, packageDir, t.TempDir(), CreateOptions{
		CachePath: t.TempDir(),
		SkipSBOM:  true,
	})
	require.NoError(t, err)

	pkgLayout, err := layout.LoadFromTar(ctx, packagePath, layout.PackageLayoutOptions{VerificationStrategy: layout.VerifyNever})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, pkgLayout.Cleanup()) })

	metadata := pkgLayout.Definition().Metadata
	require.Equal(t, "field-url", metadata.URL)
	require.Equal(t, "field-image", metadata.Image)
	require.Equal(t, "field-authors", metadata.Authors)
	require.Equal(t, "field-documentation", metadata.Documentation)
	require.Equal(t, "field-source", metadata.Source)
	require.Equal(t, "field-vendor", metadata.Vendor)
	require.Equal(t, map[string]string{
		"url":           "annotation-url",
		"image":         "annotation-image",
		"authors":       "annotation-authors",
		"documentation": "annotation-documentation",
		"source":        "annotation-source",
		"vendor":        "annotation-vendor",
	}, metadata.Annotations)
}

func TestPackageCreatePublishArch(t *testing.T) {
	ctx := testutil.TestContext(t)
	tests := []struct {
		name         string
		path         string
		expectedArch string
	}{
		{
			name:         "should use pkg.metadata.architecture when global arch not set",
			path:         filepath.Join("testdata", "create", "create-publish-arch"),
			expectedArch: "amd64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := createRegistry(ctx, t)
			packageSource, err := Create(ctx, tt.path, fmt.Sprintf("oci://%s", reg.String()), CreateOptions{
				RemoteOptions: defaultTestRemoteOptions(),
			})
			require.NoError(t, err)
			layout := pullFromRemote(ctx, t, packageSource, tt.expectedArch, "", t.TempDir(), defaultTestRemoteOptions())
			require.Equal(t, tt.expectedArch, layout.Definition().Metadata.Architecture)
		})
	}
}

func TestPackageCreateSignsArchiveAndOCIOutputs(t *testing.T) {
	ctx := testutil.TestContext(t)
	signOpts := signing.DefaultSignBlobOptions()
	signOpts.Key = filepath.Join("testdata", "publish", "cosign.key")
	signOpts.Password = "password"
	publicKeyPath := filepath.Join("testdata", "publish", "cosign.pub")
	source := filepath.Join("testdata", "create", "create-publish-arch")

	t.Run("archive", func(t *testing.T) {
		packagePath, err := Create(ctx, source, t.TempDir(), CreateOptions{
			CachePath:       t.TempDir(),
			SignBlobOptions: &signOpts,
		})
		require.NoError(t, err)

		verifyOpts := signing.DefaultVerifyBlobOptions()
		verifyOpts.Key = publicKeyPath
		pkgLayout, err := layout.LoadFromTar(ctx, packagePath, layout.PackageLayoutOptions{
			VerificationStrategy: layout.VerifyAlways,
			VerifyBlobOptions:    &verifyOpts,
		})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, pkgLayout.Cleanup()) })
		require.True(t, pkgLayout.IsSigned())
		require.FileExists(t, filepath.Join(pkgLayout.DirPath(), layout.Bundle))
	})

	t.Run("deprecated key fields archive", func(t *testing.T) {
		packagePath, err := Create(ctx, source, t.TempDir(), CreateOptions{
			CachePath:          t.TempDir(),
			SigningKeyPath:     signOpts.Key,
			SigningKeyPassword: signOpts.Password,
		})
		require.NoError(t, err)

		verifyOpts := signing.DefaultVerifyBlobOptions()
		verifyOpts.Key = publicKeyPath
		pkgLayout, err := layout.LoadFromTar(ctx, packagePath, layout.PackageLayoutOptions{
			VerificationStrategy: layout.VerifyAlways,
			VerifyBlobOptions:    &verifyOpts,
		})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, pkgLayout.Cleanup()) })
		require.True(t, pkgLayout.IsSigned())
	})

	t.Run("OCI", func(t *testing.T) {
		registryRef := createRegistry(ctx, t)
		packageRef, err := Create(ctx, source, fmt.Sprintf("oci://%s", registryRef.String()), CreateOptions{
			CachePath:       t.TempDir(),
			RemoteOptions:   defaultTestRemoteOptions(),
			SignBlobOptions: &signOpts,
		})
		require.NoError(t, err)

		pkgLayout := pullFromRemote(ctx, t, packageRef, "amd64", publicKeyPath, t.TempDir(), defaultTestRemoteOptions())
		t.Cleanup(func() { require.NoError(t, pkgLayout.Cleanup()) })
		require.True(t, pkgLayout.IsSigned())
		require.FileExists(t, filepath.Join(pkgLayout.DirPath(), layout.Bundle))
	})

	t.Run("deprecated key fields OCI", func(t *testing.T) {
		registryRef := createRegistry(ctx, t)
		packageRef, err := Create(ctx, source, fmt.Sprintf("oci://%s", registryRef.String()), CreateOptions{
			CachePath:          t.TempDir(),
			RemoteOptions:      defaultTestRemoteOptions(),
			SigningKeyPath:     signOpts.Key,
			SigningKeyPassword: signOpts.Password,
		})
		require.NoError(t, err)

		pkgLayout := pullFromRemote(ctx, t, packageRef, "amd64", publicKeyPath, t.TempDir(), defaultTestRemoteOptions())
		t.Cleanup(func() { require.NoError(t, pkgLayout.Cleanup()) })
		require.True(t, pkgLayout.IsSigned())
	})

	t.Run("explicit signing options override deprecated key fields", func(t *testing.T) {
		packagePath, err := Create(ctx, source, t.TempDir(), CreateOptions{
			CachePath:          t.TempDir(),
			SignBlobOptions:    &signOpts,
			SigningKeyPath:     filepath.Join(t.TempDir(), "missing.key"),
			SigningKeyPassword: "wrong-password",
		})
		require.NoError(t, err)

		verifyOpts := signing.DefaultVerifyBlobOptions()
		verifyOpts.Key = publicKeyPath
		pkgLayout, err := layout.LoadFromTar(ctx, packagePath, layout.PackageLayoutOptions{
			VerificationStrategy: layout.VerifyAlways,
			VerifyBlobOptions:    &verifyOpts,
		})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, pkgLayout.Cleanup()) })
		require.True(t, pkgLayout.IsSigned())
	})
}

func TestPackageCreateDifferentialOCIPackage(t *testing.T) {
	ctx := testutil.TestContext(t)
	tests := []struct {
		name           string
		newPackagePath string
		oldPackagePath string
	}{
		{
			name:           "differential package builds from OCI source",
			oldPackagePath: filepath.Join("testdata", "create", "differential", "older-version"),
			newPackagePath: filepath.Join("testdata", "create", "differential"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := createRegistry(ctx, t)
			packageSource, err := Create(ctx, tt.oldPackagePath, fmt.Sprintf("oci://%s", reg.String()), CreateOptions{
				RemoteOptions: defaultTestRemoteOptions(),
			})
			require.NoError(t, err)
			tmpdir := t.TempDir()
			newPackageSource, err := Create(ctx, tt.newPackagePath, tmpdir, CreateOptions{
				DifferentialPackagePath: fmt.Sprintf("oci://%s", packageSource),
				RemoteOptions:           defaultTestRemoteOptions(),
				CachePath:               t.TempDir(),
			})
			require.NoError(t, err)
			require.Equal(t, filepath.Join(tmpdir, "zarf-package-differential-test-amd64-0.0.1-differential-0.0.2.tar.zst"), newPackageSource)
		})
	}
}
