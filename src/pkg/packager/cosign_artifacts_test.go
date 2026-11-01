// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"context"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	ociremote "github.com/sigstore/cosign/v3/pkg/oci/remote"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/types"
)

func TestCosignTagMatchesCosign(t *testing.T) {
	t.Parallel()
	digestRef, err := name.NewDigest("ghcr.io/zarf-dev/zarf/agent@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	require.NoError(t, err)

	sigTag, err := cosignTag(digestRef, "sig")
	require.NoError(t, err)
	expectedSig, err := ociremote.SignatureTag(digestRef)
	require.NoError(t, err)
	require.Equal(t, expectedSig.String(), sigTag.String())

	attTag, err := cosignTag(digestRef, "att")
	require.NoError(t, err)
	expectedAtt, err := ociremote.AttestationTag(digestRef)
	require.NoError(t, err)
	require.Equal(t, expectedAtt.String(), attTag.String())
}

func TestGetCosignArtifactsValidation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("nil auth client returns error", func(t *testing.T) {
		t.Parallel()
		artifacts, err := getCosignArtifacts(ctx, "ghcr.io/zarf-dev/zarf/agent:v0.1.0", nil, types.RemoteOptions{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "auth client is required")
		require.Nil(t, artifacts)
	})

	t.Run("invalid image reference returns error", func(t *testing.T) {
		t.Parallel()
		artifacts, err := getCosignArtifacts(ctx, "INVALID:::REFERENCE", nil, types.RemoteOptions{})
		require.Error(t, err)
		require.Nil(t, artifacts)
	})
}
