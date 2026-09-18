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

func TestVerifyBundle(t *testing.T) {
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

	newOptions := func(t *testing.T, key string) BundleVerificationOptions {
		t.Helper()
		opts := DefaultVerifyBlobOptions()
		opts.Key = key
		bundleOpts, err := NewBundleVerificationOptions(ctx, opts)
		require.NoError(t, err)
		return bundleOpts
	}

	verifyBundle := func(t *testing.T, blobPath, bundlePath, key string) error {
		t.Helper()
		_, err := VerifyBundle(ctx, blobPath, bundlePath, newOptions(t, key))
		return err
	}

	t.Run("matches cosign for valid and tampered local-key bundles", func(t *testing.T) {
		blobPath, bundlePath := newBundle(t)
		require.NoError(t, verifyBundle(t, blobPath, bundlePath, pubPath))

		cosignOpts := DefaultVerifyBlobOptions()
		cosignOpts.Key = pubPath
		cosignOpts.BundlePath = bundlePath
		require.NoError(t, CosignVerifyBlobWithOptions(ctx, blobPath, cosignOpts))

		require.NoError(t, os.WriteFile(blobPath, []byte("tampered"), 0o644))
		require.Error(t, verifyBundle(t, blobPath, bundlePath, pubPath))
		require.Error(t, CosignVerifyBlobWithOptions(ctx, blobPath, cosignOpts))
	})

	t.Run("matches cosign for digest artifact references", func(t *testing.T) {
		blobPath, bundlePath := newBundle(t)
		payload, err := os.ReadFile(blobPath)
		require.NoError(t, err)
		digest := sha256.Sum256(payload)
		artifactRef := "sha256:" + hex.EncodeToString(digest[:])

		directOpts := newOptions(t, pubPath)
		_, err = VerifyBundle(ctx, artifactRef, bundlePath, directOpts)
		require.NoError(t, err)

		cosignOpts := DefaultVerifyBlobOptions()
		cosignOpts.Key = pubPath
		cosignOpts.BundlePath = bundlePath
		require.NoError(t, CosignVerifyBlobWithOptions(ctx, artifactRef, cosignOpts))
	})

	t.Run("accepts environment public-key references", func(t *testing.T) {
		blobPath, bundlePath := newBundle(t)
		publicKey, err := os.ReadFile(pubPath)
		require.NoError(t, err)
		t.Setenv("ZARF_TEST_COSIGN_PUBLIC_KEY", string(publicKey))
		require.NoError(t, verifyBundle(t, blobPath, bundlePath, "env://ZARF_TEST_COSIGN_PUBLIC_KEY"))
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

		require.NoError(t, verifyBundle(t, blobPath, bundlePath, "k8s://signature-test/signing-public-key"))
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
		require.NoError(t, verifyBundle(t, blobPath, bundlePath, server.URL))
	})

	t.Run("URL public-key retrieval honors verification timeout", func(t *testing.T) {
		blobPath, bundlePath := newBundle(t)
		requestStarted := make(chan struct{})
		server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
			close(requestStarted)
			<-request.Context().Done()
		}))
		t.Cleanup(server.Close)

		opts := newOptions(t, server.URL)
		opts.Timeout = 100 * time.Millisecond
		_, err := VerifyBundle(ctx, blobPath, bundlePath, opts)

		require.ErrorIs(t, err, context.DeadlineExceeded)
		select {
		case <-requestStarted:
		default:
			t.Fatal("expected public-key request")
		}
	})

	t.Run("rejects wrong keys and corrupt bundles without cosign fallback", func(t *testing.T) {
		blobPath, bundlePath := newBundle(t)
		require.Error(t, verifyBundle(t, blobPath, bundlePath, "./testdata/nonexistent.pub"))
		require.NoError(t, os.WriteFile(bundlePath, []byte("not a bundle"), 0o644))
		require.Error(t, verifyBundle(t, blobPath, bundlePath, pubPath))
	})

	t.Run("requires a bundle path", func(t *testing.T) {
		_, err := VerifyBundle(ctx, "payload", "", newOptions(t, pubPath))
		require.EqualError(t, err, "bundle path is required")
	})

	t.Run("uses embedded trusted root for keyless verification", func(t *testing.T) {
		material, err := trustedMaterialForBundle(BundleVerificationOptions{}, nil, false)
		require.NoError(t, err)
		require.NotEmpty(t, material.FulcioCertificateAuthorities())
	})

	t.Run("uses embedded trusted root for keyed tlog verification", func(t *testing.T) {
		publicKey, err := os.ReadFile(pubPath)
		require.NoError(t, err)
		keyVerifier, closeVerifier, err := verifierFromPEM(publicKey, crypto.SHA256)
		require.NoError(t, err)
		defer closeVerifier()

		material, err := trustedMaterialForBundle(BundleVerificationOptions{IgnoreTlog: false}, keyVerifier, false)
		require.NoError(t, err)
		require.NotEmpty(t, material.RekorLogs())
		require.NotEmpty(t, material.TimestampingAuthorities())
	})

	t.Run("verifies keyless public-good bundle", func(t *testing.T) {
		opts := BundleVerificationOptions{
			CertificateIdentityRegexp: "^https://github.com/sigstore/sigstore-js/",
			CertificateOIDCIssuer:     "https://token.actions.githubusercontent.com",
			IgnoreTlog:                false,
		}

		const digestReference = "sha512:46d4e2f74c4877316640000a6fdf8a8b59f1e0847667973e9859f774dd31b8f1e0937813b777fb66a2ac67d50540fe34640966eee9fc2ccca387082b4c85cd3c"
		result, err := VerifyBundle(ctx, digestReference, "./testdata/sigstore-js-2.0.0-provenance.sigstore.json", opts)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.NotNil(t, result.VerifiedIdentity)

		invalidOpts := opts
		invalidOpts.CertificateIdentityRegexp = "^https://github.com/sigstore/other-project/"
		_, err = VerifyBundle(ctx, digestReference, "./testdata/sigstore-js-2.0.0-provenance.sigstore.json", invalidOpts)
		require.Error(t, err)
	})
}

func TestNewBundleVerificationOptions(t *testing.T) {
	ctx := testutil.TestContext(t)

	t.Run("maps package CLI controls", func(t *testing.T) {
		opts := DefaultVerifyBlobOptions()
		opts.CertVerify.CertIdentity = "https://example.test"
		opts.CertVerify.CertIdentityRegexp = "^https://example.test/"
		opts.CertVerify.CertOidcIssuer = "https://issuer.example.test"
		opts.CertVerify.CertOidcIssuerRegexp = "^https://issuer.example.test/"
		opts.CommonVerifyOptions.TrustedRootPath = "trusted-root.json"
		opts.CommonVerifyOptions.IgnoreTlog = false
		opts.CommonVerifyOptions.UseSignedTimestamps = true
		opts.Timeout = time.Second

		keylessOpts, err := NewBundleVerificationOptions(ctx, opts)

		require.NoError(t, err)
		require.Equal(t, BundleVerificationOptions{
			CertificateIdentity:         "https://example.test",
			CertificateIdentityRegexp:   "^https://example.test/",
			CertificateOIDCIssuer:       "https://issuer.example.test",
			CertificateOIDCIssuerRegexp: "^https://issuer.example.test/",
			TrustedRootPath:             "trusted-root.json",
			UseSignedTimestamps:         true,
			Timeout:                     time.Second,
		}, keylessOpts)

		opts = DefaultVerifyBlobOptions()
		opts.KeyRef = "key.pem"
		keyOpts, err := NewBundleVerificationOptions(ctx, opts)
		require.NoError(t, err)
		require.Equal(t, "key.pem", keyOpts.Key)

		opts = DefaultVerifyBlobOptions()
		opts.SignatureDigest.AlgorithmName = "sha256"
		_, err = NewBundleVerificationOptions(ctx, opts)
		require.NoError(t, err)
	})

	tests := []struct {
		name      string
		configure func(*VerifyBlobOptions)
		want      string
	}{
		{"detached signature", func(opts *VerifyBlobOptions) { opts.Signature = "signature" }, "--signature"},
		{"deprecated detached signature", func(opts *VerifyBlobOptions) { opts.SigRef = "signature" }, "--signature"},
		{"deprecated bundle path", func(opts *VerifyBlobOptions) { opts.BundlePath = "bundle.json" }, "--bundle"},
		{"security key", func(opts *VerifyBlobOptions) { opts.SecurityKey.Use = true }, "--sk"},
		{"certificate", func(opts *VerifyBlobOptions) { opts.CertVerify.Cert = "certificate" }, "--certificate"},
		{"certificate chain", func(opts *VerifyBlobOptions) { opts.CertVerify.CertChain = "chain" }, "--certificate-chain"},
		{"certificate authority roots", func(opts *VerifyBlobOptions) { opts.CertVerify.CARoots = "roots" }, "--ca-roots"},
		{"certificate authority intermediates", func(opts *VerifyBlobOptions) { opts.CertVerify.CAIntermediates = "intermediates" }, "--ca-intermediates"},
		{"timestamp certificate chain", func(opts *VerifyBlobOptions) { opts.CommonVerifyOptions.TSACertChainPath = "timestamp-chain" }, "--timestamp-certificate-chain"},
		{"signed certificate timestamp", func(opts *VerifyBlobOptions) { opts.CertVerify.SCT = "sct" }, "--sct"},
		{"SCT verification", func(opts *VerifyBlobOptions) { opts.CertVerify.IgnoreSCT = false }, "--insecure-ignore-sct"},
		{"GitHub workflow trigger", func(opts *VerifyBlobOptions) { opts.CertVerify.CertGithubWorkflowTrigger = "push" }, "--certificate-github-workflow-trigger"},
		{"GitHub workflow SHA", func(opts *VerifyBlobOptions) { opts.CertVerify.CertGithubWorkflowSha = "sha" }, "--certificate-github-workflow-sha"},
		{"GitHub workflow name", func(opts *VerifyBlobOptions) { opts.CertVerify.CertGithubWorkflowName = "workflow" }, "--certificate-github-workflow-name"},
		{"GitHub workflow repository", func(opts *VerifyBlobOptions) { opts.CertVerify.CertGithubWorkflowRepository = "owner/repo" }, "--certificate-github-workflow-repository"},
		{"GitHub workflow ref", func(opts *VerifyBlobOptions) { opts.CertVerify.CertGithubWorkflowRef = "refs/heads/main" }, "--certificate-github-workflow-ref"},
		{"Rekor URL", func(opts *VerifyBlobOptions) { opts.Rekor.URL = "https://rekor.example.test" }, "--rekor-url"},
		{"offline verification", func(opts *VerifyBlobOptions) { opts.CommonVerifyOptions.Offline = true }, "--offline"},
		{"max workers", func(opts *VerifyBlobOptions) { opts.CommonVerifyOptions.MaxWorkers = 1 }, "--max-workers"},
		{"experimental OCI", func(opts *VerifyBlobOptions) { opts.CommonVerifyOptions.ExperimentalOCI11 = true }, "--experimental-oci11"},
		{"private infrastructure", func(opts *VerifyBlobOptions) { opts.CommonVerifyOptions.PrivateInfrastructure = true }, "--private-infrastructure"},
		{"certificate chain allowance", func(opts *VerifyBlobOptions) { opts.CommonVerifyOptions.AllowCertificateChain = true }, "--allow-certificate-chain"},
		{"signature digest algorithm", func(opts *VerifyBlobOptions) { opts.SignatureDigest.AlgorithmName = "sha512" }, "--signature-digest-algorithm"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts := DefaultVerifyBlobOptions()
			tc.configure(&opts)

			_, err := NewBundleVerificationOptions(ctx, opts)
			require.EqualError(t, err, "unsupported package bundle verification option: "+tc.want)
		})
	}
}
