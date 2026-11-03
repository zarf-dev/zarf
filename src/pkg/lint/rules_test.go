// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package lint contains functions for verifying zarf yaml files are valid
package lint

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/convert"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
)

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

func TestImagesWithoutDomain(t *testing.T) {
	t.Parallel()

	component := v1alpha1.ZarfComponent{
		Images: []string{
			"myapp:1.0.0",
			"library/myapp:1.0.0",
			"docker.io/library/myapp:1.0.0",
			"ghcr.io/zarf-dev/zarf:v0.1.0",
			"localhost:5000/myapp:1.0.0",
			"registry.com:8080/zarf-dev/whatever",
			"###ZARF_PKG_TMPL_IMAGE###",
			"###ZARF_PKG_VAR_IMAGE###",
			"busybox:###ZARF_PKG_TMPL_BUSYBOX_IMAGE###",
		},
	}
	findings := CheckPrimitives(convert.PackageFromV1alpha1(v1alpha1.ZarfPackage{Components: []v1alpha1.ZarfComponent{component}}), false)
	var expected []PackageFinding
	for i, image := range component.Images[:6] {
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
					"###ZARF_PKG_VAR_IMAGE###",
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
		},
		Images: []v1beta1.Image{
			{Name: image},
			{Name: invalid},
			{Name: "registry.com/myapp@" + digest},
			{Name: "registry.com/myapp:sha256-57a654ace69ec02ba8973093b6a786faa15640575fbf0dbb603db55aca2ccec8.sig"},
			{Name: "registry.com/myapp:sha256-57a654ace69ec02ba8973093b6a786faa15640575fbf0dbb603db55aca2ccec8.att"},
			{Name: "busybox@" + digest},
			{Name: "###ZARF_PKG_TMPL_IMAGE###"},
			{Name: "###ZARF_PKG_VAR_IMAGE###"},
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
				{Name: "###ZARF_PKG_VAR_IMAGE###"},
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
				{RuleID: RuleImageUnpinned, YqPath: path + ".images.[0]", Description: "Image not pinned with digest", Item: image, Severity: SevInfo},
				{RuleID: RuleImageReferenceInvalid, YqPath: path + ".images.[1]", Description: "Failed to parse image reference", Item: invalid, Severity: SevWarn},
				{RuleID: RuleImageReferenceInvalid, YqPath: path + ".images.[6]", Description: "Failed to parse image reference", Item: "###ZARF_PKG_TMPL_IMAGE###", Severity: SevWarn},
				{RuleID: RuleImageReferenceInvalid, YqPath: path + ".images.[7]", Description: "Failed to parse image reference", Item: "###ZARF_PKG_VAR_IMAGE###", Severity: SevWarn},
				{RuleID: RuleFileChecksumMissing, YqPath: path + ".files.[0]", Description: "No checksum for remote file", Item: fileURL, Severity: SevWarn},
				{RuleID: RuleImageRegistryMissing, YqPath: path + ".images.[5]", Description: "Image reference does not specify a registry domain", Item: "busybox@" + digest, Severity: SevWarn},
				{RuleID: RuleImageRegistryMissing, YqPath: path + ".images.[6]", Description: "Image reference does not specify a registry domain", Item: "###ZARF_PKG_TMPL_IMAGE###", Severity: SevWarn},
				{RuleID: RuleImageRegistryMissing, YqPath: path + ".images.[7]", Description: "Image reference does not specify a registry domain", Item: "###ZARF_PKG_VAR_IMAGE###", Severity: SevWarn},
				{RuleID: RuleImageArchiveDomain, YqPath: path + ".imageArchives.[0].images.[1].name", Description: "Image archive image should use a .internal domain to avoid resolving to a public registry", Item: image, Severity: SevWarn},
				{RuleID: RuleImageArchiveDomain, YqPath: path + ".imageArchives.[0].images.[2].name", Description: "Image archive image should use a .internal domain to avoid resolving to a public registry", Item: "###ZARF_PKG_VAR_IMAGE###", Severity: SevWarn},
			}, findings)
		})
	}
}

func TestRepositoryPinning(t *testing.T) {
	t.Parallel()
	const repoURL = "https://example.com/repo.git"
	alphaWarning := PackageFinding{
		RuleID: RuleRepositoryUnpinned, YqPath: ".components.[0].repos.[0]",
		Description: "Unpinned repository", Item: repoURL, Severity: SevWarn,
	}
	betaWarning := PackageFinding{
		RuleID: RuleRepositoryUnpinned, YqPath: ".components.[0].repositories.[0]",
		Description: "Repository is not pinned to a commit", Item: repoURL, Severity: SevWarn,
	}
	tests := []struct {
		name            string
		apiVersion      string
		repository      api.Repository
		componentConfig bool
		want            []PackageFinding
	}{
		{
			name:       "an omitted API version uses alpha pinning",
			repository: api.Repository{URL: repoURL}, want: []PackageFinding{alphaWarning},
		},
		{
			name: "an unsupported API version has no primitive findings", apiVersion: "unknown",
			repository: api.Repository{URL: repoURL},
		},
		{
			name: "alpha warns when a URL has no ref", apiVersion: v1alpha1.APIVersion,
			repository: api.Repository{URL: repoURL}, want: []PackageFinding{alphaWarning},
		},
		{
			name: "alpha warns when the structured ref is empty", apiVersion: v1alpha1.APIVersion,
			repository: api.Repository{URL: repoURL, Ref: &api.GitRef{}}, want: []PackageFinding{alphaWarning},
		},
		{
			name: "alpha accepts a tag", apiVersion: v1alpha1.APIVersion,
			repository: api.Repository{URL: repoURL, Ref: &api.GitRef{Tag: "v1"}},
		},
		{
			name: "alpha accepts a branch", apiVersion: v1alpha1.APIVersion,
			repository: api.Repository{URL: repoURL, Ref: &api.GitRef{Branch: "main"}},
		},
		{
			name: "alpha accepts a commit", apiVersion: v1alpha1.APIVersion,
			repository: api.Repository{URL: repoURL, Ref: &api.GitRef{Commit: "commit"}},
		},
		{
			name: "alpha accepts multiple nonempty ref fields", apiVersion: v1alpha1.APIVersion,
			repository: api.Repository{URL: repoURL + "@old", Ref: &api.GitRef{Tag: "v1", Commit: "commit", Branch: "main"}},
		},
		{
			name: "alpha accepts an authored inline ref", apiVersion: v1alpha1.APIVersion,
			repository: api.Repository{URL: "https://dev.azure.com/zarf-dev/zarf-public-test/_git/zarf-public-test@v0.0.1"},
		},
		{
			name: "alpha accepts the presence of @ even without a ref", apiVersion: v1alpha1.APIVersion,
			repository: api.Repository{URL: repoURL + "@"},
		},
		{
			name: "alpha pinning does not validate the repository URL", apiVersion: v1alpha1.APIVersion,
			repository: api.Repository{URL: "not a URL", Ref: &api.GitRef{Branch: "main"}},
		},
		{
			name: "alpha warns on an invalid URL without a ref", apiVersion: v1alpha1.APIVersion,
			repository: api.Repository{URL: "not a URL"},
			want:       []PackageFinding{{RuleID: RuleRepositoryUnpinned, YqPath: alphaWarning.YqPath, Description: alphaWarning.Description, Item: "not a URL", Severity: SevWarn}},
		},
		{
			name: "alpha checks the authored URL even when a structured commit exists", apiVersion: v1alpha1.APIVersion,
			repository: api.Repository{URL: repoURL, LegacyURL: "authored-repo", Ref: &api.GitRef{Commit: "commit"}},
			want:       []PackageFinding{{RuleID: RuleRepositoryUnpinned, YqPath: alphaWarning.YqPath, Description: alphaWarning.Description, Item: "authored-repo", Severity: SevWarn}},
		},
		{
			name: "alpha accepts a ref in the authored URL", apiVersion: v1alpha1.APIVersion,
			repository: api.Repository{URL: repoURL, LegacyURL: repoURL + "@refs/tags/v1"},
		},
		{
			name: "alpha retains package paths for component selection", apiVersion: v1alpha1.APIVersion,
			repository: api.Repository{URL: repoURL}, componentConfig: true, want: []PackageFinding{alphaWarning},
		},
		{
			name: "beta requires a commit instead of a tag", apiVersion: v1beta1.APIVersion,
			repository: api.Repository{URL: repoURL, Ref: &api.GitRef{Tag: "v1"}}, want: []PackageFinding{betaWarning},
		},
		{
			name: "beta requires a commit instead of a branch", apiVersion: v1beta1.APIVersion,
			repository: api.Repository{URL: repoURL, Ref: &api.GitRef{Branch: "main"}}, want: []PackageFinding{betaWarning},
		},
		{
			name: "beta accepts a structured commit", apiVersion: v1beta1.APIVersion,
			repository: api.Repository{URL: repoURL, Ref: &api.GitRef{Commit: "commit"}},
		},
		{
			name: "beta accepts a structured commit despite an unpinned legacy URL", apiVersion: v1beta1.APIVersion,
			repository: api.Repository{URL: repoURL, LegacyURL: "authored-repo", Ref: &api.GitRef{Commit: "commit"}},
		},
		{
			name: "beta does not accept a ref from the legacy URL", apiVersion: v1beta1.APIVersion,
			repository: api.Repository{URL: repoURL, LegacyURL: repoURL + "@refs/tags/v1"}, want: []PackageFinding{betaWarning},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pkg := api.Package{APIVersion: tc.apiVersion, Components: []api.Component{{Repositories: []api.Repository{tc.repository}}}}
			require.Equal(t, tc.want, CheckPrimitives(pkg, tc.componentConfig))
		})
	}
}

func TestFileChecksumWarnings(t *testing.T) {
	t.Parallel()
	const fileURL = "https://example.com/file"
	alphaWarning := PackageFinding{
		RuleID: RuleFileChecksumMissing, YqPath: ".components.[0].files.[0]",
		Description: "No shasum for remote file", Item: fileURL, Severity: SevWarn,
	}
	betaWarning := PackageFinding{
		RuleID: RuleFileChecksumMissing, YqPath: ".components.[0].files.[0]",
		Description: "No checksum for remote file", Item: fileURL, Severity: SevWarn,
	}
	tests := []struct {
		name       string
		apiVersion string
		checksum   api.FileChecksum
		want       []PackageFinding
	}{
		{name: "alpha warns without a checksum", apiVersion: v1alpha1.APIVersion, want: []PackageFinding{alphaWarning}},
		{name: "beta warns without a checksum", apiVersion: v1beta1.APIVersion, want: []PackageFinding{betaWarning}},
		{
			name: "alpha accepts a digest with the default SHA-256 algorithm", apiVersion: v1alpha1.APIVersion,
			checksum: api.FileChecksum{Digest: "digest"},
		},
		{
			name: "beta accepts a digest with the default algorithm", apiVersion: v1beta1.APIVersion,
			checksum: api.FileChecksum{Digest: "digest"},
		},
		{
			name: "alpha accepts explicit SHA-256", apiVersion: v1alpha1.APIVersion,
			checksum: api.FileChecksum{Algorithm: api.ChecksumSHA256, Digest: "digest"},
		},
		{
			name: "beta accepts explicit SHA-256", apiVersion: v1beta1.APIVersion,
			checksum: api.FileChecksum{Algorithm: api.ChecksumSHA256, Digest: "digest"},
		},
		{
			name: "alpha warns because its shasum format only supports SHA-256", apiVersion: v1alpha1.APIVersion,
			checksum: api.FileChecksum{Algorithm: api.ChecksumSHA512, Digest: "digest"}, want: []PackageFinding{alphaWarning},
		},
		{
			name: "beta accepts SHA-512", apiVersion: v1beta1.APIVersion,
			checksum: api.FileChecksum{Algorithm: api.ChecksumSHA512, Digest: "digest"},
		},
		{
			name: "alpha warns when SHA-256 has no digest", apiVersion: v1alpha1.APIVersion,
			checksum: api.FileChecksum{Algorithm: api.ChecksumSHA256}, want: []PackageFinding{alphaWarning},
		},
		{
			name: "beta presence checking leaves missing digests to input validation", apiVersion: v1beta1.APIVersion,
			checksum: api.FileChecksum{Algorithm: api.ChecksumSHA256},
		},
		{
			name: "alpha cannot represent an unknown algorithm", apiVersion: v1alpha1.APIVersion,
			checksum: api.FileChecksum{Algorithm: "unknown", Digest: "digest"}, want: []PackageFinding{alphaWarning},
		},
		{
			name: "beta presence checking leaves algorithm validity to input validation", apiVersion: v1beta1.APIVersion,
			checksum: api.FileChecksum{Algorithm: "unknown", Digest: "digest"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pkg := api.Package{APIVersion: tc.apiVersion, Components: []api.Component{{Files: []api.File{{Source: fileURL, Checksum: tc.checksum}, {Source: "local.txt"}}}}}
			require.Equal(t, tc.want, CheckPrimitives(pkg, false))
		})
	}
}
