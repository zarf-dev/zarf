// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package signing

import (
	"fmt"

	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/fulcio/certificate"
)

// SigningMethod identifies how a Sigstore bundle was signed.
type SigningMethod string

const (
	// SigningMethodKeyless indicates a Fulcio-issued certificate bundle (OIDC/keyless).
	SigningMethodKeyless SigningMethod = "keyless"
	// SigningMethodKey indicates a public-key bundle.
	SigningMethodKey SigningMethod = "key"
)

// BundleInfo contains parsed metadata from a Sigstore bundle file.
type BundleInfo struct {
	Method           SigningMethod
	Identity         string // cert SAN — empty for key-based signatures
	Issuer           string // OIDC issuer — empty for key-based signatures
	HasTSATimestamps bool   // true if the bundle contains signed timestamps
}

// ReadBundleInfo parses a Sigstore bundle file and returns its signing metadata.
func ReadBundleInfo(bundlePath string) (BundleInfo, error) {
	b, err := bundle.LoadJSONFromPath(bundlePath)
	if err != nil {
		return BundleInfo{}, fmt.Errorf("loading bundle: %w", err)
	}

	timestamps, err := b.Timestamps()
	if err != nil {
		return BundleInfo{}, fmt.Errorf("reading bundle timestamps: %w", err)
	}

	vc, err := b.VerificationContent()
	if err != nil {
		return BundleInfo{}, fmt.Errorf("reading verification content: %w", err)
	}
	switch v := vc.(type) {
	case *bundle.Certificate:
		summary, err := certificate.SummarizeCertificate(v.Certificate())
		if err != nil {
			return BundleInfo{}, fmt.Errorf("reading certificate identity: %w", err)
		}
		return BundleInfo{Method: SigningMethodKeyless, Identity: summary.SubjectAlternativeName, Issuer: summary.Extensions.Issuer, HasTSATimestamps: len(timestamps) > 0}, nil
	case *bundle.PublicKey:
		return BundleInfo{Method: SigningMethodKey, HasTSATimestamps: len(timestamps) > 0}, nil
	default:
		return BundleInfo{}, fmt.Errorf("unrecognised verification content type %T", vc)
	}
}
