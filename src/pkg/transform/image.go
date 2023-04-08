// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package transform provides helper functions to transform URLs to airgap equivalents
package transform

import (
	"fmt"

	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/distribution/distribution/reference"
)

// Image represents a config for an OCI image.
type Image struct {
	Host        string
	Name        string
	Path        string
	Tag         string
	Digest      string
	Reference   string
	TagOrDigest string
}

// ImageTransformHost replaces the base url for an image and adds a crc32 of the original url to the end of the src (note image refs are not full URLs).
func ImageTransformHost(targetHost, srcReference string) (string, error) {
	image, err := ParseImageURL(srcReference)
	if err != nil {
		return "", err
	}

	// Generate a crc32 hash of the image host + name
	checksum := utils.GetCRCHash(image.Name)

	return fmt.Sprintf("%s/%s-%d%s", targetHost, image.Path, checksum, image.TagOrDigest), nil
}

// ImageTransformHostWithoutChecksum replaces the base url for an image but avoids adding a checksum of the original url (note image refs are not full URLs).
func ImageTransformHostWithoutChecksum(targetHost, srcReference string) (string, error) {
	image, err := ParseImageURL(srcReference)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%s%s", targetHost, image.Path, image.TagOrDigest), nil
}

// ParseImageURL parses a source reference into an Image struct
func ParseImageURL(srcReference string) (out Image, err error) {
	// TODO (@WSTARR) Find a better potential home for this
	ref, err := reference.ParseAnyReference(srcReference)
	if err != nil {
		return out, err
	}

	// Parse the reference into its components
	if named, ok := ref.(reference.Named); ok {
		out.Name = named.Name()
		out.Path = reference.Path(named)
		out.Host = reference.Domain(named)
		out.Reference = ref.String()
	} else {
		return out, fmt.Errorf("unable to parse image name from %s", srcReference)
	}

	// Parse the tag and add it to digestOrReference
	if tagged, ok := ref.(reference.Tagged); ok {
		out.Tag = tagged.Tag()
		out.TagOrDigest = fmt.Sprintf(":%s", tagged.Tag())
	}

	// Parse the digest and override digestOrReference
	if digested, ok := ref.(reference.Digested); ok {
		out.Digest = digested.Digest().String()
		out.TagOrDigest = fmt.Sprintf("@%s", digested.Digest().String())
	}

	return out, nil
}
