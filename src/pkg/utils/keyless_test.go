// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package utils

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

func writeBundleFixture(t *testing.T, certDER []byte) string {
	t.Helper()
	bundle := map[string]any{
		"verificationMaterial": map[string]any{
			"x509CertificateChain": map[string]any{
				"certificates": []map[string]any{
					{"rawBytes": base64.StdEncoding.EncodeToString(certDER)},
				},
			},
		},
	}
	data, err := json.Marshal(bundle)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "zarf.bundle.sig")
	require.NoError(t, os.WriteFile(path, data, 0o600))
	return path
}

func makeCert(t *testing.T, tmpl *x509.Certificate) []byte {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl.SerialNumber = big.NewInt(1)
	tmpl.NotBefore = time.Now()
	tmpl.NotAfter = time.Now().Add(time.Hour)
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	require.NoError(t, err)
	return der
}

func TestReadKeylessIdentityFromBundle(t *testing.T) {
	t.Parallel()

	t.Run("email SAN with V2 issuer extension", func(t *testing.T) {
		t.Parallel()
		issuerVal, err := asn1.Marshal("https://oauth2.sigstore.dev/auth")
		require.NoError(t, err)
		der := makeCert(t, &x509.Certificate{
			Subject:        pkix.Name{CommonName: "ephemeral"},
			EmailAddresses: []string{"signer@example.com"},
			ExtraExtensions: []pkix.Extension{
				{Id: sigstoreIssuerOIDV2, Value: issuerVal},
			},
		})
		path := writeBundleFixture(t, der)

		identity, issuer, err := ReadKeylessIdentityFromBundle(path)
		require.NoError(t, err)
		require.Equal(t, "signer@example.com", identity)
		require.Equal(t, "https://oauth2.sigstore.dev/auth", issuer)
	})

	t.Run("URI SAN with legacy issuer extension", func(t *testing.T) {
		t.Parallel()
		ghaURI, err := url.Parse("https://github.com/example/repo/.github/workflows/release.yml@refs/heads/main")
		require.NoError(t, err)
		der := makeCert(t, &x509.Certificate{
			Subject: pkix.Name{CommonName: "ephemeral"},
			URIs:    []*url.URL{ghaURI},
			ExtraExtensions: []pkix.Extension{
				{Id: sigstoreIssuerOIDLegacy, Value: []byte("https://token.actions.githubusercontent.com")},
			},
		})
		path := writeBundleFixture(t, der)

		identity, issuer, err := ReadKeylessIdentityFromBundle(path)
		require.NoError(t, err)
		require.Equal(t, ghaURI.String(), identity)
		require.Equal(t, "https://token.actions.githubusercontent.com", issuer)
	})

	t.Run("V2 takes precedence over legacy when both are present", func(t *testing.T) {
		t.Parallel()
		v2Val, err := asn1.Marshal("https://v2-issuer.example.com")
		require.NoError(t, err)
		der := makeCert(t, &x509.Certificate{
			Subject:        pkix.Name{CommonName: "ephemeral"},
			EmailAddresses: []string{"signer@example.com"},
			ExtraExtensions: []pkix.Extension{
				{Id: sigstoreIssuerOIDLegacy, Value: []byte("https://legacy.example.com")},
				{Id: sigstoreIssuerOIDV2, Value: v2Val},
			},
		})
		path := writeBundleFixture(t, der)

		_, issuer, err := ReadKeylessIdentityFromBundle(path)
		require.NoError(t, err)
		require.Equal(t, "https://v2-issuer.example.com", issuer)
	})

	t.Run("missing bundle file errors", func(t *testing.T) {
		t.Parallel()
		_, _, err := ReadKeylessIdentityFromBundle(filepath.Join(t.TempDir(), "nonexistent.json"))
		require.Error(t, err)
	})

	t.Run("bundle without certificate errors", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "empty.json")
		require.NoError(t, os.WriteFile(path, []byte(`{"verificationMaterial":{}}`), 0o600))
		_, _, err := ReadKeylessIdentityFromBundle(path)
		require.ErrorContains(t, err, "no certificate")
	})
}
