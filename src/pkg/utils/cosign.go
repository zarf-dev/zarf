// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic utility functions.
package utils

import (
	"context"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/sigstore/cosign/v3/cmd/cosign/cli/options"
	"github.com/sigstore/cosign/v3/cmd/cosign/cli/sign"
	"github.com/sigstore/cosign/v3/cmd/cosign/cli/verify"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	ociremote "github.com/sigstore/cosign/v3/pkg/oci/remote"

	// Register the provider-specific plugins
	_ "github.com/sigstore/sigstore/pkg/signature/kms/aws"
	_ "github.com/sigstore/sigstore/pkg/signature/kms/azure"
	_ "github.com/sigstore/sigstore/pkg/signature/kms/gcp"
	_ "github.com/sigstore/sigstore/pkg/signature/kms/hashivault"

	"github.com/zarf-dev/zarf/src/pkg/logger"
)

// Default cosign configuration
const (
	CosignDefaultTimeout = 3 * time.Minute
)

// SignBlobOptions contains all options for cosign sign-blob operations.
// This structure supports all cosign v3 sign-blob capabilities.
type SignBlobOptions struct {
	// ========================================
	// Basic Key Options
	// ========================================

	// KeyRef is the path to private key or KMS URI
	KeyRef string

	// PassFunc provides password for encrypted keys
	PassFunc cosign.PassFunc

	// ========================================
	// Security Key Options
	// ========================================

	// Sk enables hardware security key (YubiKey, etc.)
	Sk bool

	// Slot specifies the security key slot
	// Options: authentication, signature, card-authentication, key-management
	// Default: "signature"
	Slot string

	// ========================================
	// Keyless/OIDC Options
	// ========================================

	// IDToken is the OIDC identity token or path to token file
	IDToken string

	// OIDCIssuer is the OIDC provider URL
	// Default: "https://oauth2.sigstore.dev/auth"
	OIDCIssuer string

	// OIDCClientID is the OIDC client ID
	// Default: "sigstore"
	OIDCClientID string

	// OIDCClientSecret is the OIDC client secret
	OIDCClientSecret string

	// OIDCRedirectURL is the redirect URL for OAuth flow
	// Default: "http://localhost:0/auth/callback"
	OIDCRedirectURL string

	// OIDCProvider specifies the OIDC provider
	// Options: spiffe, google, github-actions, filesystem, buildkite-agent
	OIDCProvider string

	// OIDCDisableProviders disables ambient OIDC credential detection
	OIDCDisableProviders bool

	// FulcioAuthFlow specifies the OAuth2 flow
	// Options: normal, device, token, client_credentials
	// Default: "normal"
	FulcioAuthFlow string

	// ========================================
	// Sigstore Infrastructure
	// ========================================

	// FulcioURL is the Fulcio PKI server URL
	// Default: "https://fulcio.sigstore.dev"
	FulcioURL string

	// RekorURL is the Rekor transparency log URL
	// Default: "https://rekor.sigstore.dev"
	RekorURL string

	// TLogUpload enables transparency log upload
	// Default: false for Zarf (explicit opt-in)
	TLogUpload bool

	// InsecureSkipFulcioVerify skips Fulcio SCT verification (testing only)
	InsecureSkipFulcioVerify bool

	// IssueCertificateForExistingKey issues a Fulcio certificate even when using existing key
	IssueCertificateForExistingKey bool

	// ========================================
	// Timestamp Authority (RFC 3161)
	// ========================================

	// TSAServerURL is the RFC3161 timestamp server URL
	TSAServerURL string

	// RFC3161TimestampPath is where to write the RFC3161 timestamp
	RFC3161TimestampPath string

	// TSAClientCert is the X.509 client certificate for TSA (PEM)
	TSAClientCert string

	// TSAClientKey is the X.509 client private key for TSA (PEM)
	TSAClientKey string

	// TSAClientCACert is the X.509 CA certificate for TSA verification (PEM)
	TSAClientCACert string

	// TSAServerName is the expected SAN in TSA server certificate
	TSAServerName string

	// ========================================
	// Bundle & Output Options
	// ========================================

	// BundlePath is where to write the verification bundle
	// Bundle contains signature, certificate, timestamp, rekor entry
	BundlePath string

	// NewBundleFormat uses protobuf bundle format (vs legacy JSON)
	// Default: true
	NewBundleFormat bool

	// OutputSignature is a custom path for the signature file
	OutputSignature string

	// OutputCertificate is where to write the certificate (keyless mode)
	OutputCertificate string

	// B64 controls base64 encoding of outputs
	// Default: true
	B64 bool

	// ========================================
	// General Options
	// ========================================

	// SkipConfirmation skips confirmation prompts
	SkipConfirmation bool

	// Timeout for signing operations
	// Default: 3m0s
	Timeout time.Duration

	// Verbose enables debug output
	Verbose bool
}

// VerifyBlobOptions contains all options for cosign verify-blob operations.
// This structure supports all cosign v3 verify-blob capabilities.
type VerifyBlobOptions struct {
	// KeyRef is the path to public key
	KeyRef string

	// ========================================
	// Keyless Verification Options
	// ========================================

	// CertificateIdentity is the expected identity in the certificate
	CertificateIdentity string

	// CertificateOIDCIssuer is the expected OIDC issuer
	CertificateOIDCIssuer string

	// CertificateChain is the path to certificate chain
	CertificateChain string

	// ========================================
	// Bundle Verification
	// ========================================

	// BundlePath is the path to verification bundle
	BundlePath string

	// ========================================
	// Transparency Log Options
	// ========================================

	// RekorURL is the Rekor transparency log URL
	RekorURL string

	// IgnoreTlog skips transparency log verification
	IgnoreTlog bool

	// ========================================
	// Signature Options
	// ========================================

	// SigRef is the path to signature file
	SigRef string

	// IgnoreSCT skips SCT verification
	IgnoreSCT bool

	// Offline enables offline verification mode
	Offline bool

	// ========================================
	// Timestamp Options
	// ========================================

	// RFC3161TimestampPath is the path to RFC3161 timestamp
	RFC3161TimestampPath string

	// TSACertChainPath is the path to TSA certificate chain
	TSACertChainPath string

	// ========================================
	// General Options
	// ========================================

	// Timeout for verification operations
	Timeout time.Duration
}

// ShouldSign returns true if the options indicate that signing should be performed.
// This checks if any signing key material is configured (KeyRef, IDToken, or Sk).
func (opts SignBlobOptions) ShouldSign() bool {
	return opts.KeyRef != "" || opts.IDToken != "" || opts.Sk
}

// DefaultSignBlobOptions returns SignBlobOptions with Zarf defaults
func DefaultSignBlobOptions() SignBlobOptions {
	return SignBlobOptions{
		Slot:            "signature",
		OIDCIssuer:      "https://oauth2.sigstore.dev/auth",
		OIDCClientID:    "sigstore",
		OIDCRedirectURL: "http://localhost:0/auth/callback",
		FulcioAuthFlow:  "normal",
		FulcioURL:       "https://fulcio.sigstore.dev",
		RekorURL:        "https://rekor.sigstore.dev",
		TLogUpload:      false, // Zarf default: explicit opt-in
		NewBundleFormat: true,
		B64:             true,
		Timeout:         CosignDefaultTimeout,
		Verbose:         false,
	}
}

// DefaultVerifyBlobOptions returns VerifyBlobOptions with Zarf defaults
func DefaultVerifyBlobOptions() VerifyBlobOptions {
	return VerifyBlobOptions{
		IgnoreSCT:  true,
		Offline:    true,
		IgnoreTlog: true,
		Timeout:    CosignDefaultTimeout,
	}
}

// CosignSignBlobWithOptions signs a blob with comprehensive cosign options.
// This function supports all cosign v3 sign-blob capabilities.
func CosignSignBlobWithOptions(ctx context.Context, blobPath string, opts SignBlobOptions) ([]byte, error) {
	l := logger.From(ctx)

	// Build root options
	rootOpts := &options.RootOptions{
		Verbose: opts.Verbose,
		Timeout: opts.Timeout,
	}

	// Build comprehensive key options
	keyOpts := options.KeyOpts{
		// Basic
		KeyRef:   opts.KeyRef,
		PassFunc: opts.PassFunc,

		// Security Key
		Sk:   opts.Sk,
		Slot: opts.Slot,

		// OIDC
		IDToken:              opts.IDToken,
		OIDCIssuer:           opts.OIDCIssuer,
		OIDCClientID:         opts.OIDCClientID,
		OIDCClientSecret:     opts.OIDCClientSecret,
		OIDCRedirectURL:      opts.OIDCRedirectURL,
		OIDCProvider:         opts.OIDCProvider,
		OIDCDisableProviders: opts.OIDCDisableProviders,
		FulcioAuthFlow:       opts.FulcioAuthFlow,

		// Sigstore
		FulcioURL:                      opts.FulcioURL,
		RekorURL:                       opts.RekorURL,
		InsecureSkipFulcioVerify:       opts.InsecureSkipFulcioVerify,
		IssueCertificateForExistingKey: opts.IssueCertificateForExistingKey,

		// TSA
		TSAServerURL:         opts.TSAServerURL,
		TSAClientCert:        opts.TSAClientCert,
		TSAClientKey:         opts.TSAClientKey,
		TSAClientCACert:      opts.TSAClientCACert,
		TSAServerName:        opts.TSAServerName,
		RFC3161TimestampPath: opts.RFC3161TimestampPath,

		// Bundle
		BundlePath:      opts.BundlePath,
		NewBundleFormat: opts.NewBundleFormat,

		// Confirmation
		SkipConfirmation: opts.SkipConfirmation,
	}

	l.Debug("signing blob with cosign",
		"keyRef", opts.KeyRef,
		"sk", opts.Sk,
		"tlogUpload", opts.TLogUpload,
		"bundlePath", opts.BundlePath)

	sig, err := sign.SignBlobCmd(
		rootOpts,
		keyOpts,
		blobPath,
		opts.B64,
		opts.OutputSignature,
		opts.OutputCertificate,
		opts.TLogUpload,
	)
	if err != nil {
		return nil, err
	}

	l.Debug("blob signed successfully", "signatureLength", len(sig))
	return sig, nil
}

// CosignVerifyBlobWithOptions verifies a blob signature with comprehensive cosign options.
// This function supports all cosign v3 verify-blob capabilities.
func CosignVerifyBlobWithOptions(ctx context.Context, blobPath string, opts VerifyBlobOptions) error {
	l := logger.From(ctx)

	keyOpts := options.KeyOpts{
		KeyRef:               opts.KeyRef,
		BundlePath:           opts.BundlePath,
		RekorURL:             opts.RekorURL,
		RFC3161TimestampPath: opts.RFC3161TimestampPath,
		TSACertChainPath:     opts.TSACertChainPath,
	}

	certVerifyOpts := options.CertVerifyOptions{
		CertIdentity:   opts.CertificateIdentity,
		CertOidcIssuer: opts.CertificateOIDCIssuer,
		CertChain:      opts.CertificateChain,
		IgnoreSCT:      opts.IgnoreSCT,
	}

	cmd := &verify.VerifyBlobCmd{
		KeyOpts:           keyOpts,
		CertVerifyOptions: certVerifyOpts,
		SigRef:            opts.SigRef,
		IgnoreSCT:         opts.IgnoreSCT,
		Offline:           opts.Offline,
		IgnoreTlog:        opts.IgnoreTlog,
	}

	l.Debug("verifying blob with cosign",
		"keyRef", opts.KeyRef,
		"sigRef", opts.SigRef,
		"offline", opts.Offline)

	err := cmd.Exec(ctx, blobPath)
	if err != nil {
		return err
	}

	l.Debug("blob signature verified successfully")
	return nil
}

// CosignVerifyBlob verifies a signature using basic options (legacy function, maintained for compatibility).
// For new code, use CosignVerifyBlobWithOptions for full control.
func CosignVerifyBlob(ctx context.Context, blobRef, sigRef, keyPath string) error {
	opts := DefaultVerifyBlobOptions()
	opts.KeyRef = keyPath
	opts.SigRef = sigRef
	return CosignVerifyBlobWithOptions(ctx, blobRef, opts)
}

// CosignSignBlob signs a blob using basic options (legacy function, maintained for compatibility).
// For new code, use CosignSignBlobWithOptions for full control.
func CosignSignBlob(blobPath, outputSigPath, keyPath string, passFn cosign.PassFunc) ([]byte, error) {
	ctx := context.Background()
	opts := DefaultSignBlobOptions()
	opts.KeyRef = keyPath
	opts.PassFunc = passFn
	opts.OutputSignature = outputSigPath
	return CosignSignBlobWithOptions(ctx, blobPath, opts)
}

// GetCosignArtifacts returns signatures and attestations for the given image
func GetCosignArtifacts(image string) ([]string, error) {
	var nameOpts []name.Option

	ref, err := name.ParseReference(image, nameOpts...)
	if err != nil {
		return nil, err
	}

	// Return empty if we don't have a signature on the image
	var remoteOpts []ociremote.Option
	simg, _ := ociremote.SignedEntity(ref, remoteOpts...) //nolint:errcheck
	if simg == nil {
		return nil, nil
	}

	// Errors are dogsled because these functions always return a name.Tag which we can check for layers
	sigRef, _ := ociremote.SignatureTag(ref, remoteOpts...)   //nolint:errcheck
	attRef, _ := ociremote.AttestationTag(ref, remoteOpts...) //nolint:errcheck

	ss, err := simg.Signatures()
	if err != nil {
		return nil, err
	}
	ssLayers, err := ss.Layers()
	if err != nil {
		return nil, err
	}

	var cosignArtifactList = make([]string, 0)
	if 0 < len(ssLayers) {
		cosignArtifactList = append(cosignArtifactList, sigRef.String())
	}

	atts, err := simg.Attestations()
	if err != nil {
		return nil, err
	}
	aLayers, err := atts.Layers()
	if err != nil {
		return nil, err
	}
	if 0 < len(aLayers) {
		cosignArtifactList = append(cosignArtifactList, attRef.String())
	}
	return cosignArtifactList, nil
}
