// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package transform provides helper functions to transform URLs to airgap equivalents
package transform

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var imageRefs = []string{
	"nginx",
	"nginx:1.23.3",
	"defenseunicorns/zarf-agent:v0.22.1",
	"defenseunicorns/zarf-agent@sha256:84605f731c6a18194794c51e70021c671ab064654b751aa57e905bce55be13de",
	"busybox:latest@sha256:3fbc632167424a6d997e74f52b878d7cc478225cffac6bc977eedfe51c7f4e79",
	"ghcr.io/stefanprodan/podinfo:6.3.3",
	"registry1.dso.mil/ironbank/opensource/defenseunicorns/zarf/zarf-agent:v0.25.0",
	"gitlab.com/project/gitea/gitea:1.19.3-rootless-zarf-3431384023",
	"oci://10.43.130.183:5000/stefanprodan/manifests/podinfo",
}

var badImageRefs = []string{
	"i am not a ref at all",
	"C:\\Users\\zarf",
	"http://urls.are/not/refs",
}

func TestImageTransformHost(t *testing.T) {
	var expectedResult = []string{
		// Normal git repos and references for pushing/pulling
		"gitlab.com/project/library/nginx:latest-zarf-3793515731",
		"gitlab.com/project/library/nginx:1.23.3-zarf-3793515731",
		"gitlab.com/project/defenseunicorns/zarf-agent:v0.22.1-zarf-4283503412",
		"gitlab.com/project/defenseunicorns/zarf-agent@sha256:84605f731c6a18194794c51e70021c671ab064654b751aa57e905bce55be13de",
		"gitlab.com/project/library/busybox@sha256:3fbc632167424a6d997e74f52b878d7cc478225cffac6bc977eedfe51c7f4e79",
		"gitlab.com/project/stefanprodan/podinfo:6.3.3-zarf-2985051089",
		"gitlab.com/project/ironbank/opensource/defenseunicorns/zarf/zarf-agent:v0.25.0-zarf-2003217571",
		"gitlab.com/project/gitea/gitea:1.19.3-rootless-zarf-3431384023",
		"gitlab.com/project/stefanprodan/manifests/podinfo:latest-zarf-531355090",
	}

	for idx, ref := range imageRefs {
		newRef, err := ImageTransformHost("gitlab.com/project", ref)
		require.NoError(t, err)
		require.Equal(t, expectedResult[idx], newRef)
	}

	for _, ref := range badImageRefs {
		_, err := ImageTransformHost("gitlab.com/project", ref)
		require.Error(t, err)
	}
}

func TestImageTransformHostWithoutChecksum(t *testing.T) {
	var expectedResult = []string{
		"gitlab.com/project/library/nginx:latest",
		"gitlab.com/project/library/nginx:1.23.3",
		"gitlab.com/project/defenseunicorns/zarf-agent:v0.22.1",
		"gitlab.com/project/defenseunicorns/zarf-agent@sha256:84605f731c6a18194794c51e70021c671ab064654b751aa57e905bce55be13de",
		"gitlab.com/project/library/busybox@sha256:3fbc632167424a6d997e74f52b878d7cc478225cffac6bc977eedfe51c7f4e79",
		"gitlab.com/project/stefanprodan/podinfo:6.3.3",
		"gitlab.com/project/ironbank/opensource/defenseunicorns/zarf/zarf-agent:v0.25.0",
		"gitlab.com/project/gitea/gitea:1.19.3-rootless-zarf-3431384023",
		"gitlab.com/project/stefanprodan/manifests/podinfo:latest",
	}

	for idx, ref := range imageRefs {
		newRef, err := ImageTransformHostWithoutChecksum("gitlab.com/project", ref)
		require.NoError(t, err)
		require.Equal(t, expectedResult[idx], newRef)
	}

	for _, ref := range badImageRefs {
		_, err := ImageTransformHostWithoutChecksum("gitlab.com/project", ref)
		require.Error(t, err)
	}
}

func TestParseImageRef(t *testing.T) {
	var expectedResult = [][]string{
		{"docker.io/", "library/nginx", "latest", ""},
		{"docker.io/", "library/nginx", "1.23.3", ""},
		{"docker.io/", "defenseunicorns/zarf-agent", "v0.22.1", ""},
		{"docker.io/", "defenseunicorns/zarf-agent", "", "sha256:84605f731c6a18194794c51e70021c671ab064654b751aa57e905bce55be13de"},
		{"docker.io/", "library/busybox", "latest", "sha256:3fbc632167424a6d997e74f52b878d7cc478225cffac6bc977eedfe51c7f4e79"},
		{"ghcr.io/", "stefanprodan/podinfo", "6.3.3", ""},
		{"registry1.dso.mil/", "ironbank/opensource/defenseunicorns/zarf/zarf-agent", "v0.25.0", ""},
		{"gitlab.com/", "project/gitea/gitea", "1.19.3-rootless-zarf-3431384023", ""},
		{"10.43.130.183:5000/", "stefanprodan/manifests/podinfo", "latest", ""},
	}

	for idx, ref := range imageRefs {
		img, err := ParseImageRef(ref)
		require.NoError(t, err)
		tag := expectedResult[idx][2]
		digest := expectedResult[idx][3]
		var tagOrDigest string
		var tagAndDigest string
		if tag != "" {
			tagOrDigest = ":" + tag
			tagAndDigest = ":" + tag
		}
		if digest != "" {
			tagOrDigest = "@" + digest
			tagAndDigest += "@" + digest
		}
		path := expectedResult[idx][1]
		name := expectedResult[idx][0] + path
		reference := name + tagAndDigest

		require.Equal(t, reference, img.Reference)
		require.Equal(t, name, img.Name)
		require.Equal(t, path, img.Path)
		require.Equal(t, tag, img.Tag)
		require.Equal(t, digest, img.Digest)
		require.Equal(t, tagOrDigest, img.TagOrDigest)
	}

	for _, ref := range badImageRefs {
		_, err := ParseImageRef(ref)
		require.Error(t, err)
	}
}
