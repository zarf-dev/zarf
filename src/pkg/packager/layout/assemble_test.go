// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/archive"
)

func TestGetChecksum(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	files := map[string]string{
		"empty.txt":                "",
		"foo":                      "bar",
		"zarf.yaml":                "Zarf Yaml Data",
		"checksums.txt":            "Old Checksum Data",
		"nested/directory/file.md": "nested",
	}
	for k, v := range files {
		err := os.MkdirAll(filepath.Join(tmpDir, filepath.Dir(k)), 0o700)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(tmpDir, k), []byte(v), 0o600)
		require.NoError(t, err)
	}

	checksumContent, checksumHash, err := getChecksum(tmpDir)
	require.NoError(t, err)

	expectedContent := `233562de1a0288b139c4fa40b7d189f806e906eeb048517aeb67f34ac0e2faf1 nested/directory/file.md
e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 empty.txt
fcde2b2edba56bf408601fb721fe9b5c338d10ee429ea04fae5511b68fbf8fb9 foo
`
	require.Equal(t, expectedContent, checksumContent)
	require.Equal(t, "7c554cf67e1c2b50a1b728299c368cd56d53588300c37479623f29a52812ca3f", checksumHash)
}

func TestSignPackage(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "zarf.yaml")
	signedPath := filepath.Join(tmpDir, "zarf.yaml.sig")

	err := os.WriteFile(yamlPath, []byte("foobar"), 0o644)
	require.NoError(t, err)

	err = signPackage(tmpDir, "", "")
	require.NoError(t, err)
	require.NoFileExists(t, signedPath)

	err = signPackage(tmpDir, "./testdata/cosign.key", "wrongpassword")
	require.EqualError(t, err, "reading key: decrypt: encrypted: decryption failed")

	err = signPackage(tmpDir, "./testdata/cosign.key", "test")
	require.NoError(t, err)
	require.FileExists(t, signedPath)
}

func TestCreateReproducibleTarballFromDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("hello world"), 0o600)
	require.NoError(t, err)
	tarPath := filepath.Join(t.TempDir(), "data.tar")

	err = createReproducibleTarballFromDir(tmpDir, "", tarPath, true)
	require.NoError(t, err)

	shaSum, err := helpers.GetSHA256OfFile(tarPath)
	require.NoError(t, err)
	require.Equal(t, "c09d17f612f241cdf549e5fb97c9e063a8ad18ae7a9f3af066332ed6b38556ad", shaSum)
}

func TestAssemblePackageWithAbsoluteValuesPath(t *testing.T) {
	t.Parallel()

	// Arrange
	tmpDir := t.TempDir()

	// Create absolute path values file in separate directory
	valuesDir := t.TempDir()
	absoluteValuesPath := filepath.Join(valuesDir, "absolute-values.yaml")
	absoluteValuesContent := `replicaCount: 3
image:
  tag: "2.0.0"`
	err := os.WriteFile(absoluteValuesPath, []byte(absoluteValuesContent), 0o600)
	require.NoError(t, err)

	// Create relative path values file
	relativeValuesContent := `service:
  type: ClusterIP
  port: 8080`
	err = os.WriteFile(filepath.Join(tmpDir, "relative-values.yaml"), []byte(relativeValuesContent), 0o600)
	require.NoError(t, err)

	// Create minimal chart structure
	chartDir := filepath.Join(tmpDir, "test-chart")
	err = os.MkdirAll(chartDir, 0o700)
	require.NoError(t, err)

	chartYaml := `apiVersion: v2
name: test-chart
version: 1.0.0
description: Test chart for absolute path handling`
	err = os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte(chartYaml), 0o600)
	require.NoError(t, err)

	component := v1alpha1.ZarfComponent{
		Name: "test-component",
		Charts: []v1alpha1.ZarfChart{
			{
				Name:      "test-chart",
				LocalPath: "test-chart",
				ValuesFiles: []string{
					absoluteValuesPath,     // Absolute path
					"relative-values.yaml", // Relative path
				},
			},
		},
	}

	// Act
	buildPath := t.TempDir()
	err = assemblePackageComponent(context.Background(), component, tmpDir, buildPath)
	require.NoError(t, err)

	// Assert
	componentPath := filepath.Join(buildPath, "components", component.Name+".tar")
	require.FileExists(t, componentPath)

	// Extract component to verify contents
	extractPath := t.TempDir()
	err = archive.Decompress(context.Background(), componentPath, extractPath, archive.DecompressOpts{})
	require.NoError(t, err)

	componentExtractPath := filepath.Join(extractPath, component.Name)

	// Verify both values files exist
	absoluteValuesFile := filepath.Join(componentExtractPath, "values", "test-chart--0")
	relativeValuesFile := filepath.Join(componentExtractPath, "values", "test-chart--1")
	require.FileExists(t, absoluteValuesFile)
	require.FileExists(t, relativeValuesFile)

	// Verify absolute path values content
	absoluteContent, err := os.ReadFile(absoluteValuesFile)
	require.NoError(t, err)
	require.Contains(t, string(absoluteContent), "replicaCount: 3")
	require.Contains(t, string(absoluteContent), `tag: "2.0.0"`)

	// Verify relative path values content
	relativeContent, err := os.ReadFile(relativeValuesFile)
	require.NoError(t, err)
	require.Contains(t, string(relativeContent), "type: ClusterIP")
	require.Contains(t, string(relativeContent), "port: 8080")
}
