// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package signature

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

type mockVerifier struct {
	verifyFn func(ctx context.Context, files Files) error
}

func (m *mockVerifier) VerifyPackage(ctx context.Context, files Files) error {
	if m.verifyFn != nil {
		return m.verifyFn(ctx, files)
	}
	return nil
}

type mockSigner struct {
	signFn func(ctx context.Context, req SignRequest) error
}

func (m *mockSigner) SignPackage(ctx context.Context, req SignRequest) error {
	if m.signFn != nil {
		return m.signFn(ctx, req)
	}
	return nil
}

func TestErrNoVerificationMaterial(t *testing.T) {
	t.Parallel()

	require.NotNil(t, ErrNoVerificationMaterial)
	wrapped := fmt.Errorf("wrap: %w", ErrNoVerificationMaterial)
	require.True(t, errors.Is(wrapped, ErrNoVerificationMaterial))
}

func TestVerifierInterface(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	files := Files{
		ZarfYAML:        "/path/to/zarf.yaml",
		Bundle:          "/path/to/zarf.bundle.sig",
		LegacySignature: "/path/to/zarf.yaml.sig",
		Signed:          true,
	}

	t.Run("successful verification", func(t *testing.T) {
		t.Parallel()
		var called bool
		v := &mockVerifier{
			verifyFn: func(_ context.Context, f Files) error {
				called = true
				require.Equal(t, files, f)
				return nil
			},
		}
		var verifier Verifier = v
		err := verifier.VerifyPackage(ctx, files)
		require.NoError(t, err)
		require.True(t, called)
	})

	t.Run("verification returns error", func(t *testing.T) {
		t.Parallel()
		v := &mockVerifier{
			verifyFn: func(_ context.Context, _ Files) error {
				return ErrNoVerificationMaterial
			},
		}
		var verifier Verifier = v
		err := verifier.VerifyPackage(ctx, files)
		require.ErrorIs(t, err, ErrNoVerificationMaterial)
	})
}

func TestSignerInterface(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	req := SignRequest{
		ZarfYAML:    "/tmp/zarf.yaml",
		Bundle:      "/tmp/zarf.bundle.sig",
		Destination: "/destination/zarf.bundle.sig",
	}

	t.Run("successful signing", func(t *testing.T) {
		t.Parallel()
		var called bool
		s := &mockSigner{
			signFn: func(_ context.Context, r SignRequest) error {
				called = true
				require.Equal(t, req, r)
				return nil
			},
		}
		var signer Signer = s
		err := signer.SignPackage(ctx, req)
		require.NoError(t, err)
		require.True(t, called)
	})

	t.Run("signing returns error", func(t *testing.T) {
		t.Parallel()
		expectedErr := errors.New("signing failed")
		s := &mockSigner{
			signFn: func(_ context.Context, _ SignRequest) error {
				return expectedErr
			},
		}
		var signer Signer = s
		err := signer.SignPackage(ctx, req)
		require.ErrorIs(t, err, expectedErr)
	})
}

