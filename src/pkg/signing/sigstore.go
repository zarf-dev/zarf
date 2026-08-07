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
	"github.com/sigstore/cosign/v3/pkg/cosign/pivkey"
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

// SigstoreVerifyBundleWithOptions verifies a Sigstore bundle directly with
// sigstore-go and returns the verified bundle contents. Callers must use
// CosignVerifyBlobWithOptions for legacy .sig files.
func SigstoreVerifyBundleWithOptions(ctx context.Context, blobPath string, opts VerifyBlobOptions) (*verify.VerificationResult, error) {
	l := logger.From(ctx)

	if opts.KeyRef != "" {
		l.Warn("VerifyBlobOptions.KeyRef is deprecated, use Key (removed in v1.0)")
		if opts.Key == "" {
			opts.Key = opts.KeyRef
		}
	}
	if opts.SigRef != "" {
		l.Warn("VerifyBlobOptions.SigRef is deprecated, use Signature (removed in v1.0)")
		if opts.Signature == "" {
			opts.Signature = opts.SigRef
		}
	}
	if err := validateSigstoreBundleOptions(opts); err != nil {
		return nil, err
	}
	if opts.BundlePath == "" {
		return nil, errors.New("bundle path is required")
	}
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	b, err := bundle.LoadJSONFromPath(opts.BundlePath)
	if err != nil {
		return nil, fmt.Errorf("loading Sigstore bundle: %w", err)
	}

	hashAlgorithm, err := opts.SignatureDigest.HashAlgorithm()
	if err != nil {
		return nil, err
	}
	keyVerifier, closeKey, err := resolveBundleVerifier(ctx, opts, hashAlgorithm)
	if err != nil {
		return nil, fmt.Errorf("loading verifier from key options: %w", err)
	}
	defer closeKey()

	useSignedTimestamps := opts.CommonVerifyOptions.UseSignedTimestamps
	if !opts.CommonVerifyOptions.IgnoreTlog && keyVerifier == nil {
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

	artifactPolicy, err := bundleArtifactPolicy(blobPath)
	if err != nil {
		return nil, err
	}
	result, err := sev.Verify(b, verify.NewPolicy(artifactPolicy, policyOptions...))
	if err != nil {
		return nil, err
	}
	l.Debug("blob signature verified successfully with sigstore-go")
	return result, nil
}

func validateSigstoreBundleOptions(opts VerifyBlobOptions) error {
	if opts.Key != "" && (opts.CertVerify.CertIdentity != "" || opts.CertVerify.CertIdentityRegexp != "") {
		return errors.New("key cannot be combined with certificate identity verification")
	}
	if opts.Key != "" && opts.SecurityKey.Use {
		return errors.New("key cannot be combined with security-key verification")
	}
	// These options are rejected by cosign for protobuf bundles. Keeping that
	// contract prevents detached material from silently weakening verification.
	unsupported := []struct {
		value string
		name  string
	}{
		{opts.Signature, "detached signature"},
		{opts.CertVerify.Cert, "certificate"},
		{opts.CertVerify.CertChain, "certificate chain"},
		{opts.CertVerify.CARoots, "certificate authority roots"},
		{opts.CertVerify.CAIntermediates, "certificate authority intermediates"},
		{opts.CommonVerifyOptions.TSACertChainPath, "timestamp certificate chain"},
		{opts.CertVerify.SCT, "signed certificate timestamp"},
	}
	for _, option := range unsupported {
		if option.value != "" {
			return fmt.Errorf("unsupported verification material for Sigstore bundles: %s", option.name)
		}
	}
	return nil
}

func sigstoreVerificationOptions(opts VerifyBlobOptions, hasKey bool, useSignedTimestamps bool) ([]verify.VerifierOption, []verify.PolicyOption, error) {
	verifierOptions := []verify.VerifierOption{}
	policyOptions := []verify.PolicyOption{}
	if hasKey {
		policyOptions = append(policyOptions, verify.WithKey())
	} else {
		san, err := verify.NewSANMatcher(opts.CertVerify.CertIdentity, opts.CertVerify.CertIdentityRegexp)
		if err != nil {
			return nil, nil, err
		}
		issuer, err := verify.NewIssuerMatcher(opts.CertVerify.CertOidcIssuer, opts.CertVerify.CertOidcIssuerRegexp)
		if err != nil {
			return nil, nil, err
		}
		identity, err := verify.NewCertificateIdentity(san, issuer, certificate.Extensions{
			GithubWorkflowTrigger:    opts.CertVerify.CertGithubWorkflowTrigger,
			GithubWorkflowSHA:        opts.CertVerify.CertGithubWorkflowSha,
			GithubWorkflowName:       opts.CertVerify.CertGithubWorkflowName,
			GithubWorkflowRepository: opts.CertVerify.CertGithubWorkflowRepository,
			GithubWorkflowRef:        opts.CertVerify.CertGithubWorkflowRef,
		})
		if err != nil {
			return nil, nil, err
		}
		policyOptions = append(policyOptions, verify.WithCertificateIdentity(identity))
		if !opts.CertVerify.IgnoreSCT {
			verifierOptions = append(verifierOptions, verify.WithSignedCertificateTimestamps(1))
		}
	}

	if !opts.CommonVerifyOptions.IgnoreTlog {
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
	if opts.CommonVerifyOptions.IgnoreTlog && !useSignedTimestamps {
		if hasKey {
			verifierOptions = append(verifierOptions, verify.WithNoObserverTimestamps())
		} else {
			verifierOptions = append(verifierOptions, verify.WithCurrentTime())
		}
	}
	return verifierOptions, policyOptions, nil
}

func trustedMaterialForBundle(opts VerifyBlobOptions, keyVerifier signature.Verifier, useSignedTimestamps bool) (root.TrustedMaterial, error) {
	var material root.TrustedMaterial = &root.BaseTrustedMaterial{}
	needRoot := opts.CommonVerifyOptions.TrustedRootPath != "" || keyVerifier == nil || !opts.CommonVerifyOptions.IgnoreTlog || useSignedTimestamps
	if needRoot {
		var err error
		if path := opts.CommonVerifyOptions.TrustedRootPath; path != "" {
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

func resolveBundleVerifier(ctx context.Context, opts VerifyBlobOptions, hashAlgorithm crypto.Hash) (signature.Verifier, func(), error) {
	if opts.SecurityKey.Use {
		key, err := pivkey.GetKeyWithSlot(opts.SecurityKey.Slot)
		if err != nil {
			return nil, func() {}, err
		}
		verifier, err := key.Verifier()
		if err != nil {
			key.Close()
			return nil, func() {}, err
		}
		return verifier, key.Close, nil
	}
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
	raw, err := loadPublicKeyReference(opts.Key)
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

func loadPublicKeyReference(reference string) ([]byte, error) {
	switch {
	case strings.HasPrefix(reference, "env://"):
		value, ok := os.LookupEnv(strings.TrimPrefix(reference, "env://"))
		if !ok {
			return nil, fmt.Errorf("loading URL: env var $%s not found", strings.TrimPrefix(reference, "env://"))
		}
		return []byte(value), nil
	case strings.HasPrefix(reference, "http://") || strings.HasPrefix(reference, "https://"):
		// #nosec G107 -- the public key location is an explicit user input.
		response, err := http.Get(reference)
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

func readBundleArtifact(reference string) ([]byte, error) {
	if reference == "-" {
		return io.ReadAll(os.Stdin)
	}
	return loadPublicKeyReference(reference)
}

// bundleArtifactPolicy mirrors cosign's blob verifier: an unreadable artifact
// may instead be an explicitly supplied algorithm:hex-digest reference.
func bundleArtifactPolicy(reference string) (verify.ArtifactPolicyOption, error) {
	artifact, readErr := readBundleArtifact(reference)
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
