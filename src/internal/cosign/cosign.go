// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cosign provides generic cosign functions.
package cosign

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

// SignBlobOptions embeds Cosign's native options and adds Zarf-specific configuration.
// By embedding options.KeyOpts, we get direct access to all Cosign signing capabilities
// while maintaining a clean interface for Zarf users.
type SignBlobOptions struct {
	// Embed Cosign's KeyOpts for signing configuration
	options.KeyOpts

	// Zarf-specific options for output control
	OutputSignature   string // Custom path for signature file
	OutputCertificate string // Where to write certificate (keyless mode)

	// General options
	Verbose bool          // Enable debug output
	Timeout time.Duration // Timeout for signing operations
}

// VerifyBlobOptions embeds Cosign's native options for verification.
// By embedding options.KeyOpts and options.CertVerifyOptions, we get direct access
// to all Cosign verification capabilities.
type VerifyBlobOptions struct {
	// Embed Cosign's KeyOpts for key-based verification
	options.KeyOpts

	// Embed Cosign's CertVerifyOptions for certificate-based (keyless) verification
	options.CertVerifyOptions

	// Verification-specific options
	SigRef     string // Path to signature file
	Offline    bool   // Enable offline verification mode
	IgnoreTlog bool   // Skip transparency log verification

	// General options
	Timeout time.Duration // Timeout for verification operations
}

// ShouldSign returns true if the options indicate that signing should be performed.
// This checks if any signing key material is configured (KeyRef, IDToken, or Sk).
func (opts SignBlobOptions) ShouldSign() bool {
	return opts.KeyRef != "" || opts.IDToken != "" || opts.Sk
}

// DefaultSignBlobOptions returns SignBlobOptions with Zarf defaults.
// Configures sensible defaults for offline/air-gapped environments.
func DefaultSignBlobOptions() SignBlobOptions {
	return SignBlobOptions{
		KeyOpts: options.KeyOpts{
			Slot:             "signature",
			OIDCIssuer:       "https://oauth2.sigstore.dev/auth",
			OIDCClientID:     "sigstore",
			OIDCRedirectURL:  "http://localhost:0/auth/callback",
			FulcioAuthFlow:   "normal",
			FulcioURL:        "https://fulcio.sigstore.dev",
			RekorURL:         "https://rekor.sigstore.dev",
			NewBundleFormat:  false,
			SkipConfirmation: false,
		},
		Timeout: CosignDefaultTimeout,
		Verbose: false,
	}
}

// DefaultVerifyBlobOptions returns VerifyBlobOptions with Zarf defaults.
// Configures sensible defaults for offline/air-gapped environments.
func DefaultVerifyBlobOptions() VerifyBlobOptions {
	return VerifyBlobOptions{
		CertVerifyOptions: options.CertVerifyOptions{
			IgnoreSCT: true, // Skip SCT verification by default
		},
		Offline:    true,
		IgnoreTlog: true,
		Timeout:    CosignDefaultTimeout,
	}
}

// CosignSignBlobWithOptions signs a blob with comprehensive cosign options.
// This function supports all cosign v3 sign-blob capabilities by leveraging
// the embedded KeyOpts structure.
func CosignSignBlobWithOptions(ctx context.Context, blobPath string, opts SignBlobOptions) ([]byte, error) {
	l := logger.From(ctx)

	// Build root options
	rootOpts := &options.RootOptions{
		Verbose: opts.Verbose,
		Timeout: opts.Timeout,
	}

	// Use the embedded KeyOpts directly - no need to copy fields!
	keyOpts := opts.KeyOpts

	l.Debug("signing blob with cosign",
		"keyRef", opts.KeyRef,
		"sk", opts.Sk,
		"bundlePath", opts.BundlePath)

	// SignBlobCmd signature: (ro *RootOptions, ko KeyOpts, payloadPath string, b64 bool, outputSignature string, outputCertificate string, tlogUpload bool)
	// Note: Some params like b64 and tlogUpload are not in KeyOpts, so we need to handle defaults
	b64 := true         // Default: base64 encode signature
	tlogUpload := false // Zarf default: don't upload to transparency log (offline/air-gap friendly)

	sig, err := sign.SignBlobCmd(
		rootOpts,
		keyOpts,
		blobPath,
		b64,
		opts.OutputSignature,
		opts.OutputCertificate,
		tlogUpload,
	)
	if err != nil {
		return nil, err
	}

	l.Debug("blob signed successfully", "signatureLength", len(sig))
	return sig, nil
}

// CosignVerifyBlobWithOptions verifies a blob signature with comprehensive cosign options.
// This function supports all cosign v3 verify-blob capabilities by leveraging
// the embedded KeyOpts and CertVerifyOptions structures.
func CosignVerifyBlobWithOptions(ctx context.Context, blobPath string, opts VerifyBlobOptions) error {
	l := logger.From(ctx)

	// Use the embedded structs directly - no need to copy fields!
	keyOpts := opts.KeyOpts
	certVerifyOpts := opts.CertVerifyOptions

	cmd := &verify.VerifyBlobCmd{
		KeyOpts:           keyOpts,
		CertVerifyOptions: certVerifyOpts,
		SigRef:            opts.SigRef,
		IgnoreSCT:         opts.IgnoreSCT, // From CertVerifyOptions
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
