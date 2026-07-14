// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package signing

import (
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
