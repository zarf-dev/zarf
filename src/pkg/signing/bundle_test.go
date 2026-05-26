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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sigstore/sigstore-go/pkg/fulcio/certificate"
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
				{Id: certificate.OIDIssuerV2, Value: issuerVal},
			},
		})
		path := writeBundleFixture(t, cert, "certificate")

		info, err := ReadBundleInfo(path)
		require.NoError(t, err)
		require.Equal(t, SigningMethodKeyless, info.Method)
		require.Equal(t, "signer@example.com", info.Identity)
		require.Equal(t, "https://github.com/login/oauth", info.Issuer)
	})

	t.Run("key-based bundle returns key method with empty identity and issuer", func(t *testing.T) {
		t.Parallel()
		path := writeKeyBasedBundleFixture(t)

		info, err := ReadBundleInfo(path)
		require.NoError(t, err)
		require.Equal(t, SigningMethodKey, info.Method)
		require.Empty(t, info.Identity)
		require.Empty(t, info.Issuer)
	})

	t.Run("missing bundle file errors", func(t *testing.T) {
		t.Parallel()
		_, err := ReadBundleInfo(filepath.Join(t.TempDir(), "nonexistent.json"))
		require.Error(t, err)
	})
}
