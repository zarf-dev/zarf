// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package transform provides helper functions to transform URLs to airgap equivalents
package transform

import (
	"fmt"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"io"
	"os"
	"strings"

	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/distribution/reference"
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
	image, err := ParseImageRef(srcReference)
	if err != nil {
		return "", err
	}

	// check if image has already been transformed
	if strings.HasPrefix(targetHost, image.Host) {
		return srcReference, nil
	}

	// Generate a crc32 hash of the image host + name
	checksum := helpers.GetCRCHash(image.Name)

	// If this image is specified by digest then don't add a checksum it as it will already be a specific SHA
	if image.Digest != "" {
		return fmt.Sprintf("%s/%s@%s", targetHost, image.Path, image.Digest), nil
	}

	return fmt.Sprintf("%s/%s:%s-zarf-%d", targetHost, image.Path, image.Tag, checksum), nil
}

// ImageTransformHostWithoutChecksum replaces the base url for an image but avoids adding a checksum of the original url (note image refs are not full URLs).
func ImageTransformHostWithoutChecksum(targetHost, srcReference string) (string, error) {
	image, err := ParseImageRef(srcReference)
	if err != nil {
		return "", err
	}

	// check if image has already been transformed
	if strings.HasPrefix(targetHost, image.Host) {
		return srcReference, nil
	}

	return fmt.Sprintf("%s/%s%s", targetHost, image.Path, image.TagOrDigest), nil
}

// ParseImageRef parses a source reference into an Image struct
func ParseImageRef(srcReference string) (out Image, err error) {
	if IsTarball(srcReference) {
		out, err = TarballToImage(srcReference)
		if err != nil {
			return out, err
		}
	} else {
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

		// If no tag or digest was provided use the default tag (latest)
		if out.TagOrDigest == "" {
			out.Tag = "latest"
			out.TagOrDigest = ":latest"
			out.Reference += ":latest"
		}
	}

	return out, nil
}

// TarballToImage converts a tarball to an Image struct
func TarballToImage(srcReference string) (Image, error) {
	var (
		digest   string
		repoTags []string
		out      = Image{}
		err      error
	)
	if digest, err = getDigestFromTarball(srcReference); err != nil {
		return out, err
	}
	if repoTags, err = getRepoTagsFromTarball(srcReference); err != nil {
		return out, err
	}
	imageRef := repoTags[0]

	splitTag := strings.Split(repoTags[0], ":")
	imagePath := splitTag[0]

	splitRepo := strings.Split(splitTag[0], "/")
	imageName := splitRepo[len(splitRepo)-1]

	out.Name = imageName
	out.Digest = digest
	out.TagOrDigest = fmt.Sprintf("@%s", digest)
	out.Host = ""
	out.Path = imagePath
	out.Reference = imageRef
	if len(splitTag) > 1 {
		out.Tag = splitTag[1]
	}
	return out, nil
}

// IsTarball checks if a given reference is a tarball
func IsTarball(srcReference string) bool {
	return strings.HasSuffix(srcReference, ".tar") || strings.HasSuffix(srcReference, ".tar.gz") || strings.HasSuffix(srcReference, ".tgz")
}

func getDigestFromTarball(path string) (string, error) {
	var digest string
	if img, err := tarball.ImageFromPath(path, nil); err != nil {
		return digest, err
	} else {
		if digestHash, err := img.Digest(); err != nil {
			return digest, err
		} else {
			return digestHash.String(), nil
		}
	}
}

func getRepoTagsFromTarball(path string) ([]string, error) {
	if manifest, err := tarball.LoadManifest(func() (io.ReadCloser, error) {
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		return file, nil
	}); err != nil {
		return nil, err
	} else {
		return manifest[0].RepoTags, nil
	}
}
