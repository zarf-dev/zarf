// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package signing

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// makeCert creates a self-signed test certificate from the given template.
func makeCert(t *testing.T, tmpl *x509.Certificate) *x509.Certificate {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl.SerialNumber = big.NewInt(1)
	tmpl.NotBefore = time.Now()
	tmpl.NotAfter = time.Now().Add(time.Hour)
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return cert
}

// writeBundleFixture writes a minimal sigstore-go-valid bundle containing cert.
// shape: "x509CertificateChain" or "certificate".
func writeBundleFixture(t *testing.T, cert *x509.Certificate, shape string) string {
	t.Helper()
	var verMaterial map[string]any
	switch shape {
	case "certificate":
		verMaterial = map[string]any{
			"certificate": map[string]any{
				"rawBytes": base64.StdEncoding.EncodeToString(cert.Raw),
			},
		}
	case "x509CertificateChain":
		verMaterial = map[string]any{
			"x509CertificateChain": map[string]any{
				"certificates": []map[string]any{
					{"rawBytes": base64.StdEncoding.EncodeToString(cert.Raw)},
				},
			},
		}
	default:
		t.Fatalf("unknown bundle shape: %s", shape)
	}
	b := map[string]any{
		"mediaType":            "application/vnd.dev.sigstore.bundle+json;version=0.1",
		"verificationMaterial": verMaterial,
		"messageSignature": map[string]any{
			"messageDigest": map[string]any{
				"algorithm": "SHA2_256",
				"digest":    base64.StdEncoding.EncodeToString(make([]byte, 32)),
			},
			"signature": base64.StdEncoding.EncodeToString([]byte("dummy")),
		},
	}
	data, err := json.Marshal(b)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "zarf.bundle.json")
	require.NoError(t, os.WriteFile(path, data, 0o600))
	return path
}

// writeKeyBasedBundleFixture writes a minimal sigstore-go-valid bundle using a
// public key hint (not a certificate), representing a key-based signature.
func writeKeyBasedBundleFixture(t *testing.T) string {
	t.Helper()
	b := map[string]any{
		"mediaType": "application/vnd.dev.sigstore.bundle+json;version=0.1",
		"verificationMaterial": map[string]any{
			"publicKey": map[string]any{"hint": "test-key-id"},
		},
		"messageSignature": map[string]any{
			"messageDigest": map[string]any{
				"algorithm": "SHA2_256",
				"digest":    base64.StdEncoding.EncodeToString(make([]byte, 32)),
			},
			"signature": base64.StdEncoding.EncodeToString([]byte("dummy")),
		},
	}
	data, err := json.Marshal(b)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "zarf.bundle.json")
	require.NoError(t, os.WriteFile(path, data, 0o600))
	return path
}

func TestExtractIdentityFromCert(t *testing.T) {
	t.Parallel()

	t.Run("email SAN with V2 issuer OID", func(t *testing.T) {
		t.Parallel()
		issuerVal, err := asn1.Marshal("https://oauth2.sigstore.dev/auth")
		require.NoError(t, err)
		cert := makeCert(t, &x509.Certificate{
			Subject:        pkix.Name{CommonName: "ephemeral"},
			EmailAddresses: []string{"signer@example.com"},
			ExtraExtensions: []pkix.Extension{
				{Id: sigstoreIssuerOIDV2, Value: issuerVal},
			},
		})

		identity, issuer := extractIdentityFromCert(cert)
		require.Equal(t, "signer@example.com", identity)
		require.Equal(t, "https://oauth2.sigstore.dev/auth", issuer)
	})

	t.Run("URI SAN with legacy issuer OID", func(t *testing.T) {
		t.Parallel()
		ghaURI, err := url.Parse("https://github.com/example/repo/.github/workflows/release.yml@refs/heads/main")
		require.NoError(t, err)
		cert := makeCert(t, &x509.Certificate{
			Subject: pkix.Name{CommonName: "ephemeral"},
			URIs:    []*url.URL{ghaURI},
			ExtraExtensions: []pkix.Extension{
				{Id: sigstoreIssuerOIDLegacy, Value: []byte("https://token.actions.githubusercontent.com")},
			},
		})

		identity, issuer := extractIdentityFromCert(cert)
		require.Equal(t, ghaURI.String(), identity)
		require.Equal(t, "https://token.actions.githubusercontent.com", issuer)
	})

	t.Run("V2 OID takes precedence over legacy when both present", func(t *testing.T) {
		t.Parallel()
		v2Val, err := asn1.Marshal("https://v2-issuer.example.com")
		require.NoError(t, err)
		cert := makeCert(t, &x509.Certificate{
			Subject:        pkix.Name{CommonName: "ephemeral"},
			EmailAddresses: []string{"signer@example.com"},
			ExtraExtensions: []pkix.Extension{
				{Id: sigstoreIssuerOIDLegacy, Value: []byte("https://legacy.example.com")},
				{Id: sigstoreIssuerOIDV2, Value: v2Val},
			},
		})

		_, issuer := extractIdentityFromCert(cert)
		require.Equal(t, "https://v2-issuer.example.com", issuer)
	})

	t.Run("DNS SAN used when no email or URI", func(t *testing.T) {
		t.Parallel()
		cert := makeCert(t, &x509.Certificate{
			Subject:  pkix.Name{CommonName: "ephemeral"},
			DNSNames: []string{"host.example.com"},
		})

		identity, _ := extractIdentityFromCert(cert)
		require.Equal(t, "host.example.com", identity)
	})
}

func TestReadBundleInfo(t *testing.T) {
	t.Parallel()

	t.Run("certificate bundle returns keyless method with identity and issuer", func(t *testing.T) {
		t.Parallel()
		issuerVal, err := asn1.Marshal("https://github.com/login/oauth")
		require.NoError(t, err)
		cert := makeCert(t, &x509.Certificate{
			Subject:        pkix.Name{CommonName: "ephemeral"},
			EmailAddresses: []string{"signer@example.com"},
			ExtraExtensions: []pkix.Extension{
				{Id: sigstoreIssuerOIDV2, Value: issuerVal},
			},
		})
		path := writeBundleFixture(t, cert, "certificate")

		info, err := ReadBundleInfo(path)
		require.NoError(t, err)
		require.Equal(t, "keyless", info.Method)
		require.Equal(t, "signer@example.com", info.Identity)
		require.Equal(t, "https://github.com/login/oauth", info.Issuer)
	})

	t.Run("key-based bundle returns key method with empty identity and issuer", func(t *testing.T) {
		t.Parallel()
		path := writeKeyBasedBundleFixture(t)

		info, err := ReadBundleInfo(path)
		require.NoError(t, err)
		require.Equal(t, "key", info.Method)
		require.Empty(t, info.Identity)
		require.Empty(t, info.Issuer)
	})

	t.Run("missing bundle file errors", func(t *testing.T) {
		t.Parallel()
		_, err := ReadBundleInfo(filepath.Join(t.TempDir(), "nonexistent.json"))
		require.Error(t, err)
	})
}

func TestReadKeylessIdentityFromBundle(t *testing.T) {
	t.Parallel()

	t.Run("x509CertificateChain bundle returns identity and issuer", func(t *testing.T) {
		t.Parallel()
		issuerVal, err := asn1.Marshal("https://token.actions.githubusercontent.com")
		require.NoError(t, err)
		ghaURI, err := url.Parse("https://github.com/example/repo/.github/workflows/release.yml@refs/heads/main")
		require.NoError(t, err)
		cert := makeCert(t, &x509.Certificate{
			Subject: pkix.Name{CommonName: "ephemeral"},
			URIs:    []*url.URL{ghaURI},
			ExtraExtensions: []pkix.Extension{
				{Id: sigstoreIssuerOIDV2, Value: issuerVal},
			},
		})
		path := writeBundleFixture(t, cert, "x509CertificateChain")

		identity, issuer, err := ReadKeylessIdentityFromBundle(path)
		require.NoError(t, err)
		require.Equal(t, ghaURI.String(), identity)
		require.Equal(t, "https://token.actions.githubusercontent.com", issuer)
	})

	t.Run("certificate bundle returns identity and issuer", func(t *testing.T) {
		t.Parallel()
		issuerVal, err := asn1.Marshal("https://github.com/login/oauth")
		require.NoError(t, err)
		cert := makeCert(t, &x509.Certificate{
			Subject:        pkix.Name{CommonName: "ephemeral"},
			EmailAddresses: []string{"signer@example.com"},
			ExtraExtensions: []pkix.Extension{
				{Id: sigstoreIssuerOIDV2, Value: issuerVal},
			},
		})
		path := writeBundleFixture(t, cert, "certificate")

		identity, issuer, err := ReadKeylessIdentityFromBundle(path)
		require.NoError(t, err)
		require.Equal(t, "signer@example.com", identity)
		require.Equal(t, "https://github.com/login/oauth", issuer)
	})

	t.Run("missing bundle file errors", func(t *testing.T) {
		t.Parallel()
		_, _, err := ReadKeylessIdentityFromBundle(filepath.Join(t.TempDir(), "nonexistent.json"))
		require.Error(t, err)
	})

	t.Run("key-based bundle errors", func(t *testing.T) {
		t.Parallel()
		path := writeKeyBasedBundleFixture(t)
		_, _, err := ReadKeylessIdentityFromBundle(path)
		require.ErrorContains(t, err, "not a keyless signature")
	})
}
