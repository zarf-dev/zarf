// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package lint contains functions for verifying zarf yaml files are valid
package lint

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/convert"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
)

func TestUnpinnedRepo(t *testing.T) {
	t.Parallel()

	unpinnedRepo := "https://github.com/zarf-dev/zarf-public-test.git"
	component := v1alpha1.ZarfComponent{
		Repos: []string{
			unpinnedRepo,
			"https://dev.azure.com/zarf-dev/zarf-public-test/_git/zarf-public-test@v0.0.1",
		},
	}
	findings := CheckPrimitives(convert.PackageFromV1alpha1(v1alpha1.ZarfPackage{Components: []v1alpha1.ZarfComponent{component}}), false)
	expected := []PackageFinding{
		{
			RuleID:      RuleRepositoryUnpinned,
			Item:        unpinnedRepo,
			Description: "Unpinned repository",
			Severity:    SevWarn,
			YqPath:      ".components.[0].repos.[0]",
		},
	}
	require.Equal(t, expected, findings)
}

func TestUnpinnedImageFindings(t *testing.T) {
	t.Parallel()

	unpinnedImage := "registry.com:9001/whatever/image:1.0.0"
	badImage := "badimage:badimage@@sha256:3fbc632167424a6d997e74f5"
	cosignSignature := "ghcr.io/stefanprodan/podinfo:sha256-57a654ace69ec02ba8973093b6a786faa15640575fbf0dbb603db55aca2ccec8.sig"
	cosignAttestation := "ghcr.io/stefanprodan/podinfo:sha256-57a654ace69ec02ba8973093b6a786faa15640575fbf0dbb603db55aca2ccec8.att"
	component := v1alpha1.ZarfComponent{
		Images: []string{
			unpinnedImage,
			"busybox:latest@sha256:3fbc632167424a6d997e74f52b878d7cc478225cffac6bc977eedfe51c7f4e79",
			badImage,
			cosignSignature,
			cosignAttestation,
		},
	}
	findings := CheckPrimitives(convert.PackageFromV1alpha1(v1alpha1.ZarfPackage{Components: []v1alpha1.ZarfComponent{component}}), false)
	expected := []PackageFinding{
		{
			RuleID:      RuleImageUnpinned,
			Item:        unpinnedImage,
			Description: "Image not pinned with digest",
			Severity:    SevInfo,
			YqPath:      ".components.[0].images.[0]",
		},
		{
			RuleID:      RuleImageReferenceInvalid,
			Item:        badImage,
			Description: "Failed to parse image reference",
			Severity:    SevWarn,
			YqPath:      ".components.[0].images.[2]",
		},
		{RuleID: RuleImageRegistryMissing, Item: component.Images[1], Description: "Image reference does not specify a registry domain", Severity: SevWarn, YqPath: ".components.[0].images.[1]"},
		{RuleID: RuleImageRegistryMissing, Item: badImage, Description: "Image reference does not specify a registry domain", Severity: SevWarn, YqPath: ".components.[0].images.[2]"},
	}
	require.Equal(t, expected, findings)
}

func TestUnpinnnedFileWarning(t *testing.T) {
	t.Parallel()

	fileURL := "http://example.com/file.zip"
	localFile := "local.txt"
	zarfFiles := []v1alpha1.ZarfFile{
		{
			Source: fileURL,
		},
		{
			Source: localFile,
		},
		{
			Source: fileURL,
			Shasum: "fake-shasum",
		},
	}
	component := v1alpha1.ZarfComponent{Files: zarfFiles}
	findings := CheckPrimitives(convert.PackageFromV1alpha1(v1alpha1.ZarfPackage{Components: []v1alpha1.ZarfComponent{component}}), false)
	expected := []PackageFinding{
		{
			RuleID:      RuleFileChecksumMissing,
			Item:        fileURL,
			Description: "No shasum for remote file",
			Severity:    SevWarn,
			YqPath:      ".components.[0].files.[0]",
		},
	}
	require.Equal(t, expected, findings)
	require.Len(t, findings, 1)
}

func TestImagesWithoutDomain(t *testing.T) {
	t.Parallel()

	component := v1alpha1.ZarfComponent{
		Images: []string{
			"myapp:1.0.0",
			"library/myapp:1.0.0",
			"docker.io/library/myapp:1.0.0",
			"ghcr.io/zarf-dev/zarf:v0.1.0",
			"localhost:5000/myapp:1.0.0",
			"###ZARF_PKG_TMPL_IMAGE###",
		},
	}
	findings := CheckPrimitives(convert.PackageFromV1alpha1(v1alpha1.ZarfPackage{Components: []v1alpha1.ZarfComponent{component}}), false)
	var expected []PackageFinding
	for i, image := range component.Images[:5] {
		expected = append(expected, PackageFinding{RuleID: RuleImageUnpinned, Item: image, Description: "Image not pinned with digest", Severity: SevInfo, YqPath: fmt.Sprintf(".components.[0].images.[%d]", i)})
	}
	expected = append(expected, []PackageFinding{
		{
			RuleID:      RuleImageRegistryMissing,
			Item:        "myapp:1.0.0",
			Description: "Image reference does not specify a registry domain",
			Severity:    SevWarn,
			YqPath:      ".components.[0].images.[0]",
		},
		{
			RuleID:      RuleImageRegistryMissing,
			Item:        "library/myapp:1.0.0",
			Description: "Image reference does not specify a registry domain",
			Severity:    SevWarn,
			YqPath:      ".components.[0].images.[1]",
		},
	}...)
	require.Equal(t, expected, findings)
}

func TestImageArchivesWithoutInternalDomain(t *testing.T) {
	t.Parallel()

	component := v1alpha1.ZarfComponent{
		ImageArchives: []v1alpha1.ImageArchive{
			{
				Path: "images.tar",
				Images: []string{
					"zarf.internal/myapp:1.0.0",
					"my.registry.internal:5000/myapp:1.0.0",
					"myapp:1.0.0",
					"docker.io/library/myapp:1.0.0",
					"###ZARF_PKG_TMPL_IMAGE###",
				},
			},
		},
	}
	findings := CheckPrimitives(convert.PackageFromV1alpha1(v1alpha1.ZarfPackage{Components: []v1alpha1.ZarfComponent{component}}), false)
	expected := []PackageFinding{
		{
			RuleID:      RuleImageArchiveDomain,
			Item:        "myapp:1.0.0",
			Description: "Image archive image should use a .internal domain to avoid resolving to a public registry",
			Severity:    SevWarn,
			YqPath:      ".components.[0].imageArchives.[0].images.[2]",
		},
		{
			RuleID:      RuleImageArchiveDomain,
			Item:        "docker.io/library/myapp:1.0.0",
			Description: "Image archive image should use a .internal domain to avoid resolving to a public registry",
			Severity:    SevWarn,
			YqPath:      ".components.[0].imageArchives.[0].images.[3]",
		},
	}
	require.Equal(t, expected, findings)
}

func TestIsImagePinned(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected bool
		err      error
	}{
		{
			input:    "registry.com:8080/zarf-dev/whatever",
			expected: false,
			err:      nil,
		},
		{
			input:    "ghcr.io/zarf-dev/pepr/controller:v0.15.0",
			expected: false,
			err:      nil,
		},
		{
			input:    "busybox:latest@sha256:3fbc632167424a6d997e74f52b878d7cc478225cffac6bc977eedfe51c7f4e79",
			expected: true,
			err:      nil,
		},
		{
			input:    "busybox:bad/image",
			expected: false,
			err:      errors.New("unable to parse ref busybox:bad/image: invalid reference format"),
		},
		{
			input:    "busybox:###ZARF_PKG_TMPL_BUSYBOX_IMAGE###",
			expected: true,
			err:      nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			actual, err := isPinnedImage(tc.input)
			if err != nil {
				require.EqualError(t, err, tc.err.Error())
			}
			require.Equal(t, tc.expected, actual)
		})
	}
}

func TestCheckPrimitivesV1Beta1(t *testing.T) {
	t.Parallel()
	const (
		repoURL = "https://example.com/repo.git"
		image   = "registry.com/myapp:1.0"
		invalid = "registry.com/badimage:badimage@@sha256:3fbc632167424a6d997e74f5"
		digest  = "sha256:3fbc632167424a6d997e74f52b878d7cc478225cffac6bc977eedfe51c7f4e79"
		fileURL = "https://example.com/file.zip"
	)
	component := v1beta1.ComponentSpec{
		Repositories: []v1beta1.Repository{
			{URL: repoURL},
			{URL: repoURL, Ref: &v1beta1.GitRef{Tag: "v1.0"}},
			{URL: repoURL, Ref: &v1beta1.GitRef{Branch: "main"}},
			{URL: repoURL, Ref: &v1beta1.GitRef{Commit: "0123456789012345678901234567890123456789"}},
		},
		Images: []v1beta1.Image{
			{Name: image},
			{Name: invalid},
			{Name: "registry.com/myapp@" + digest},
			{Name: "registry.com/myapp:sha256-57a654ace69ec02ba8973093b6a786faa15640575fbf0dbb603db55aca2ccec8.sig"},
			{Name: "registry.com/myapp:sha256-57a654ace69ec02ba8973093b6a786faa15640575fbf0dbb603db55aca2ccec8.att"},
			{Name: "busybox@" + digest},
		},
		Files: []v1beta1.File{
			{Source: fileURL},
			{Source: fileURL, Checksum: digest},
			{Source: "local.txt"},
		},
		ImageArchives: []v1beta1.ImageArchive{{
			Path: "images.tar",
			Images: []v1beta1.ArchiveImage{
				{Name: "zarf.internal:5000/myapp:1.0"},
				{Name: image},
			},
		}},
	}
	for _, componentConfig := range []bool{false, true} {
		t.Run(fmt.Sprintf("componentConfig=%t", componentConfig), func(t *testing.T) {
			t.Parallel()
			pkg := v1beta1.Package{APIVersion: v1beta1.APIVersion, Components: []v1beta1.Component{{}, {}, {ComponentSpec: component}}}
			path := ".components.[2]"
			if componentConfig {
				pkg.Components = pkg.Components[2:]
				path = ".component"
			}
			findings := CheckPrimitives(convert.PackageFromV1beta1(pkg), componentConfig)
			require.Equal(t, []PackageFinding{
				{RuleID: RuleRepositoryUnpinned, YqPath: path + ".repositories.[0]", Description: "Repository is not pinned to a commit", Item: repoURL, Severity: SevWarn},
				{RuleID: RuleRepositoryUnpinned, YqPath: path + ".repositories.[1]", Description: "Repository is not pinned to a commit", Item: repoURL, Severity: SevWarn},
				{RuleID: RuleRepositoryUnpinned, YqPath: path + ".repositories.[2]", Description: "Repository is not pinned to a commit", Item: repoURL, Severity: SevWarn},
				{RuleID: RuleImageUnpinned, YqPath: path + ".images.[0]", Description: "Image not pinned with digest", Item: image, Severity: SevInfo},
				{RuleID: RuleImageReferenceInvalid, YqPath: path + ".images.[1]", Description: "Failed to parse image reference", Item: invalid, Severity: SevWarn},
				{RuleID: RuleFileChecksumMissing, YqPath: path + ".files.[0]", Description: "No checksum for remote file", Item: fileURL, Severity: SevWarn},
				{RuleID: RuleImageRegistryMissing, YqPath: path + ".images.[5]", Description: "Image reference does not specify a registry domain", Item: "busybox@" + digest, Severity: SevWarn},
				{RuleID: RuleImageArchiveDomain, YqPath: path + ".imageArchives.[0].images.[1].name", Description: "Image archive image should use a .internal domain to avoid resolving to a public registry", Item: image, Severity: SevWarn},
			}, findings)
		})
	}
}
