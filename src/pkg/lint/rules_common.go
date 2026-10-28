// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package lint contains functions for verifying zarf yaml files are valid
package lint

import (
	"strings"

	"github.com/zarf-dev/zarf/src/pkg/helpers"
	"github.com/zarf-dev/zarf/src/pkg/transform"
)

func isPinnedImageReference(image string) (bool, error) {
	transformedImage, err := transform.ParseImageRef(image)
	if err != nil {
		return false, err
	}
	if isCosignSignature(transformedImage.Tag) || isCosignAttestation(transformedImage.Tag) {
		return true, nil
	}
	return transformedImage.Digest != "", nil
}

func isCosignSignature(image string) bool {
	return strings.HasSuffix(image, ".sig")
}

func isCosignAttestation(image string) bool {
	return strings.HasSuffix(image, ".att")
}

// imageDomain returns the registry domain explicitly specified in the image
// reference. An empty string is returned when the reference does not include a
// domain, in which case the registry would default to docker.io. This mirrors
// the domain detection used by the distribution/reference library.
func imageDomain(image string) string {
	image = strings.TrimPrefix(image, helpers.OCIURLPrefix)
	i := strings.IndexRune(image, '/')
	if i == -1 {
		return ""
	}
	prefix := image[:i]
	if strings.ContainsAny(prefix, ".:") || prefix == "localhost" || strings.ToLower(prefix) != prefix {
		return prefix
	}
	return ""
}

// hasInternalDomain returns true if the image's registry domain uses the
// reserved .internal top-level domain, which never resolves on the public
// internet and is the recommended convention for locally-built images.
func hasInternalDomain(image string) bool {
	domain := imageDomain(image)
	// Strip any port so domains such as zarf.internal:5000 are still matched.
	if host, _, ok := strings.Cut(domain, ":"); ok {
		domain = host
	}
	return strings.HasSuffix(domain, ".internal")
}
