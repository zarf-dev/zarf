// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic utility functions.
package utils

import (
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"fmt"

	"github.com/sigstore/sigstore-go/pkg/bundle"
)

// Sigstore custom OIDs for the OIDC issuer claim embedded in Fulcio certs.
//   - sigstoreIssuerOIDLegacy: raw string in extension value
//   - sigstoreIssuerOIDV2: DER-encoded UTF8String, used by Fulcio v1+
var (
	sigstoreIssuerOIDLegacy = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 1}
	sigstoreIssuerOIDV2     = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 8}
)

// extractIdentityFromCert extracts the signer identity (cert SAN) and OIDC issuer
// from a Fulcio-issued X.509 certificate using Sigstore OID extensions.
// SAN priority: email > URI > DNS. OID priority: V2 > legacy.
func extractIdentityFromCert(cert *x509.Certificate) (identity, issuer string) {
	switch {
	case len(cert.EmailAddresses) > 0:
		identity = cert.EmailAddresses[0]
	case len(cert.URIs) > 0:
		identity = cert.URIs[0].String()
	case len(cert.DNSNames) > 0:
		identity = cert.DNSNames[0]
	}

	for _, ext := range cert.Extensions {
		switch {
		case ext.Id.Equal(sigstoreIssuerOIDV2):
			var s string
			if _, decErr := asn1.Unmarshal(ext.Value, &s); decErr == nil {
				issuer = s
				return identity, issuer
			}
		case ext.Id.Equal(sigstoreIssuerOIDLegacy) && issuer == "":
			issuer = string(ext.Value)
		}
	}

	return identity, issuer
}

// ReadKeylessIdentityFromBundle parses a Sigstore bundle file and returns the
// signer identity (cert SAN) and OIDC issuer claim. Returns an error if the
// bundle does not contain a certificate (i.e. is not a keyless signature).
func ReadKeylessIdentityFromBundle(bundlePath string) (identity, issuer string, err error) {
	b, err := bundle.LoadJSONFromPath(bundlePath)
	if err != nil {
		return "", "", fmt.Errorf("loading bundle: %w", err)
	}

	vc, err := b.VerificationContent()
	if err != nil {
		return "", "", fmt.Errorf("reading verification content: %w", err)
	}

	certHolder, ok := vc.(*bundle.Certificate)
	if !ok {
		return "", "", errors.New("bundle does not contain a certificate (not a keyless signature)")
	}

	identity, issuer = extractIdentityFromCert(certHolder.Certificate())
	return identity, issuer, nil
}
