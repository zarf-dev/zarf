// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package transform provides helper functions to transform URLs to airgap equivalents
package transform

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var imageRefs = []string{
	"nginx",
	"nginx:1.23.3",
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
		"gitlab.com/project/library/nginx:latest-zarf-3793515731",
		"gitlab.com/project/library/nginx:1.23.3-zarf-3793515731",
		"gitlab.com/project/stefanprodan/podinfo:6.3.3-zarf-2985051089",
		"gitlab.com/project/ironbank/opensource/defenseunicorns/zarf/zarf-agent:v0.25.0-zarf-2003217571",
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
		"gitlab.com/project/library/nginx:latest",
		"gitlab.com/project/library/nginx:1.23.3",
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
