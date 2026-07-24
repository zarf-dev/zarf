// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package zoci_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
	"oras.land/oras-go/v2/registry"
)

func TestReferenceAtDigest(t *testing.T) {
	t.Parallel()

	source, err := registry.ParseReference("registry.example/zarf-packages/my-package:latest")
	require.NoError(t, err)

	const digest = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	actual, err := zoci.ReferenceAtDigest(source, digest)
	require.NoError(t, err)
	require.Equal(t, source.Registry, actual.Registry)
	require.Equal(t, source.Repository, actual.Repository)
	require.Equal(t, digest, actual.Reference)
	require.Equal(t, "registry.example/zarf-packages/my-package@"+digest, actual.String())
}

func TestReferenceAtDigest_InvalidReference(t *testing.T) {
	t.Parallel()

	source, err := registry.ParseReference("registry.example/zarf-packages/my-package:latest")
	require.NoError(t, err)

	_, err = zoci.ReferenceAtDigest(source, "not-a-digest")
	require.ErrorContains(t, err, "invalid digest-pinned reference")
}
