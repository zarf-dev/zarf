// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package lint

import (
	"fmt"

	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/pkg/helpers"
)

// CheckComponentValuesV1Beta1 checks recommended practices on a resolved v1beta1 component.
// path is the component's location in the source definition.
func CheckComponentValuesV1Beta1(c v1beta1.ComponentSpec, path string) []PackageFinding {
	var findings []PackageFinding
	findings = append(findings, checkForUnpinnedReposV1Beta1(c, path)...)
	findings = append(findings, checkForUnpinnedImagesV1Beta1(c, path)...)
	findings = append(findings, checkForUnpinnedFilesV1Beta1(c, path)...)
	findings = append(findings, checkForImagesWithoutDomainV1Beta1(c, path)...)
	findings = append(findings, checkForImageArchivesWithoutInternalDomainV1Beta1(c, path)...)
	return findings
}

func checkForUnpinnedReposV1Beta1(c v1beta1.ComponentSpec, path string) []PackageFinding {
	var findings []PackageFinding
	for i, repo := range c.Repositories {
		if repo.Ref == nil || repo.Ref.Commit == "" {
			findings = append(findings, PackageFinding{
				YqPath:      fmt.Sprintf("%s.repositories.[%d]", path, i),
				Description: "Repository is not pinned to a commit",
				Item:        repo.URL,
				Severity:    SevWarn,
			})
		}
	}
	return findings
}

func checkForUnpinnedImagesV1Beta1(c v1beta1.ComponentSpec, path string) []PackageFinding {
	var findings []PackageFinding
	for i, image := range c.Images {
		imagePath := fmt.Sprintf("%s.images.[%d]", path, i)
		pinned, err := isPinnedImageReference(image.Name)
		switch {
		case err != nil:
			findings = append(findings, PackageFinding{
				YqPath:      imagePath,
				Description: "Failed to parse image reference",
				Item:        image.Name,
				Severity:    SevWarn,
			})
		case !pinned:
			findings = append(findings, PackageFinding{
				YqPath:      imagePath,
				Description: "Image not pinned with digest",
				Item:        image.Name,
				Severity:    SevWarn,
			})
		}
	}
	return findings
}

func checkForImagesWithoutDomainV1Beta1(c v1beta1.ComponentSpec, path string) []PackageFinding {
	var findings []PackageFinding
	for i, image := range c.Images {
		if imageDomain(image.Name) == "" {
			findings = append(findings, PackageFinding{
				YqPath:      fmt.Sprintf("%s.images.[%d]", path, i),
				Description: "Image reference does not specify a registry domain",
				Item:        image.Name,
				Severity:    SevWarn,
			})
		}
	}
	return findings
}

func checkForUnpinnedFilesV1Beta1(c v1beta1.ComponentSpec, path string) []PackageFinding {
	var findings []PackageFinding
	for i, file := range c.Files {
		if file.Checksum == "" && helpers.IsURL(file.Source) {
			findings = append(findings, PackageFinding{
				YqPath:      fmt.Sprintf("%s.files.[%d]", path, i),
				Description: "No shasum for remote file",
				Item:        file.Source,
				Severity:    SevWarn,
			})
		}
	}
	return findings
}

func checkForImageArchivesWithoutInternalDomainV1Beta1(c v1beta1.ComponentSpec, path string) []PackageFinding {
	var findings []PackageFinding
	for i, archive := range c.ImageArchives {
		for j, image := range archive.Images {
			if !hasInternalDomain(image) {
				findings = append(findings, PackageFinding{
					YqPath:      fmt.Sprintf("%s.imageArchives.[%d].images.[%d]", path, i, j),
					Description: "Image archive image should use a .internal domain to avoid resolving to a public registry",
					Item:        image,
					Severity:    SevWarn,
				})
			}
		}
	}
	return findings
}
