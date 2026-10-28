// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/internal/checksum"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"github.com/zarf-dev/zarf/src/types"
	"oras.land/oras-go/v2/registry"
)

func TestLoadPackageSelectsGranularResources(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)
	sourceDir := t.TempDir()
	firstAttachmentPath := path.Join(layout.AttachmentsDir, "first-README.md")
	secondAttachmentPath := path.Join(layout.AttachmentsDir, "second-README.md")
	sbomResourcePath := layout.ComponentSBOMResourcePath("metrics")
	unselectedSBOMResourcePath := layout.ComponentSBOMResourcePath("logs")
	resources := map[string][]byte{
		firstAttachmentPath:        []byte("first attachment"),
		secondAttachmentPath:       []byte("second attachment"),
		sbomResourcePath:           []byte(`{"artifacts":[]}`),
		unselectedSBOMResourcePath: []byte(`{"artifacts":[{"name":"logs"}]}`),
		"metrics.txt":              []byte("metrics"),
	}
	for resourcePath, contents := range resources {
		filePath := filepath.Join(sourceDir, filepath.FromSlash(resourcePath))
		require.NoError(t, os.MkdirAll(filepath.Dir(filePath), 0o700))
		require.NoError(t, os.WriteFile(filePath, contents, 0o600))
	}

	checksums := make([]byte, 0)
	for _, resourcePath := range []string{firstAttachmentPath, secondAttachmentPath, sbomResourcePath, unselectedSBOMResourcePath, "metrics.txt"} {
		digest, err := checksum.GetSHA256Hash(bytes.NewReader(resources[resourcePath]))
		require.NoError(t, err)
		checksums = append(checksums, fmt.Sprintf("%s %s\n", digest, resourcePath)...)
	}
	aggregateChecksum, err := checksum.GetSHA256Hash(bytes.NewReader(checksums))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, layout.Checksums), checksums, 0o600))
	zarfYAML := fmt.Sprintf(`apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: granular-resources
  version: 0.0.1
  architecture: amd64
build:
  architecture: amd64
  aggregateChecksum: %s
attachments:
  first: docs/README.md
  second: other/README.md
components:
  - name: metrics
    files:
      - source: metrics.txt
        destination: metrics.txt
`, aggregateChecksum)
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, layout.ZarfYAML), []byte(zarfYAML), 0o600))

	sourceLayout, err := layout.LoadFromDir(ctx, sourceDir, layout.PackageLayoutOptions{
		VerificationStrategy: layout.VerifyNever,
	})
	require.NoError(t, err)

	registryRef := registry.Reference{
		Registry:   testutil.SetupInMemoryRegistryDynamic(ctx, t),
		Repository: "granular-resources",
	}
	packageRef, err := PublishPackage(ctx, sourceLayout, registryRef, PublishPackageOptions{
		OCIConcurrency: 3,
		RemoteOptions:  types.RemoteOptions{PlainHTTP: true},
	})
	require.NoError(t, err)

	loadOptions := func(selection zoci.LayerSelection) LoadOptions {
		return LoadOptions{
			Architecture:         "amd64",
			CachePath:            t.TempDir(),
			LayerSelection:       &selection,
			VerificationStrategy: layout.VerifyNever,
			RemoteOptions:        types.RemoteOptions{PlainHTTP: true},
		}
	}

	attachmentsLayout, err := LoadPackage(ctx, packageRef.String(), loadOptions(zoci.LayerSelection{
		Attachments: &zoci.AttachmentSelection{Keys: []string{"first"}},
	}))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, attachmentsLayout.Cleanup()) })
	require.FileExists(t, filepath.Join(attachmentsLayout.DirPath(), firstAttachmentPath))
	require.NoFileExists(t, filepath.Join(attachmentsLayout.DirPath(), secondAttachmentPath))
	require.NoFileExists(t, filepath.Join(attachmentsLayout.DirPath(), filepath.FromSlash(sbomResourcePath)))
	require.NoFileExists(t, filepath.Join(attachmentsLayout.DirPath(), filepath.FromSlash(unselectedSBOMResourcePath)))

	attachmentsOutput := t.TempDir()
	require.NoError(t, attachmentsLayout.GetAttachments(ctx, attachmentsOutput, []string{"first"}))
	attachmentsContents, err := os.ReadFile(filepath.Join(attachmentsOutput, "first-README.md"))
	require.NoError(t, err)
	require.Equal(t, resources[firstAttachmentPath], attachmentsContents)
	require.NoFileExists(t, filepath.Join(attachmentsOutput, "second-README.md"))

	sbomLayout, err := LoadPackage(ctx, packageRef.String(), loadOptions(zoci.LayerSelection{
		SBOMs: &zoci.SBOMSelection{Keys: []string{"metrics"}},
	}))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sbomLayout.Cleanup()) })
	require.FileExists(t, filepath.Join(sbomLayout.DirPath(), filepath.FromSlash(sbomResourcePath)))
	require.NoFileExists(t, filepath.Join(sbomLayout.DirPath(), filepath.FromSlash(unselectedSBOMResourcePath)))
	require.NoFileExists(t, filepath.Join(sbomLayout.DirPath(), firstAttachmentPath))
	require.NoFileExists(t, filepath.Join(sbomLayout.DirPath(), secondAttachmentPath))

	sbomOutput := t.TempDir()
	require.NoError(t, sbomLayout.GetSBOM(ctx, sbomOutput, nil))
	sbomContents, err := os.ReadFile(filepath.Join(sbomOutput, "files", "metrics.json"))
	require.NoError(t, err)
	require.Equal(t, resources[sbomResourcePath], sbomContents)
	require.NoFileExists(t, filepath.Join(sbomOutput, "files", "logs.json"))
}
