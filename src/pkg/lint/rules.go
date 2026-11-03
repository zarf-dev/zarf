// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package lint

import (
	"fmt"
	"strings"

	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/pkg/helpers"
)

// Stable semantic rule IDs.
const (
	RuleRepositoryUnpinned    = "repository-unpinned"
	RuleImageReferenceInvalid = "image-reference-invalid"
	RuleImageUnpinned         = "image-unpinned"
	RuleImageRegistryMissing  = "image-registry-missing"
	RuleFileChecksumMissing   = "file-checksum-missing"
	RuleImageArchiveDomain    = "image-archive-domain"
	RuleValuesSchemaMissing   = "values-schema-missing"
	RuleValuesUnused          = "values-unused"
	RuleTemplateSyntax        = "template-syntax"
	RuleTemplateInputMissing  = "template-input-missing"
	RuleTemplateRender        = "template-render"
	RuleValuesMapping         = "values-mapping"
	RuleChartValuesSchema     = "chart-values-schema"
	RuleChartSchemaSkipped    = "chart-schema-skipped"
)

var skippableRules = map[string]struct{}{
	RuleRepositoryUnpinned:    {},
	RuleImageReferenceInvalid: {},
	RuleImageUnpinned:         {},
	RuleImageRegistryMissing:  {},
	RuleFileChecksumMissing:   {},
	RuleImageArchiveDomain:    {},
	RuleValuesSchemaMissing:   {},
	RuleValuesUnused:          {},
	RuleTemplateSyntax:        {},
	RuleTemplateInputMissing:  {},
	RuleTemplateRender:        {},
	RuleValuesMapping:         {},
	RuleChartValuesSchema:     {},
	RuleChartSchemaSkipped:    {},
}

// ValidateSkipRules rejects unknown rule IDs.
func ValidateSkipRules(rules []string) error {
	for _, rule := range rules {
		if _, ok := skippableRules[rule]; !ok {
			return fmt.Errorf("unknown lint rule %q", rule)
		}
	}
	return nil
}

// CheckPrimitives checks recommended practices on a resolved package's repositories,
// images, files, and image archives. componentConfig selects singular component paths.
func CheckPrimitives(pkg api.Package, componentConfig bool) []PackageFinding {
	version := pkg.GetAPIVersion()
	if version != v1alpha1.APIVersion && version != v1beta1.APIVersion {
		return nil
	}
	legacy := version == v1alpha1.APIVersion
	var findings []PackageFinding
	for i, component := range pkg.Components {
		path := fmt.Sprintf(".components.[%d]", i)
		if componentConfig && !legacy {
			path = ".component"
		}
		findings = append(findings, checkRepositories(component.Repositories, path, legacy)...)
		findings = append(findings, checkImagePins(component.Images, path, legacy)...)
		findings = append(findings, checkFileChecksums(component.Files, path, legacy)...)
		findings = append(findings, checkImageDomains(component.Images, path, legacy)...)
		findings = append(findings, checkImageArchiveDomains(component.ImageArchives, path, legacy)...)
	}
	return findings
}

func checkRepositories(repositories []api.Repository, path string, legacy bool) []PackageFinding {
	description := "Repository is not pinned to a commit"
	field := "repositories"
	if legacy {
		description = "Unpinned repository"
		field = "repos"
	}
	var findings []PackageFinding
	for i, repo := range repositories {
		item := repo.URL
		pinned := repo.Ref != nil && repo.Ref.Commit != ""
		if legacy {
			if repo.LegacyURL != "" {
				item = repo.LegacyURL
			}
			pinned = strings.Contains(item, "@")
			// Without an authored URL, alpha conversion appends @ref for any
			// nonempty tag, commit, or branch. All such locators count as pinned.
			if repo.LegacyURL == "" && repo.Ref != nil {
				pinned = pinned || repo.Ref.Tag != "" || repo.Ref.Commit != "" || repo.Ref.Branch != ""
			}
		}
		if !pinned {
			findings = append(findings, PackageFinding{
				RuleID: RuleRepositoryUnpinned, YqPath: fmt.Sprintf("%s.%s.[%d]", path, field, i),
				Description: description, Item: item, Severity: SevWarn,
			})
		}
	}
	return findings
}

func checkImagePins(images []api.Image, path string, legacy bool) []PackageFinding {
	var findings []PackageFinding
	for i, image := range images {
		pinned, err := isPinnedImageReference(image.Name)
		if err != nil && legacy && isTemplatedImage(image.Name) {
			continue
		}
		imagePath := fmt.Sprintf("%s.images.[%d]", path, i)
		switch {
		case err != nil:
			findings = append(findings, PackageFinding{
				RuleID: RuleImageReferenceInvalid, YqPath: imagePath,
				Description: "Failed to parse image reference", Item: image.Name, Severity: SevWarn,
			})
		case !pinned:
			findings = append(findings, PackageFinding{
				RuleID: RuleImageUnpinned, YqPath: imagePath,
				Description: "Image not pinned with digest", Item: image.Name, Severity: SevInfo,
			})
		}
	}
	return findings
}

func checkFileChecksums(files []api.File, path string, legacy bool) []PackageFinding {
	description := "No checksum for remote file"
	if legacy {
		description = "No shasum for remote file"
	}
	var findings []PackageFinding
	for i, file := range files {
		checksumSet := file.Checksum.IsSet()
		if legacy {
			checksumSet = file.Checksum.GetAlgorithm() == api.ChecksumSHA256 && file.Checksum.Digest != ""
		}
		if !checksumSet && helpers.IsURL(file.Source) {
			findings = append(findings, PackageFinding{
				RuleID: RuleFileChecksumMissing, YqPath: fmt.Sprintf("%s.files.[%d]", path, i),
				Description: description, Item: file.Source, Severity: SevWarn,
			})
		}
	}
	return findings
}

func checkImageDomains(images []api.Image, path string, legacy bool) []PackageFinding {
	var findings []PackageFinding
	for i, image := range images {
		if legacy && isTemplatedImage(image.Name) {
			continue
		}
		if imageDomain(image.Name) == "" {
			findings = append(findings, PackageFinding{
				RuleID: RuleImageRegistryMissing, YqPath: fmt.Sprintf("%s.images.[%d]", path, i),
				Description: "Image reference does not specify a registry domain", Item: image.Name, Severity: SevWarn,
			})
		}
	}
	return findings
}

func checkImageArchiveDomains(archives []api.ImageArchive, path string, legacy bool) []PackageFinding {
	suffix := ".name"
	if legacy {
		suffix = ""
	}
	var findings []PackageFinding
	for i, archive := range archives {
		for j, image := range archive.Images {
			if legacy && isTemplatedImage(image.Name) {
				continue
			}
			if !hasInternalDomain(image.Name) {
				findings = append(findings, PackageFinding{
					RuleID: RuleImageArchiveDomain, YqPath: fmt.Sprintf("%s.imageArchives.[%d].images.[%d]%s", path, i, j, suffix),
					Description: "Image archive image should use a .internal domain to avoid resolving to a public registry", Item: image.Name, Severity: SevWarn,
				})
			}
		}
	}
	return findings
}
