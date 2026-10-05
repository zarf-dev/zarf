// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"encoding/base64"
	"path"
	"regexp"
	"strings"
)

const (
	// SBOMResourcesDir contains individually-addressable SBOM resources in v1beta1 packages.
	SBOMResourcesDir = "sboms"
)

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

var legacySBOMFilenamePattern = regexp.MustCompile(`(?m)[^a-zA-Z0-9\.\-]`)

// NormalizeSBOMFilename returns the filename used for archive-backed SBOMs.
func NormalizeSBOMFilename(identifier string) string {
	return legacySBOMFilenamePattern.ReplaceAllString(identifier, "_")
}
