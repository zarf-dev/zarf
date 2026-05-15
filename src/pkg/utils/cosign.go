// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic utility functions.
package utils

import (
	"context"
	"fmt"
	"os"
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

// CosignDefaultTimeout is the default timeout for cosign sign and verify operations.
const CosignDefaultTimeout = 3 * time.Minute

// nonPromptingPassFunc resolves a private-key password without ever blocking on
// terminal or stdin input.
var nonPromptingPassFunc = cosign.PassFunc(func(_ bool) ([]byte, error) {
	if pw, ok := os.LookupEnv("COSIGN_PASSWORD"); ok {
		return []byte(pw), nil
	}
	return []byte{}, nil
})

// SignBlobOptions wraps cosign's SignBlobOptions with zarf-specific fields.
type SignBlobOptions struct {
	options.SignBlobOptions

	Verbose   bool
	Timeout   time.Duration
	Password  string
	PassFunc  cosign.PassFunc
	Overwrite bool

	// Deprecated: use Key (promoted from the embedded SignBlobOptions). Removed in v1.0.
	KeyRef string
}

// VerifyBlobOptions wraps cosign's VerifyBlobOptions with zarf-specific fields.
type VerifyBlobOptions struct {
	options.VerifyBlobOptions

	Timeout time.Duration

	// Deprecated: use Key (promoted from the embedded VerifyBlobOptions). Removed in v1.0.
	KeyRef string
	// Deprecated: use Signature (promoted from the embedded VerifyBlobOptions). Removed in v1.0.
	SigRef string
}

// ShouldSign returns true if any signing key material is configured.
// KeyRef is included for backward compatibility; it's synced to Key in
// CosignSignBlobWithOptions.
func (opts SignBlobOptions) ShouldSign() bool {
	return opts.Key != "" || opts.KeyRef != "" || opts.Fulcio.IdentityToken != "" || opts.SecurityKey.Use
}

// CheckOverwrite errors if any output file exists and Overwrite is false.
func (opts SignBlobOptions) CheckOverwrite(ctx context.Context) error {
	for _, path := range []string{opts.BundlePath, opts.OutputCertificate, opts.OutputSignature} {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			if !opts.Overwrite {
				return fmt.Errorf("file at path %s already exists", path)
			}
			logger.From(ctx).Debug("overwriting existing file", "path", path)
		}
	}
	return nil
}

// DefaultSignBlobOptions returns SignBlobOptions seeded with zarf defaults.
// Divergence: TlogUpload defaults to false (cosign default true) for airgap.
func DefaultSignBlobOptions() SignBlobOptions {
	var opts SignBlobOptions
	opts.TlogUpload = false
	opts.Base64Output = true
	opts.NewBundleFormat = true
	opts.SecurityKey.Slot = "signature"
	opts.OIDC.ClientID = "sigstore"
	opts.Fulcio.AuthFlow = "normal"
	opts.Timeout = CosignDefaultTimeout
	return opts
}

// DefaultVerifyBlobOptions returns VerifyBlobOptions seeded with zarf defaults.
// Divergences: IgnoreTlog and IgnoreSCT default to true (cosign default false) for airgap.
func DefaultVerifyBlobOptions() VerifyBlobOptions {
	var opts VerifyBlobOptions
	opts.CommonVerifyOptions.IgnoreTlog = true
	opts.CertVerify.IgnoreSCT = true
	opts.CommonVerifyOptions.NewBundleFormat = true
	opts.Timeout = CosignDefaultTimeout
	return opts
}

// CosignSignBlobWithOptions signs a blob via cosign's SignBlobCmd.
// Mirrors cmd/cosign/cli/signblob.go (v3.0.6) SignBlob().RunE.
func CosignSignBlobWithOptions(ctx context.Context, blobPath string, opts SignBlobOptions) ([]byte, error) {
	l := logger.From(ctx)

	if opts.KeyRef != "" {
		l.Warn("SignBlobOptions.KeyRef is deprecated, use Key (removed in v1.0)")
		if opts.Key == "" {
			opts.Key = opts.KeyRef
		}
	}

	rootOpts := &options.RootOptions{
		Verbose: opts.Verbose,
		Timeout: opts.Timeout,
	}

	oidcClientSecret, err := opts.OIDC.ClientSecret()
	if err != nil {
		return nil, err
	}

	ko := options.KeyOpts{
		KeyRef:                         opts.Key,
		PassFunc:                       nonPromptingPassFunc,
		Sk:                             opts.SecurityKey.Use,
		Slot:                           opts.SecurityKey.Slot,
		FulcioURL:                      opts.Fulcio.URL,
		IDToken:                        opts.Fulcio.IdentityToken,
		FulcioAuthFlow:                 opts.Fulcio.AuthFlow,
		InsecureSkipFulcioVerify:       opts.Fulcio.InsecureSkipFulcioVerify,
		RekorURL:                       opts.Rekor.URL,
		OIDCIssuer:                     opts.OIDC.Issuer,
		OIDCClientID:                   opts.OIDC.ClientID,
		OIDCClientSecret:               oidcClientSecret,
		OIDCRedirectURL:                opts.OIDC.RedirectURL,
		OIDCDisableProviders:           opts.OIDC.DisableAmbientProviders,
		BundlePath:                     opts.BundlePath,
		NewBundleFormat:                opts.NewBundleFormat,
		SkipConfirmation:               opts.SkipConfirmation,
		TSAClientCACert:                opts.TSAClientCACert,
		TSAClientCert:                  opts.TSAClientCert,
		TSAClientKey:                   opts.TSAClientKey,
		TSAServerName:                  opts.TSAServerName,
		TSAServerURL:                   opts.TSAServerURL,
		RFC3161TimestampPath:           opts.RFC3161TimestampPath,
		IssueCertificateForExistingKey: opts.IssueCertificate,
		SigningAlgorithm:               opts.SigningAlgorithm,
	}

	switch {
	case opts.PassFunc != nil:
		ko.PassFunc = opts.PassFunc
	case opts.Password != "":
		password := opts.Password
		ko.PassFunc = cosign.PassFunc(func(_ bool) ([]byte, error) {
			return []byte(password), nil
		})
	}

	if err := opts.CheckOverwrite(ctx); err != nil {
		return nil, err
	}

	l.Debug("signing blob with cosign",
		"key", opts.Key,
		"sk", opts.SecurityKey.Use,
		"bundlePath", opts.BundlePath)

	sig, err := sign.SignBlobCmd(
		ctx,
		rootOpts,
		ko,
		blobPath,
		opts.Cert,
		opts.CertChain,
		opts.Base64Output,
		opts.OutputSignature,
		opts.OutputCertificate,
		opts.TlogUpload,
	)
	if err != nil {
		return nil, err
	}

	l.Debug("blob signed successfully", "signatureLength", len(sig))
	return sig, nil
}

// CosignVerifyBlobWithOptions verifies a blob via cosign's VerifyBlobCmd.
// Mirrors cmd/cosign/cli/verify.go (v3.0.6) VerifyBlob().RunE.
func CosignVerifyBlobWithOptions(ctx context.Context, blobPath string, opts VerifyBlobOptions) error {
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

	hashAlgorithm, err := opts.SignatureDigest.HashAlgorithm()
	if err != nil {
		return err
	}

	if opts.CommonVerifyOptions.PrivateInfrastructure {
		opts.CommonVerifyOptions.IgnoreTlog = true
	}

	ko := options.KeyOpts{
		KeyRef:               opts.Key,
		Sk:                   opts.SecurityKey.Use,
		Slot:                 opts.SecurityKey.Slot,
		RekorURL:             opts.Rekor.URL,
		BundlePath:           opts.BundlePath,
		RFC3161TimestampPath: opts.RFC3161TimestampPath,
		TSACertChainPath:     opts.CommonVerifyOptions.TSACertChainPath,
		NewBundleFormat:      opts.CommonVerifyOptions.NewBundleFormat,
	}

	cmd := &verify.VerifyBlobCmd{
		KeyOpts:                      ko,
		CertVerifyOptions:            opts.CertVerify,
		CertRef:                      opts.CertVerify.Cert,
		CertChain:                    opts.CertVerify.CertChain,
		CARoots:                      opts.CertVerify.CARoots,
		CAIntermediates:              opts.CertVerify.CAIntermediates,
		SigRef:                       opts.Signature,
		CertGithubWorkflowTrigger:    opts.CertVerify.CertGithubWorkflowTrigger,
		CertGithubWorkflowSHA:        opts.CertVerify.CertGithubWorkflowSha,
		CertGithubWorkflowName:       opts.CertVerify.CertGithubWorkflowName,
		CertGithubWorkflowRepository: opts.CertVerify.CertGithubWorkflowRepository,
		CertGithubWorkflowRef:        opts.CertVerify.CertGithubWorkflowRef,
		IgnoreSCT:                    opts.CertVerify.IgnoreSCT,
		SCTRef:                       opts.CertVerify.SCT,
		Offline:                      opts.CommonVerifyOptions.Offline,
		IgnoreTlog:                   opts.CommonVerifyOptions.IgnoreTlog,
		UseSignedTimestamps:          opts.CommonVerifyOptions.UseSignedTimestamps,
		TrustedRootPath:              opts.CommonVerifyOptions.TrustedRootPath,
		HashAlgorithm:                hashAlgorithm,
	}

	l.Debug("verifying blob with cosign",
		"key", opts.Key,
		"signature", opts.Signature,
		"bundlePath", opts.BundlePath,
		"offline", opts.CommonVerifyOptions.Offline)

	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	if err := cmd.Exec(ctx, blobPath); err != nil {
		return err
	}

	l.Debug("blob signature verified successfully")
	return nil
}

// GetCosignArtifacts returns signatures and attestations for the given image.
func GetCosignArtifacts(image string) ([]string, error) {
	var nameOpts []name.Option

	ref, err := name.ParseReference(image, nameOpts...)
	if err != nil {
		return nil, err
	}

	var remoteOpts []ociremote.Option
	simg, _ := ociremote.SignedEntity(ref, remoteOpts...) //nolint:errcheck
	if simg == nil {
		return nil, nil
	}

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
