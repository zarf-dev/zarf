// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic helper functions.
package utils

import (
	"fmt"

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

// SwapHost Perform base url replacement and adds a crc32 of the original url to the end of the src.
func SwapHost(src string, targetHost string) (string, error) {
	image, err := parseImageURL(src)
	if err != nil {
		return "", err
	}

	// Generate a crc32 hash of the image host + name
	checksum := GetCRCHash(image.Name)

	return fmt.Sprintf("%s/%s-%d%s", targetHost, image.Path, checksum, image.TagOrDigest), nil
}

// SwapHostWithoutChecksum Perform base url replacement but avoids adding a checksum of the original url.
func SwapHostWithoutChecksum(src string, targetHost string) (string, error) {
	image, err := parseImageURL(src)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%s%s", targetHost, image.Path, image.TagOrDigest), nil
}

func parseImageURL(src string) (out Image, err error) {
	ref, err := reference.ParseAnyReference(src)
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
		return out, fmt.Errorf("unable to parse image name from %s", src)
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
