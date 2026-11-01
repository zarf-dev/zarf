// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package signing

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/signature"
)

// PackageSigner signs Zarf packages with cosign.
type PackageSigner struct {
	opts SignBlobOptions
}

// NewPackageSigner returns a signature.Signer that signs packages with opts.
func NewPackageSigner(opts SignBlobOptions) *PackageSigner {
	return &PackageSigner{opts: opts}
}

// SignPackage signs req.ZarfYAML and writes the bundle to req.Bundle.
func (s *PackageSigner) SignPackage(ctx context.Context, req signature.SignRequest) error {
	l := logger.From(ctx)

	// cosign v3.1.1+ writes only the bundle when NewBundleFormat=true.
	opts := s.opts
	opts.NewBundleFormat = true
	opts.BundlePath = req.Destination
	if err := opts.CheckOverwrite(ctx); err != nil {
		return err
	}
	opts.BundlePath = req.Bundle

	if _, err := CosignSignBlobWithOptions(ctx, req.ZarfYAML, opts); err != nil {
		return err
	}

	if info, err := ReadBundleInfo(req.Bundle); err == nil {
		if info.Identity != "" {
			l.Info("keyless signed package", "identity", info.Identity, "issuer", info.Issuer)
		}
	} else {
		l.Debug("could not read bundle info after signing", "error", err)
	}
	return nil
}

// PackageVerifier verifies Zarf package signatures with cosign.
type PackageVerifier struct {
	opts VerifyBlobOptions
}

// NewPackageVerifier returns a signature.Verifier that verifies packages with opts.
func NewPackageVerifier(opts VerifyBlobOptions) *PackageVerifier {
	return &PackageVerifier{opts: opts}
}

// VerifyPackage verifies the bundle, or the legacy signature when there is no bundle.
func (v *PackageVerifier) VerifyPackage(ctx context.Context, files signature.Files) error {
	l := logger.From(ctx)
	opts := v.opts

	// Sync the deprecated KeyRef alias before computing hasKey so callers using
	// only KeyRef are not rejected for missing material. CosignVerifyBlobWithOptions
	// emits the deprecation warning when invoked.
	if opts.Key == "" && opts.KeyRef != "" {
		opts.Key = opts.KeyRef
	}

	hasKey := opts.Key != ""
	hasKeylessIdentity := opts.CertVerify.CertIdentity != "" || opts.CertVerify.CertIdentityRegexp != ""
	hasCert := opts.CertVerify.Cert != ""
	hasVerificationMaterial := hasKey || hasKeylessIdentity || hasCert

	// Handle the case where the package is not signed
	if !files.Signed {
		if hasVerificationMaterial {
			// Providing material implies expecting a signature — always fatal.
			return errors.New("verification material was provided but the package is not signed")
		}
		return fmt.Errorf("package is not signed - verification cannot be performed: %w", signature.ErrNoVerificationMaterial)
	}

	// Check for bundle format signature (preferred). Parse it once for both method
	// detection (fast-fail below) and the verify path.
	bundleInfo, bundleErr := ReadBundleInfo(files.Bundle)
	hasBundleInfo := bundleErr == nil

	// Early validation: fail fast with a method-specific message before cosign emits a generic error.
	if hasBundleInfo {
		switch bundleInfo.Method {
		case SigningMethodKeyless:
			if !hasKeylessIdentity && !hasCert {
				return fmt.Errorf("package was signed with keyless method; provide --certificate-identity + --certificate-oidc-issuer to verify: %w", signature.ErrNoVerificationMaterial)
			}
		case SigningMethodKey:
			if !hasKey && !hasCert {
				return fmt.Errorf("package was signed with a key; provide --key to verify: %w", signature.ErrNoVerificationMaterial)
			}
		}
	}

	if !hasVerificationMaterial {
		return fmt.Errorf("package is signed but no verification material was provided (--key, --certificate-identity + --certificate-oidc-issuer): %w", signature.ErrNoVerificationMaterial)
	}

	// Preserve a caller-provided staging directory while retaining Zarf's default.
	if opts.TempDir == "" {
		opts.TempDir = config.CommonOptions.TempDirectory
	}

	if hasBundleInfo {
		opts.BundlePath = files.Bundle
		// Auto-enable UseSignedTimestamps when the bundle contains timestamps.
		// The bundle was signed with a TSA; using those timestamps is required to
		// verify the signature after the short-lived Fulcio cert expires.
		if bundleInfo.HasTSATimestamps && !opts.CommonVerifyOptions.UseSignedTimestamps {
			l.Debug("bundle contains TSA timestamps; enabling signed-timestamp verification automatically")
			opts.CommonVerifyOptions.UseSignedTimestamps = true
		}
		return CosignVerifyBlobWithOptions(ctx, files.ZarfYAML, opts)
	}
	if !errors.Is(bundleErr, os.ErrNotExist) {
		return fmt.Errorf("error checking bundle signature: %w", bundleErr)
	}

	// Bundle doesn't exist, check for legacy signature format
	_, sigStatErr := os.Stat(files.LegacySignature)
	if sigStatErr != nil {
		if errors.Is(sigStatErr, os.ErrNotExist) {
			return fmt.Errorf("signature not found: neither bundle nor legacy signature exists")
		}
		return fmt.Errorf("error checking legacy signature: %w", sigStatErr)
	}

	// Legacy signatures don't carry a certificate chain, so keyless identity
	// verification has nothing to match against. Fail fast with a clear message
	// rather than letting cosign emit a generic key/cert/bundle error.
	if hasKeylessIdentity {
		return errors.New("keyless verification requires bundle-format signatures, but this package has only a legacy .sig. Ask the publisher to re-sign with bundle format, or verify with --key")
	}

	// Legacy signature found
	l.Warn("bundle format signature not found: legacy signature is being deprecated.")
	opts.Signature = files.LegacySignature

	opts.CommonVerifyOptions.NewBundleFormat = false
	return CosignVerifyBlobWithOptions(ctx, files.ZarfYAML, opts)
}
