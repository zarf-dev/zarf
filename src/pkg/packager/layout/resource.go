// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"encoding/base64"
	"path"
	"strings"

	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
)

const (
	// DocumentationDir contains individually-addressable documentation resources in v1beta1 packages.
	DocumentationDir = "documentation"
	// SBOMResourcesDir contains individually-addressable SBOM resources in v1beta1 packages.
	SBOMResourcesDir = "sboms"

	// ResourceKindAnnotation identifies the kind of a granular package resource.
	ResourceKindAnnotation = "dev.zarf.resource.kind"
	// ResourceKeyAnnotation identifies a granular package resource within its kind.
	ResourceKeyAnnotation = "dev.zarf.resource.key"
	// ResourceKindDocumentation identifies a documentation resource.
	ResourceKindDocumentation = "documentation"
	// ResourceKindSBOM identifies an SBOM resource.
	ResourceKindSBOM = "sbom"
)

// UsesGranularResourceLayout reports whether a package stores documentation and SBOMs as individual files.
// v1alpha1, including definitions that omit apiVersion, retains archive-backed resources.
func UsesGranularResourceLayout(pkg api.Package) bool {
	return pkg.GetAPIVersion() != v1alpha1.APIVersion
}

// DocumentationResourcePath returns the v1beta1 package-relative path for a documentation file.
func DocumentationResourcePath(fileName string) string {
	return path.Join(DocumentationDir, fileName)
}

// SBOMResourcePath returns the v1beta1 package-relative path for an SBOM resource key.
func SBOMResourcePath(key string) string {
	if componentName, ok := strings.CutPrefix(key, "component:"); ok {
		return path.Join(SBOMResourcesDir, "files", componentName+".json")
	}
	return path.Join(SBOMResourcesDir, "images", base64.RawURLEncoding.EncodeToString([]byte(key))+".json")
}

// SBOMResourceKey returns the resource key encoded in a v1beta1 SBOM resource path.
func SBOMResourceKey(resourcePath string) (string, bool) {
	switch path.Dir(resourcePath) {
	case path.Join(SBOMResourcesDir, "files"):
		componentName := strings.TrimSuffix(path.Base(resourcePath), ".json")
		if componentName == "" || path.Ext(resourcePath) != ".json" {
			return "", false
		}
		return "component:" + componentName, true
	case path.Join(SBOMResourcesDir, "images"):
		if path.Ext(resourcePath) != ".json" {
			return "", false
		}
		encodedKey := strings.TrimSuffix(path.Base(resourcePath), ".json")
		key, err := base64.RawURLEncoding.DecodeString(encodedKey)
		if err != nil || !strings.HasPrefix(string(key), "image:") {
			return "", false
		}
		return string(key), true
	default:
		return "", false
	}
}
