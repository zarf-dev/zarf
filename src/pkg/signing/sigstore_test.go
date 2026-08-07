// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package signing

import (
	"context"
	"crypto"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/zarf-dev/zarf/src/test/testutil"
	corev1 "k8s.io/api/core/v1"
)

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
		_, err := SigstoreVerifyBundleWithOptions(ctx, blobPath, opts)
		return err
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
		_, err = SigstoreVerifyBundleWithOptions(ctx, artifactRef, directOpts)
		require.NoError(t, err)

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

	t.Run("accepts Kubernetes public-key references", func(t *testing.T) {
		blobPath, bundlePath := newBundle(t)
		publicKey, err := os.ReadFile(pubPath)
		require.NoError(t, err)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if request.URL.Path != "/api/v1/namespaces/signature-test/secrets/signing-public-key" {
				http.NotFound(w, request)
				return
			}
			if err := json.NewEncoder(w).Encode(corev1.Secret{
				Data: map[string][]byte{"cosign.pub": publicKey},
			}); err != nil {
				t.Errorf("writing Kubernetes Secret response: %v", err)
			}
		}))
		t.Cleanup(server.Close)

		kubeconfigPath := filepath.Join(t.TempDir(), "kubeconfig")
		kubeconfig := fmt.Sprintf(`apiVersion: v1
clusters:
- cluster:
    server: %s
  name: signing-test
contexts:
- context:
    cluster: signing-test
    namespace: signature-test
  name: signing-test
current-context: signing-test
`, server.URL)
		require.NoError(t, os.WriteFile(kubeconfigPath, []byte(kubeconfig), 0o600))
		t.Setenv("KUBECONFIG", kubeconfigPath)

		require.NoError(t, verify(t, blobPath, bundlePath, "k8s://signature-test/signing-public-key"))
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

	t.Run("URL public-key retrieval honors verification timeout", func(t *testing.T) {
		blobPath, bundlePath := newBundle(t)
		requestStarted := make(chan struct{})
		server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
			close(requestStarted)
			<-request.Context().Done()
		}))
		t.Cleanup(server.Close)

		opts := DefaultVerifyBlobOptions()
		opts.Key = server.URL
		opts.BundlePath = bundlePath
		opts.Timeout = 100 * time.Millisecond
		_, err := SigstoreVerifyBundleWithOptions(ctx, blobPath, opts)

		require.ErrorIs(t, err, context.DeadlineExceeded)
		select {
		case <-requestStarted:
		default:
			t.Fatal("expected public-key request")
		}
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
		_, err := SigstoreVerifyBundleWithOptions(ctx, blobPath, opts)
		require.ErrorContains(t, err, "detached signature")
		require.NotContains(t, err.Error(), "--")
	})

	t.Run("supports deprecated key alias", func(t *testing.T) {
		blobPath, bundlePath := newBundle(t)
		opts := DefaultVerifyBlobOptions()
		opts.KeyRef = pubPath
		opts.BundlePath = bundlePath
		_, err := SigstoreVerifyBundleWithOptions(ctx, blobPath, opts)
		require.NoError(t, err)
	})

	t.Run("uses embedded trusted root for keyless verification", func(t *testing.T) {
		opts := DefaultVerifyBlobOptions()
		material, err := trustedMaterialForBundle(opts, nil, false)
		require.NoError(t, err)
		require.NotEmpty(t, material.FulcioCertificateAuthorities())
	})

	t.Run("uses embedded trusted root for keyed tlog verification", func(t *testing.T) {
		publicKey, err := os.ReadFile(pubPath)
		require.NoError(t, err)
		keyVerifier, closeVerifier, err := verifierFromPEM(publicKey, crypto.SHA256)
		require.NoError(t, err)
		defer closeVerifier()

		opts := DefaultVerifyBlobOptions()
		opts.CommonVerifyOptions.IgnoreTlog = false
		material, err := trustedMaterialForBundle(opts, keyVerifier, false)
		require.NoError(t, err)
		require.NotEmpty(t, material.RekorLogs())
		require.NotEmpty(t, material.TimestampingAuthorities())
	})

	t.Run("verifies keyless public-good bundle", func(t *testing.T) {
		opts := DefaultVerifyBlobOptions()
		opts.BundlePath = "./testdata/sigstore-js-2.0.0-provenance.sigstore.json"
		opts.CertVerify.CertIdentityRegexp = "^https://github.com/sigstore/sigstore-js/"
		opts.CertVerify.CertOidcIssuer = "https://token.actions.githubusercontent.com"
		opts.CommonVerifyOptions.IgnoreTlog = false

		const digestReference = "sha512:46d4e2f74c4877316640000a6fdf8a8b59f1e0847667973e9859f774dd31b8f1e0937813b777fb66a2ac67d50540fe34640966eee9fc2ccca387082b4c85cd3c"
		result, err := SigstoreVerifyBundleWithOptions(ctx, digestReference, opts)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.NotNil(t, result.VerifiedIdentity)

		invalidOpts := opts
		invalidOpts.CertVerify.CertIdentityRegexp = "^https://github.com/sigstore/other-project/"
		_, err = SigstoreVerifyBundleWithOptions(ctx, digestReference, invalidOpts)
		require.Error(t, err)
	})
}

func TestSigstoreBundleValidationErrorsUseLibraryTerms(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*VerifyBlobOptions)
		want      string
	}{
		{
			name: "requires bundle path",
			want: "bundle path is required",
		},
		{
			name: "rejects key with certificate identity",
			configure: func(opts *VerifyBlobOptions) {
				opts.BundlePath = "bundle.json"
				opts.Key = "key.pem"
				opts.CertVerify.CertIdentity = "https://example.test"
			},
			want: "key cannot be combined with certificate identity verification",
		},
		{
			name: "rejects key with security key",
			configure: func(opts *VerifyBlobOptions) {
				opts.BundlePath = "bundle.json"
				opts.Key = "key.pem"
				opts.SecurityKey.Use = true
			},
			want: "key cannot be combined with security-key verification",
		},
		{
			name: "rejects detached signature",
			configure: func(opts *VerifyBlobOptions) {
				opts.Signature = "signature"
			},
			want: "unsupported verification material for Sigstore bundles: detached signature",
		},
		{
			name: "rejects certificate",
			configure: func(opts *VerifyBlobOptions) {
				opts.CertVerify.Cert = "certificate"
			},
			want: "unsupported verification material for Sigstore bundles: certificate",
		},
		{
			name: "rejects certificate chain",
			configure: func(opts *VerifyBlobOptions) {
				opts.CertVerify.CertChain = "chain"
			},
			want: "unsupported verification material for Sigstore bundles: certificate chain",
		},
		{
			name: "rejects certificate authority roots",
			configure: func(opts *VerifyBlobOptions) {
				opts.CertVerify.CARoots = "roots"
			},
			want: "unsupported verification material for Sigstore bundles: certificate authority roots",
		},
		{
			name: "rejects certificate authority intermediates",
			configure: func(opts *VerifyBlobOptions) {
				opts.CertVerify.CAIntermediates = "intermediates"
			},
			want: "unsupported verification material for Sigstore bundles: certificate authority intermediates",
		},
		{
			name: "rejects timestamp certificate chain",
			configure: func(opts *VerifyBlobOptions) {
				opts.CommonVerifyOptions.TSACertChainPath = "timestamp-chain"
			},
			want: "unsupported verification material for Sigstore bundles: timestamp certificate chain",
		},
		{
			name: "rejects signed certificate timestamp",
			configure: func(opts *VerifyBlobOptions) {
				opts.CertVerify.SCT = "sct"
			},
			want: "unsupported verification material for Sigstore bundles: signed certificate timestamp",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts := DefaultVerifyBlobOptions()
			if tc.configure != nil {
				tc.configure(&opts)
			}

			var err error
			if opts.BundlePath == "" {
				_, err = SigstoreVerifyBundleWithOptions(testutil.TestContext(t), "", opts)
			} else {
				err = validateSigstoreBundleOptions(opts)
			}
			require.ErrorContains(t, err, tc.want)
			require.NotContains(t, err.Error(), "--")
		})
	}
}
