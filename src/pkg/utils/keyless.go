// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package utils

import (
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// Sigstore custom OIDs for the OIDC issuer claim embedded in Fulcio certs.
//   - sigstoreIssuerOIDLegacy: raw string in extension value
//   - sigstoreIssuerOIDV2: DER-encoded UTF8String, used by Fulcio v1+
var (
	sigstoreIssuerOIDLegacy = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 1}
	sigstoreIssuerOIDV2     = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 8}
)

// ReadKeylessIdentityFromBundle parses a Sigstore bundle file and returns the
// signer identity (cert SAN) and OIDC issuer claim. Used at sign time so users
// learn the identity their keyless flow resolved to.
func ReadKeylessIdentityFromBundle(bundlePath string) (identity, issuer string, err error) {
	data, err := os.ReadFile(bundlePath)
	if err != nil {
		return "", "", err
	}

	// Sigstore bundle VerificationMaterial is a oneof: newer keyless bundles use
	// "certificate" (single Fulcio cert), legacy/chain variants use
	// "x509CertificateChain.certificates[]". Try both.
	var b struct {
		VerificationMaterial struct {
			Certificate struct {
				RawBytes string `json:"rawBytes"`
			} `json:"certificate"`
			X509CertificateChain struct {
				Certificates []struct {
					RawBytes string `json:"rawBytes"`
				} `json:"certificates"`
			} `json:"x509CertificateChain"`
		} `json:"verificationMaterial"`
	}
	if err := json.Unmarshal(data, &b); err != nil {
		return "", "", fmt.Errorf("parsing bundle JSON: %w", err)
	}

	rawBytes := b.VerificationMaterial.Certificate.RawBytes
	if rawBytes == "" && len(b.VerificationMaterial.X509CertificateChain.Certificates) > 0 {
		rawBytes = b.VerificationMaterial.X509CertificateChain.Certificates[0].RawBytes
	}
	if rawBytes == "" {
		return "", "", errors.New("bundle contains no certificate")
	}

	der, err := base64.StdEncoding.DecodeString(rawBytes)
	if err != nil {
		return "", "", fmt.Errorf("decoding cert: %w", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return "", "", fmt.Errorf("parsing cert: %w", err)
	}

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
				return identity, issuer, nil
			}
		case ext.Id.Equal(sigstoreIssuerOIDLegacy) && issuer == "":
			issuer = string(ext.Value)
		}
	}

	return identity, issuer, nil
}
