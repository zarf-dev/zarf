// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"encoding/base64"
	"fmt"
	"path"
	"regexp"
	"strings"
)

const (
	// SBOMResourcesDir contains individually-addressable SBOM resources in v1beta1 packages.
	SBOMResourcesDir = "sboms"
	// SBOMMediaTypeSyftJSON identifies an SBOM encoded in Syft's JSON format.
	SBOMMediaTypeSyftJSON     = "application/vnd.syft+json"
	legacySBOMComponentPrefix = "zarf-component-"
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

func legacySBOMArchiveFiles(keys []string) ([]string, error) {
	files := make([]string, 0, len(keys))
	keysByFile := make(map[string]string, len(keys))
	for _, key := range keys {
		file, err := legacySBOMArchiveFile(key)
		if err != nil {
			return nil, err
		}
		if conflictingKey, exists := keysByFile[file]; exists && conflictingKey != key {
			return nil, fmt.Errorf("legacy SBOM resource keys %q and %q both normalize to %q", conflictingKey, key, file)
		}
		keysByFile[file] = key
		files = append(files, file)
	}
	return files, nil
}

func legacySBOMArchiveFile(key string) (string, error) {
	switch {
	case strings.HasPrefix(key, "component:"):
		componentName := strings.TrimPrefix(key, "component:")
		if componentName == "" {
			return "", fmt.Errorf("invalid legacy SBOM component key %q", key)
		}
		return NormalizeSBOMFilename(legacySBOMComponentPrefix + componentName + ".json"), nil
	case strings.HasPrefix(key, "image:"):
		imageIdentifier := strings.TrimPrefix(key, "image:")
		if imageIdentifier == "" {
			return "", fmt.Errorf("invalid legacy SBOM image key %q", key)
		}
		return NormalizeSBOMFilename(imageIdentifier + ".json"), nil
	default:
		return "", fmt.Errorf("invalid legacy SBOM resource key %q", key)
	}
}
