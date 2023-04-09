// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package transform provides helper functions to transform URLs to airgap equivalents
package transform

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var imageRefs = []string{
	"nginx:1.23.3",
	"defenseunicorns/zarf-agent:v0.22.1",
	"defenseunicorns/zarf-agent@sha256:84605f731c6a18194794c51e70021c671ab064654b751aa57e905bce55be13de",
	"ghcr.io/stefanprodan/podinfo:6.3.3",
	"registry1.dso.mil/ironbank/opensource/defenseunicorns/zarf/zarf-agent:v0.25.0",
}

var badImageRefs = []string{
	"i am not a ref at all",
	"C:\\Users\\zarf",
	"http://urls.are/not/refs",
}

func TestImageTransformHost(t *testing.T) {
	var expectedResult = []string{
		// Normal git repos and references for pushing/pulling
		"gitlab.com/project/library/nginx-3793515731:1.23.3",
		"gitlab.com/project/defenseunicorns/zarf-agent-4283503412:v0.22.1",
		"gitlab.com/project/defenseunicorns/zarf-agent-4283503412@sha256:84605f731c6a18194794c51e70021c671ab064654b751aa57e905bce55be13de",
		"gitlab.com/project/stefanprodan/podinfo-2985051089:6.3.3",
		"gitlab.com/project/ironbank/opensource/defenseunicorns/zarf/zarf-agent-2003217571:v0.25.0",
	}

	for idx, ref := range imageRefs {
		newRef, err := ImageTransformHost("gitlab.com/project", ref)
		assert.NoError(t, err)
		assert.Equal(t, expectedResult[idx], newRef)
	}

	for _, ref := range badImageRefs {
		_, err := ImageTransformHost("gitlab.com/project", ref)
		assert.Error(t, err)
	}
}

func TestImageTransformHostWithoutChecksum(t *testing.T) {
	var expectedResult = []string{
		"gitlab.com/project/library/nginx:1.23.3",
		"gitlab.com/project/defenseunicorns/zarf-agent:v0.22.1",
		"gitlab.com/project/defenseunicorns/zarf-agent@sha256:84605f731c6a18194794c51e70021c671ab064654b751aa57e905bce55be13de",
		"gitlab.com/project/stefanprodan/podinfo:6.3.3",
		"gitlab.com/project/ironbank/opensource/defenseunicorns/zarf/zarf-agent:v0.25.0",
	}

	for idx, ref := range imageRefs {
		newRef, err := ImageTransformHostWithoutChecksum("gitlab.com/project", ref)
		assert.NoError(t, err)
		assert.Equal(t, expectedResult[idx], newRef)
	}

	for _, ref := range badImageRefs {
		_, err := ImageTransformHostWithoutChecksum("gitlab.com/project", ref)
		assert.Error(t, err)
	}
}

func TestParseImageRef(t *testing.T) {
	var expectedResult = [][]string{
		{"docker.io/", "library/nginx", "1.23.3", ""},
		{"docker.io/", "defenseunicorns/zarf-agent", "v0.22.1", ""},
		{"docker.io/", "defenseunicorns/zarf-agent", "", "sha256:84605f731c6a18194794c51e70021c671ab064654b751aa57e905bce55be13de"},
		{"ghcr.io/", "stefanprodan/podinfo", "6.3.3", ""},
		{"registry1.dso.mil/", "ironbank/opensource/defenseunicorns/zarf/zarf-agent", "v0.25.0", ""},
	}

	for idx, ref := range imageRefs {
		img, err := ParseImageRef(ref)
		assert.NoError(t, err)
		tag := expectedResult[idx][2]
		digest := expectedResult[idx][3]
		tagOrDigest := ":" + tag
		if tag == "" {
			tagOrDigest = "@" + digest
		}
		path := expectedResult[idx][1]
		name := expectedResult[idx][0] + path
		reference := name + tagOrDigest

		assert.Equal(t, reference, img.Reference)
		assert.Equal(t, name, img.Name)
		assert.Equal(t, path, img.Path)
		assert.Equal(t, tag, img.Tag)
		assert.Equal(t, digest, img.Digest)
		assert.Equal(t, tagOrDigest, img.TagOrDigest)
	}

	for _, ref := range badImageRefs {
		_, err := ParseImageRef(ref)
		assert.Error(t, err)
	}
}
