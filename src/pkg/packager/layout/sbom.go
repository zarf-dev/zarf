// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"encoding/base64"
	"path"
	"regexp"
	"strings"

	"github.com/zarf-dev/zarf/src/api"
)

const (
	// SBOMResourcesDir contains individually-addressable SBOM resources in v1beta1 packages.
	SBOMResourcesDir = "sboms"
	// SBOMMediaTypeSyftJSON identifies an SBOM encoded in Syft's JSON format.
	SBOMMediaTypeSyftJSON = "application/vnd.syft+json"
)

// SBOMResourceKind identifies the source represented by an SBOM resource.
type SBOMResourceKind string

const (
	// SBOMResourceComponent identifies an SBOM generated from component files.
	SBOMResourceComponent SBOMResourceKind = "component"
	// SBOMResourceImage identifies an SBOM generated from an image.
	SBOMResourceImage SBOMResourceKind = "image"
)

// ComponentSBOMResourcePath returns the v1beta1 package-relative path for a component SBOM.
func ComponentSBOMResourcePath(componentName string) string {
	return path.Join(SBOMResourcesDir, "files", componentName+".json")
}

// ImageSBOMResourcePath returns the v1beta1 package-relative path for an image SBOM.
func ImageSBOMResourcePath(identifier string) string {
	return path.Join(SBOMResourcesDir, "images", base64.RawURLEncoding.EncodeToString([]byte(identifier))+".json")
}

// ParseSBOMResourcePath returns the kind and identifier encoded in a v1beta1 SBOM resource path.
func ParseSBOMResourcePath(resourcePath string) (SBOMResourceKind, string, bool) {
	switch path.Dir(resourcePath) {
	case path.Join(SBOMResourcesDir, "files"):
		componentName := strings.TrimSuffix(path.Base(resourcePath), ".json")
		if componentName == "" || path.Ext(resourcePath) != ".json" {
			return "", "", false
		}
		return SBOMResourceComponent, componentName, true
	case path.Join(SBOMResourcesDir, "images"):
		if path.Ext(resourcePath) != ".json" {
			return "", "", false
		}
		encodedIdentifier := strings.TrimSuffix(path.Base(resourcePath), ".json")
		identifier, err := base64.RawURLEncoding.DecodeString(encodedIdentifier)
		if err != nil || len(identifier) == 0 {
			return "", "", false
		}
		return SBOMResourceImage, string(identifier), true
	default:
		return "", "", false
	}
}

// SBOMImageReferenceSet returns the image references declared by the package.
func SBOMImageReferenceSet(pkg api.Package) map[string]struct{} {
	references := make(map[string]struct{})
	for _, component := range pkg.Components {
		for _, imageReference := range component.GetImages() {
			references[imageReference] = struct{}{}
		}
	}
	return references
}

// SBOMKeyMatchesResource reports whether a requested key selects an SBOM resource.
func SBOMKeyMatchesResource(kind SBOMResourceKind, identifier, key string, imageReferences map[string]struct{}) bool {
	if identifier == key {
		return true
	}
	if kind != SBOMResourceImage {
		return false
	}
	if _, declared := imageReferences[key]; !declared {
		return false
	}
	return strings.HasPrefix(identifier, key+"-")
}

var legacySBOMFilenamePattern = regexp.MustCompile(`(?m)[^a-zA-Z0-9\.\-]`)

// NormalizeSBOMFilename returns the filename used for archive-backed SBOMs.
func NormalizeSBOMFilename(identifier string) string {
	return legacySBOMFilenamePattern.ReplaceAllString(identifier, "_")
}
