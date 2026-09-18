// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package signing

import (
	"bytes"
	"context"
	"crypto"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	cosigngit "github.com/sigstore/cosign/v3/pkg/cosign/git"
	"github.com/sigstore/cosign/v3/pkg/cosign/kubernetes"
	"github.com/sigstore/cosign/v3/pkg/cosign/pkcs11key"
	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/fulcio/certificate"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/verify"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/sigstore/sigstore/pkg/signature/kms"

	"github.com/zarf-dev/zarf/src/pkg/logger"
)

// BundleVerificationOptions are the package CLI controls supported for direct
// Sigstore bundle verification.
type BundleVerificationOptions struct {
	Key                         string
	CertificateIdentity         string
	CertificateIdentityRegexp   string
	CertificateOIDCIssuer       string
	CertificateOIDCIssuerRegexp string
	TrustedRootPath             string
	IgnoreTlog                  bool
	UseSignedTimestamps         bool
	Timeout                     time.Duration
}

// NewBundleVerificationOptions exposes the options available for package verification
func NewBundleVerificationOptions(ctx context.Context, opts VerifyBlobOptions) (BundleVerificationOptions, error) {
	if opts.KeyRef != "" {
		logger.From(ctx).Warn("VerifyBlobOptions.KeyRef is deprecated, use Key (removed in v1.0)")
		if opts.Key == "" {
			opts.Key = opts.KeyRef
		}
	}

	if err := validateSigstoreBundleOptions(opts); err != nil {
		return BundleVerificationOptions{}, err
	}

	return BundleVerificationOptions{
		Key:                         opts.Key,
		CertificateIdentity:         opts.CertVerify.CertIdentity,
		CertificateIdentityRegexp:   opts.CertVerify.CertIdentityRegexp,
		CertificateOIDCIssuer:       opts.CertVerify.CertOidcIssuer,
		CertificateOIDCIssuerRegexp: opts.CertVerify.CertOidcIssuerRegexp,
		TrustedRootPath:             opts.CommonVerifyOptions.TrustedRootPath,
		IgnoreTlog:                  opts.CommonVerifyOptions.IgnoreTlog,
		UseSignedTimestamps:         opts.CommonVerifyOptions.UseSignedTimestamps,
		Timeout:                     opts.Timeout,
	}, nil
}

// VerifyBundle verifies a Sigstore bundle directly with sigstore-go
// and returns the verified bundle contents.
func VerifyBundle(ctx context.Context, blobPath, bundlePath string, opts BundleVerificationOptions) (*verify.VerificationResult, error) {
	l := logger.From(ctx)
	if bundlePath == "" {
		return nil, errors.New("bundle path is required")
	}
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	b, err := bundle.LoadJSONFromPath(bundlePath)
	if err != nil {
		return nil, fmt.Errorf("loading Sigstore bundle: %w", err)
	}

	keyVerifier, closeKey, err := resolveBundleVerifier(ctx, opts, crypto.SHA256)
	if err != nil {
		return nil, fmt.Errorf("loading verifier from key options: %w", err)
	}
	defer closeKey()

	useSignedTimestamps := opts.UseSignedTimestamps
	if !opts.IgnoreTlog && keyVerifier == nil {
		v1, v2, err := rekorBundleVersions(b)
		if err != nil {
			return nil, err
		}
		// Rekor v2 does not provide an integrated timestamp. This mirrors
		// cosign's new-bundle verifier, which enables TSA validation for a
		// v2-only bundle automatically.
		if v2 && !v1 {
			useSignedTimestamps = true
		}
	}

	trustedMaterial, err := trustedMaterialForBundle(opts, keyVerifier, useSignedTimestamps)
	if err != nil {
		return nil, err
	}
	verifierOptions, policyOptions, err := sigstoreVerificationOptions(opts, keyVerifier != nil, useSignedTimestamps)
	if err != nil {
		return nil, err
	}
	sev, err := verify.NewVerifier(trustedMaterial, verifierOptions...)
	if err != nil {
		return nil, fmt.Errorf("creating Sigstore verifier: %w", err)
	}

	artifactPolicy, err := bundleArtifactPolicy(ctx, blobPath)
	if err != nil {
		return nil, err
	}
	result, err := sev.Verify(b, verify.NewPolicy(artifactPolicy, policyOptions...))
	if err != nil {
		return nil, err
	}
	l.Debug("blob signature verified successfully")
	return result, nil
}

func validateSigstoreBundleOptions(opts VerifyBlobOptions) error {
	if opts.Key != "" && (opts.CertVerify.CertIdentity != "" || opts.CertVerify.CertIdentityRegexp != "") {
		return errors.New("key cannot be combined with certificate identity verification")
	}

	unsupported := []struct {
		set  bool
		name string
	}{
		{opts.Signature != "" || opts.SigRef != "", "--signature"},
		{opts.BundlePath != "", "--bundle"},
		{opts.SecurityKey.Use, "--sk"},
		{opts.CertVerify.Cert != "", "--certificate"},
		{opts.CertVerify.CertChain != "", "--certificate-chain"},
		{opts.CertVerify.CARoots != "", "--ca-roots"},
		{opts.CertVerify.CAIntermediates != "", "--ca-intermediates"},
		{opts.CertVerify.SCT != "", "--sct"},
		{!opts.CertVerify.IgnoreSCT, "--insecure-ignore-sct"},
		{opts.CertVerify.CertGithubWorkflowTrigger != "", "--certificate-github-workflow-trigger"},
		{opts.CertVerify.CertGithubWorkflowSha != "", "--certificate-github-workflow-sha"},
		{opts.CertVerify.CertGithubWorkflowName != "", "--certificate-github-workflow-name"},
		{opts.CertVerify.CertGithubWorkflowRepository != "", "--certificate-github-workflow-repository"},
		{opts.CertVerify.CertGithubWorkflowRef != "", "--certificate-github-workflow-ref"},
		{opts.Rekor.URL != "", "--rekor-url"},
		{opts.CommonVerifyOptions.Offline, "--offline"},
		{opts.CommonVerifyOptions.TSACertChainPath != "", "--timestamp-certificate-chain"},
		{opts.CommonVerifyOptions.MaxWorkers != 0, "--max-workers"},
		{opts.CommonVerifyOptions.ExperimentalOCI11, "--experimental-oci11"},
		{opts.CommonVerifyOptions.PrivateInfrastructure, "--private-infrastructure"},
		{opts.CommonVerifyOptions.AllowCertificateChain, "--allow-certificate-chain"},
		{opts.SignatureDigest.AlgorithmName != "" && !strings.EqualFold(strings.TrimSpace(opts.SignatureDigest.AlgorithmName), "sha256"), "--signature-digest-algorithm"},
		{opts.TempDir != "", "temporary directory"},
	}
	for _, option := range unsupported {
		if option.set {
			return fmt.Errorf("unsupported package bundle verification option: %s", option.name)
		}
	}
	return nil
}

func sigstoreVerificationOptions(opts BundleVerificationOptions, hasKey bool, useSignedTimestamps bool) ([]verify.VerifierOption, []verify.PolicyOption, error) {
	verifierOptions := []verify.VerifierOption{}
	policyOptions := []verify.PolicyOption{}
	if hasKey {
		policyOptions = append(policyOptions, verify.WithKey())
	} else {
		san, err := verify.NewSANMatcher(opts.CertificateIdentity, opts.CertificateIdentityRegexp)
		if err != nil {
			return nil, nil, err
		}
		issuer, err := verify.NewIssuerMatcher(opts.CertificateOIDCIssuer, opts.CertificateOIDCIssuerRegexp)
		if err != nil {
			return nil, nil, err
		}
		identity, err := verify.NewCertificateIdentity(san, issuer, certificate.Extensions{})
		if err != nil {
			return nil, nil, err
		}
		policyOptions = append(policyOptions, verify.WithCertificateIdentity(identity))
	}

	if !opts.IgnoreTlog {
		verifierOptions = append(verifierOptions, verify.WithTransparencyLog(1))
		if !useSignedTimestamps {
			if hasKey {
				verifierOptions = append(verifierOptions, verify.WithNoObserverTimestamps())
			} else {
				verifierOptions = append(verifierOptions, verify.WithIntegratedTimestamps(1))
			}
		}
	}
	if useSignedTimestamps {
		verifierOptions = append(verifierOptions, verify.WithSignedTimestamps(1))
	}
	if opts.IgnoreTlog && !useSignedTimestamps {
		if hasKey {
			verifierOptions = append(verifierOptions, verify.WithNoObserverTimestamps())
		} else {
			verifierOptions = append(verifierOptions, verify.WithCurrentTime())
		}
	}
	return verifierOptions, policyOptions, nil
}

func trustedMaterialForBundle(opts BundleVerificationOptions, keyVerifier signature.Verifier, useSignedTimestamps bool) (root.TrustedMaterial, error) {
	var material root.TrustedMaterial = &root.BaseTrustedMaterial{}
	needRoot := opts.TrustedRootPath != "" || keyVerifier == nil || !opts.IgnoreTlog || useSignedTimestamps
	if needRoot {
		var err error
		if path := opts.TrustedRootPath; path != "" {
			material, err = root.NewTrustedRootFromPath(path)
		} else {
			material, err = root.NewTrustedRootFromJSON(embeddedTrustedRoot)
		}
		if err != nil {
			return nil, fmt.Errorf("loading trusted root: %w", err)
		}
	}
	if keyVerifier == nil {
		return material, nil
	}
	expiringKey := root.NewExpiringKey(keyVerifier, time.Time{}, time.Time{})
	keyMaterial := root.NewTrustedPublicKeyMaterial(func(_ string) (root.TimeConstrainedVerifier, error) {
		return expiringKey, nil
	})
	return root.TrustedMaterialCollection{material, keyMaterial}, nil
}

func rekorBundleVersions(b *bundle.Bundle) (hasV1, hasV2 bool, err error) {
	entries, err := b.TlogEntries()
	if err != nil {
		return false, false, err
	}
	for _, entry := range entries {
		if entry.IntegratedTime().IsZero() {
			hasV2 = true
		} else {
			hasV1 = true
		}
	}
	return hasV1, hasV2, nil
}

func resolveBundleVerifier(ctx context.Context, opts BundleVerificationOptions, hashAlgorithm crypto.Hash) (signature.Verifier, func(), error) {
	if opts.Key == "" {
		return nil, func() {}, nil
	}
	if strings.HasPrefix(opts.Key, "k8s://") {
		secret, err := kubernetes.GetKeyPairSecret(ctx, opts.Key)
		if err != nil {
			return nil, func() {}, err
		}
		return verifierFromPEM(secret.Data["cosign.pub"], hashAlgorithm)
	}
	if strings.HasPrefix(opts.Key, "gitlab://") {
		provider, reference, ok := strings.Cut(opts.Key, "://")
		if !ok || reference == "" {
			return nil, func() {}, errors.New("could not parse key reference, use gitlab://<ref>")
		}
		gitProvider := cosigngit.GetProvider(provider)
		if gitProvider == nil {
			return nil, func() {}, fmt.Errorf("no git provider found for %q", provider)
		}
		publicKey, err := gitProvider.GetSecret(ctx, reference, "COSIGN_PUBLIC_KEY")
		if err != nil {
			return nil, func() {}, err
		}
		return verifierFromPEM([]byte(publicKey), hashAlgorithm)
	}
	if strings.HasPrefix(opts.Key, "pkcs11:") {
		config := pkcs11key.NewPkcs11UriConfig()
		if err := config.Parse(opts.Key); err != nil {
			return nil, func() {}, fmt.Errorf("parsing pkcs11 uri: %w", err)
		}
		key, err := pkcs11key.GetKeyWithURIConfig(config, false)
		if err != nil {
			return nil, func() {}, fmt.Errorf("opening pkcs11 token key: %w", err)
		}
		verifier, err := key.Verifier()
		if err != nil {
			key.Close()
			return nil, func() {}, fmt.Errorf("initializing pkcs11 token verifier: %w", err)
		}
		return verifier, key.Close, nil
	}

	verifier, err := kms.Get(ctx, opts.Key, hashAlgorithm)
	if err == nil {
		return verifier, func() {}, nil
	}
	var providerNotFound *kms.ProviderNotFoundError
	if !errors.As(err, &providerNotFound) {
		return nil, func() {}, fmt.Errorf("kms get: %w", err)
	}
	raw, err := loadPublicKeyReference(ctx, opts.Key)
	if err != nil {
		return nil, func() {}, err
	}
	return verifierFromPEM(raw, hashAlgorithm)
}

func verifierFromPEM(raw []byte, hashAlgorithm crypto.Hash) (signature.Verifier, func(), error) {
	publicKey, err := cryptoutils.UnmarshalPEMToPublicKey(raw)
	if err != nil {
		return nil, func() {}, fmt.Errorf("pem to public key: %w", err)
	}
	verifier, err := signature.LoadVerifier(publicKey, hashAlgorithm)
	return verifier, func() {}, err
}

func loadPublicKeyReference(ctx context.Context, reference string) ([]byte, error) {
	switch {
	case strings.HasPrefix(reference, "env://"):
		value, ok := os.LookupEnv(strings.TrimPrefix(reference, "env://"))
		if !ok {
			return nil, fmt.Errorf("loading URL: env var $%s not found", strings.TrimPrefix(reference, "env://"))
		}
		return []byte(value), nil
	case strings.HasPrefix(reference, "http://") || strings.HasPrefix(reference, "https://"):
		// #nosec G107 -- the public key location is an explicit user input.
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, reference, nil)
		if err != nil {
			return nil, err
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			return nil, err
		}
		if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
			if closeErr := response.Body.Close(); closeErr != nil {
				return nil, closeErr
			}
			return nil, fmt.Errorf("loading URL %s: server returned HTTP %d", reference, response.StatusCode)
		}
		raw, readErr := io.ReadAll(response.Body)
		if closeErr := response.Body.Close(); closeErr != nil && readErr == nil {
			return nil, closeErr
		}
		return raw, readErr
	case strings.Contains(reference, "://"):
		return nil, fmt.Errorf("loading URL: unrecognized scheme: %s", strings.SplitN(reference, "://", 2)[0]+"://")
	default:
		return os.ReadFile(filepath.Clean(reference))
	}
}

func readBundleArtifact(ctx context.Context, reference string) ([]byte, error) {
	if reference == "-" {
		return io.ReadAll(os.Stdin)
	}
	return loadPublicKeyReference(ctx, reference)
}

// bundleArtifactPolicy mirrors cosign's blob verifier: an unreadable artifact
// may instead be an explicitly supplied algorithm:hex-digest reference.
func bundleArtifactPolicy(ctx context.Context, reference string) (verify.ArtifactPolicyOption, error) {
	artifact, readErr := readBundleArtifact(ctx, reference)
	if readErr == nil {
		return verify.WithArtifact(bytes.NewReader(artifact)), nil
	}

	algorithm, encodedDigest, found := strings.Cut(reference, ":")
	if !found {
		return nil, readErr
	}
	digest, err := hex.DecodeString(encodedDigest)
	if err != nil {
		return nil, err
	}
	return verify.WithArtifactDigest(algorithm, digest), nil
}
