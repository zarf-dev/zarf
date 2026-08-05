// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package signing

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zarf-dev/zarf/src/test/testutil"
)

// TestDefaultSignBlobOptions_EmptyAuthFlow guards against re-introducing a
// non-empty AuthFlow default. cosign's GetOAuthFlow treats any non-empty
// AuthFlow as an explicit override, which bypasses ambient OIDC provider
// detection (GitHub Actions, GCP, SPIFFE, etc.) entirely. Keep this empty.
func TestDefaultSignBlobOptions_EmptyAuthFlow(t *testing.T) {
	t.Parallel()
	opts := DefaultSignBlobOptions()
	require.Empty(t, opts.Fulcio.AuthFlow)
}

func TestSigstoreVerifyBundleWithOptions(t *testing.T) {
	ctx := testutil.TestContext(t)

	const keyPath = "./testdata/cosign.key"
	const pubPath = "./testdata/cosign.pub"
	const password = "test"

	newBundle := func(t *testing.T) (string, string) {
		t.Helper()
		blobPath := filepath.Join(t.TempDir(), "payload.txt")
		bundlePath := filepath.Join(t.TempDir(), "sig.bundle")
		require.NoError(t, os.WriteFile(blobPath, []byte("direct verifier payload"), 0o644))

		signOpts := DefaultSignBlobOptions()
		signOpts.Key = keyPath
		signOpts.Password = password
		signOpts.BundlePath = bundlePath
		_, err := CosignSignBlobWithOptions(ctx, blobPath, signOpts)
		require.NoError(t, err)
		return blobPath, bundlePath
	}

	verify := func(t *testing.T, blobPath, bundlePath, key string) error {
		t.Helper()
		opts := DefaultVerifyBlobOptions()
		opts.Key = key
		opts.BundlePath = bundlePath
		return SigstoreVerifyBundleWithOptions(ctx, blobPath, opts)
	}

	t.Run("matches cosign for valid and tampered local-key bundles", func(t *testing.T) {
		blobPath, bundlePath := newBundle(t)
		require.NoError(t, verify(t, blobPath, bundlePath, pubPath))

		cosignOpts := DefaultVerifyBlobOptions()
		cosignOpts.Key = pubPath
		cosignOpts.BundlePath = bundlePath
		require.NoError(t, CosignVerifyBlobWithOptions(ctx, blobPath, cosignOpts))

		require.NoError(t, os.WriteFile(blobPath, []byte("tampered"), 0o644))
		require.Error(t, verify(t, blobPath, bundlePath, pubPath))
		require.Error(t, CosignVerifyBlobWithOptions(ctx, blobPath, cosignOpts))
	})

	t.Run("matches cosign for digest artifact references", func(t *testing.T) {
		blobPath, bundlePath := newBundle(t)
		payload, err := os.ReadFile(blobPath)
		require.NoError(t, err)
		digest := sha256.Sum256(payload)
		artifactRef := "sha256:" + hex.EncodeToString(digest[:])

		directOpts := DefaultVerifyBlobOptions()
		directOpts.Key = pubPath
		directOpts.BundlePath = bundlePath
		require.NoError(t, SigstoreVerifyBundleWithOptions(ctx, artifactRef, directOpts))

		cosignOpts := directOpts
		require.NoError(t, CosignVerifyBlobWithOptions(ctx, artifactRef, cosignOpts))
	})

	t.Run("accepts environment public-key references", func(t *testing.T) {
		blobPath, bundlePath := newBundle(t)
		publicKey, err := os.ReadFile(pubPath)
		require.NoError(t, err)
		t.Setenv("ZARF_TEST_COSIGN_PUBLIC_KEY", string(publicKey))
		require.NoError(t, verify(t, blobPath, bundlePath, "env://ZARF_TEST_COSIGN_PUBLIC_KEY"))
	})

	t.Run("accepts URL public-key references", func(t *testing.T) {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Skipf("loopback listener unavailable: %v", err)
		}
		blobPath, bundlePath := newBundle(t)
		publicKey, err := os.ReadFile(pubPath)
		require.NoError(t, err)
		server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if _, err := w.Write(publicKey); err != nil {
				t.Errorf("writing public-key response: %v", err)
			}
		}))
		server.Listener = listener
		server.Start()
		defer server.Close()
		require.NoError(t, verify(t, blobPath, bundlePath, server.URL))
	})

	t.Run("rejects wrong keys and corrupt bundles without cosign fallback", func(t *testing.T) {
		blobPath, bundlePath := newBundle(t)
		require.Error(t, verify(t, blobPath, bundlePath, "./testdata/nonexistent.pub"))
		require.NoError(t, os.WriteFile(bundlePath, []byte("not a bundle"), 0o644))
		require.Error(t, verify(t, blobPath, bundlePath, pubPath))
	})

	t.Run("rejects detached verification material for bundles", func(t *testing.T) {
		blobPath, bundlePath := newBundle(t)
		opts := DefaultVerifyBlobOptions()
		opts.Key = pubPath
		opts.BundlePath = bundlePath
		opts.Signature = "detached.sig"
		err := SigstoreVerifyBundleWithOptions(ctx, blobPath, opts)
		require.ErrorContains(t, err, "--signature")
	})

	t.Run("supports deprecated key alias", func(t *testing.T) {
		blobPath, bundlePath := newBundle(t)
		opts := DefaultVerifyBlobOptions()
		opts.KeyRef = pubPath
		opts.BundlePath = bundlePath
		require.NoError(t, SigstoreVerifyBundleWithOptions(ctx, blobPath, opts))
	})

	t.Run("uses embedded trusted root for keyless verification", func(t *testing.T) {
		opts := DefaultVerifyBlobOptions()
		material, err := trustedMaterialForBundle(opts, nil, false)
		require.NoError(t, err)
		require.NotEmpty(t, material.FulcioCertificateAuthorities())
	})
}

func TestShouldSign_KeyRefAlias(t *testing.T) {
	t.Parallel()

	t.Run("KeyRef alone triggers signing", func(t *testing.T) {
		opts := SignBlobOptions{}
		opts.KeyRef = "/path/to/key"
		require.True(t, opts.ShouldSign())
	})

	t.Run("Key alone triggers signing", func(t *testing.T) {
		opts := SignBlobOptions{}
		opts.Key = "/path/to/key"
		require.True(t, opts.ShouldSign())
	})

	t.Run("Keyless alone triggers signing", func(t *testing.T) {
		opts := SignBlobOptions{}
		opts.Keyless = true
		require.True(t, opts.ShouldSign())
	})

	t.Run("empty options skip signing", func(t *testing.T) {
		require.False(t, SignBlobOptions{}.ShouldSign())
	})
}

// TestCosignSignVerifyRoundTrip exercises CosignSignBlobWithOptions and
// CosignVerifyBlobWithOptions for both the bundle format (cosign v3.1.1+ default)
// and the legacy .sig format.
func TestCosignSignVerifyRoundTrip(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)

	const keyPath = "./testdata/cosign.key"
	const pubPath = "./testdata/cosign.pub"
	const password = "test"

	writeBlob := func(t *testing.T, content string) string {
		t.Helper()
		p := filepath.Join(t.TempDir(), "payload.txt")
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
		return p
	}

	t.Run("bundle format: sign then verify succeeds", func(t *testing.T) {
		t.Parallel()
		blobPath := writeBlob(t, "bundle round-trip payload")
		bundlePath := filepath.Join(t.TempDir(), "sig.bundle")

		signOpts := DefaultSignBlobOptions()
		signOpts.Key = keyPath
		signOpts.Password = password
		signOpts.BundlePath = bundlePath

		_, err := CosignSignBlobWithOptions(ctx, blobPath, signOpts)
		require.NoError(t, err)
		require.FileExists(t, bundlePath)

		verifyOpts := DefaultVerifyBlobOptions()
		verifyOpts.Key = pubPath
		verifyOpts.BundlePath = bundlePath

		require.NoError(t, CosignVerifyBlobWithOptions(ctx, blobPath, verifyOpts))
	})

	t.Run("legacy format: sign then verify succeeds", func(t *testing.T) {
		t.Parallel()
		blobPath := writeBlob(t, "legacy round-trip payload")
		sigPath := filepath.Join(t.TempDir(), "sig.sig")

		signOpts := DefaultSignBlobOptions()
		signOpts.Key = keyPath
		signOpts.Password = password
		signOpts.OutputSignature = sigPath
		signOpts.NewBundleFormat = false

		_, err := CosignSignBlobWithOptions(ctx, blobPath, signOpts)
		require.NoError(t, err)
		require.FileExists(t, sigPath)

		verifyOpts := DefaultVerifyBlobOptions()
		verifyOpts.Key = pubPath
		verifyOpts.Signature = sigPath
		verifyOpts.CommonVerifyOptions.NewBundleFormat = false

		require.NoError(t, CosignVerifyBlobWithOptions(ctx, blobPath, verifyOpts))
	})

	t.Run("bundle format: tampered content fails verification", func(t *testing.T) {
		t.Parallel()
		blobPath := writeBlob(t, "original content")
		bundlePath := filepath.Join(t.TempDir(), "sig.bundle")

		signOpts := DefaultSignBlobOptions()
		signOpts.Key = keyPath
		signOpts.Password = password
		signOpts.BundlePath = bundlePath

		_, err := CosignSignBlobWithOptions(ctx, blobPath, signOpts)
		require.NoError(t, err)

		require.NoError(t, os.WriteFile(blobPath, []byte("tampered content"), 0o644))

		verifyOpts := DefaultVerifyBlobOptions()
		verifyOpts.Key = pubPath
		verifyOpts.BundlePath = bundlePath

		require.Error(t, CosignVerifyBlobWithOptions(ctx, blobPath, verifyOpts))
	})

	t.Run("legacy format: tampered content fails verification", func(t *testing.T) {
		t.Parallel()
		blobPath := writeBlob(t, "original content")
		sigPath := filepath.Join(t.TempDir(), "sig.sig")

		signOpts := DefaultSignBlobOptions()
		signOpts.Key = keyPath
		signOpts.Password = password
		signOpts.OutputSignature = sigPath
		signOpts.NewBundleFormat = false

		_, err := CosignSignBlobWithOptions(ctx, blobPath, signOpts)
		require.NoError(t, err)

		require.NoError(t, os.WriteFile(blobPath, []byte("tampered content"), 0o644))

		verifyOpts := DefaultVerifyBlobOptions()
		verifyOpts.Key = pubPath
		verifyOpts.Signature = sigPath
		verifyOpts.CommonVerifyOptions.NewBundleFormat = false

		require.Error(t, CosignVerifyBlobWithOptions(ctx, blobPath, verifyOpts))
	})

	t.Run("bundle format: wrong key fails verification", func(t *testing.T) {
		t.Parallel()
		blobPath := writeBlob(t, "some content")
		bundlePath := filepath.Join(t.TempDir(), "sig.bundle")

		signOpts := DefaultSignBlobOptions()
		signOpts.Key = keyPath
		signOpts.Password = password
		signOpts.BundlePath = bundlePath

		_, err := CosignSignBlobWithOptions(ctx, blobPath, signOpts)
		require.NoError(t, err)

		verifyOpts := DefaultVerifyBlobOptions()
		verifyOpts.Key = "./testdata/nonexistent.pub"
		verifyOpts.BundlePath = bundlePath

		require.Error(t, CosignVerifyBlobWithOptions(ctx, blobPath, verifyOpts))
	})

	t.Run("legacy format: wrong key fails verification", func(t *testing.T) {
		t.Parallel()
		blobPath := writeBlob(t, "some content")
		sigPath := filepath.Join(t.TempDir(), "sig.sig")

		signOpts := DefaultSignBlobOptions()
		signOpts.Key = keyPath
		signOpts.Password = password
		signOpts.OutputSignature = sigPath
		signOpts.NewBundleFormat = false

		_, err := CosignSignBlobWithOptions(ctx, blobPath, signOpts)
		require.NoError(t, err)

		verifyOpts := DefaultVerifyBlobOptions()
		verifyOpts.Key = "./testdata/nonexistent.pub"
		verifyOpts.Signature = sigPath
		verifyOpts.CommonVerifyOptions.NewBundleFormat = false

		require.Error(t, CosignVerifyBlobWithOptions(ctx, blobPath, verifyOpts))
	})

	t.Run("wrong password fails signing", func(t *testing.T) {
		t.Parallel()
		blobPath := writeBlob(t, "some content")

		signOpts := DefaultSignBlobOptions()
		signOpts.Key = keyPath
		signOpts.Password = "wrongpassword"
		signOpts.BundlePath = filepath.Join(t.TempDir(), "sig.bundle")

		_, err := CosignSignBlobWithOptions(ctx, blobPath, signOpts)
		require.Error(t, err)
	})
}
