// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package signing

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/packager/signature"
)

func TestNewPackageSignerAndVerifier(t *testing.T) {
	t.Parallel()

	signer := NewPackageSigner(DefaultSignBlobOptions())
	require.NotNil(t, signer)

	verifier := NewPackageVerifier(DefaultVerifyBlobOptions())
	require.NotNil(t, verifier)
}

func TestPackageVerifier_Validation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("unsigned package without verification material returns ErrNoVerificationMaterial", func(t *testing.T) {
		t.Parallel()
		verifier := NewPackageVerifier(DefaultVerifyBlobOptions())
		files := signature.Files{
			Signed: false,
		}
		err := verifier.VerifyPackage(ctx, files)
		require.Error(t, err)
		require.ErrorIs(t, err, signature.ErrNoVerificationMaterial)
	})

	t.Run("unsigned package with verification material provided returns fatal error", func(t *testing.T) {
		t.Parallel()
		opts := DefaultVerifyBlobOptions()
		opts.Key = "./testdata/cosign.pub"
		verifier := NewPackageVerifier(opts)
		files := signature.Files{
			Signed: false,
		}
		err := verifier.VerifyPackage(ctx, files)
		require.Error(t, err)
		require.Contains(t, err.Error(), "verification material was provided but the package is not signed")
	})

	t.Run("signed package without verification material returns ErrNoVerificationMaterial", func(t *testing.T) {
		t.Parallel()
		verifier := NewPackageVerifier(DefaultVerifyBlobOptions())
		files := signature.Files{
			Signed: true,
		}
		err := verifier.VerifyPackage(ctx, files)
		require.Error(t, err)
		require.ErrorIs(t, err, signature.ErrNoVerificationMaterial)
	})

	t.Run("signed package with missing bundle and legacy signature returns not found", func(t *testing.T) {
		t.Parallel()
		opts := DefaultVerifyBlobOptions()
		opts.Key = "./testdata/cosign.pub"
		verifier := NewPackageVerifier(opts)
		files := signature.Files{
			Signed:          true,
			Bundle:          filepath.Join(t.TempDir(), "missing.bundle"),
			LegacySignature: filepath.Join(t.TempDir(), "missing.sig"),
		}
		err := verifier.VerifyPackage(ctx, files)
		require.Error(t, err)
		require.Contains(t, err.Error(), "signature not found")
	})

	t.Run("legacy signature with keyless identity returns error requiring bundle format", func(t *testing.T) {
		t.Parallel()
		tempDir := t.TempDir()
		legacySig := filepath.Join(tempDir, "zarf.yaml.sig")
		require.NoError(t, os.WriteFile(legacySig, []byte("fake-sig"), 0600))

		opts := DefaultVerifyBlobOptions()
		opts.CertVerify.CertIdentity = "user@example.com"
		verifier := NewPackageVerifier(opts)

		files := signature.Files{
			Signed:          true,
			Bundle:          filepath.Join(tempDir, "missing.bundle"),
			LegacySignature: legacySig,
		}
		err := verifier.VerifyPackage(ctx, files)
		require.Error(t, err)
		require.Contains(t, err.Error(), "keyless verification requires bundle-format signatures")
	})
}

func TestPackageSigner_And_Verifier_Roundtrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tempDir := t.TempDir()
	zarfYAMLPath := filepath.Join(tempDir, "zarf.yaml")
	require.NoError(t, os.WriteFile(zarfYAMLPath, []byte("kind: ZarfPackageConfig\nmetadata:\n  name: test\n"), 0600))

	bundlePath := filepath.Join(tempDir, "zarf.bundle.sig")

	signOpts := DefaultSignBlobOptions()
	signOpts.Key = "./testdata/cosign.key"
	signOpts.Password = "test"
	signer := NewPackageSigner(signOpts)

	req := signature.SignRequest{
		ZarfYAML:    zarfYAMLPath,
		Bundle:      bundlePath,
		Destination: bundlePath,
	}

	err := signer.SignPackage(ctx, req)
	require.NoError(t, err)
	require.FileExists(t, bundlePath)

	verifyOpts := DefaultVerifyBlobOptions()
	verifyOpts.Key = "./testdata/cosign.pub"
	verifier := NewPackageVerifier(verifyOpts)

	files := signature.Files{
		ZarfYAML: zarfYAMLPath,
		Bundle:   bundlePath,
		Signed:   true,
	}

	err = verifier.VerifyPackage(ctx, files)
	require.NoError(t, err)
}

