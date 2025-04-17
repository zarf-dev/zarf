// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package pki provides a simple way to generate a CA and signed server keypair.
package pki

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/pkg/logger"
)

// Based off of https://github.com/dmcgowan/quicktls/blob/master/main.go

// Use 2048 because we are aiming for low-resource / max-compatibility.
const rsaBits = 2048
const org = "Zarf Cluster"

// 13 months is the max length allowed by browsers.
const validFor = time.Hour * 24 * 375

// provides a simple way to mock time.Now() in tests
var now = time.Now

// GeneratedPKI is a struct for storing generated PKI data.
type GeneratedPKI struct {
	CA   []byte `json:"ca"`
	Cert []byte `json:"cert"`
	Key  []byte `json:"key"`
}

// GeneratePKI create a CA and signed server keypair.
func GeneratePKI(host string, dnsNames ...string) (GeneratedPKI, error) {
	notAfter := now().Add(validFor)
	return generatePKI(host, notAfter, dnsNames...)
}

func generatePKI(host string, notAfter time.Time, dnsNames ...string) (GeneratedPKI, error) {
	results := GeneratedPKI{}
	ca, caKey, err := generateCA(notAfter)
	if err != nil {
		return GeneratedPKI{}, fmt.Errorf("unable to generate the ephemeral CA: %w", err)
	}
	hostCert, hostKey, err := generateCert(host, ca, caKey, notAfter, dnsNames...)
	if err != nil {
		return GeneratedPKI{}, fmt.Errorf("unable to generate the cert for %s: %w", host, err)
	}
	results.CA = pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: ca.Raw,
	})
	results.Cert = pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: hostCert.Raw,
	})
	results.Key = pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(hostKey),
	})
	return results, nil
}

// newCertificate creates a new template.
func newCertificate(notAfter time.Time) (*x509.Certificate, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to generate the certificate serial number: %w", err)
	}
	notBefore := now()
	cert := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{org},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	return cert, nil
}

// newPrivateKey creates a new private key.
func newPrivateKey() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, rsaBits)
}

// generateCA creates a new CA certificate, saves the certificate
// and returns the x509 certificate and crypto private key. This
// private key should never be saved to disk, but rather used to
// immediately generate further certificates.
func generateCA(notAfter time.Time) (*x509.Certificate, *rsa.PrivateKey, error) {
	template, err := newCertificate(notAfter)
	if err != nil {
		return nil, nil, err
	}
	template.IsCA = true
	template.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature
	template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
	template.Subject.CommonName = "ca.private.zarf.dev"
	template.Subject.Organization = []string{"Zarf Community"}

	priv, err := newPrivateKey()
	if err != nil {
		return nil, nil, err
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, priv.Public(), priv)
	if err != nil {
		return nil, nil, err
	}

	ca, err := x509.ParseCertificate(derBytes)
	if err != nil {
		return nil, nil, err
	}

	return ca, priv, nil
}

// generateCert generates a new certificate for the given host using the
// provided certificate authority. The cert and key files are stored in
// the provided files.
func generateCert(host string, ca *x509.Certificate, caKey *rsa.PrivateKey, notAfter time.Time, dnsNames ...string) (*x509.Certificate, *rsa.PrivateKey, error) {
	template, err := newCertificate(notAfter)
	if err != nil {
		return nil, nil, err
	}

	template.IPAddresses = append(template.IPAddresses, net.ParseIP(helpers.IPV4Localhost))

	// Only use SANs to keep golang happy, https://go-review.googlesource.com/c/go/+/231379
	if ip := net.ParseIP(host); ip != nil {
		template.IPAddresses = append(template.IPAddresses, ip)
	} else {
		template.DNSNames = append(template.DNSNames, host)
		template.DNSNames = append(template.DNSNames, dnsNames...)
	}

	template.Subject.CommonName = host

	privateKey, err := newPrivateKey()
	if err != nil {
		return nil, nil, err
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, ca, privateKey.Public(), caKey)
	if err != nil {
		return nil, nil, err
	}

	cert, err := x509.ParseCertificate(derBytes)
	if err != nil {
		return nil, nil, err
	}

	return cert, privateKey, nil
}

// CheckForExpiredCert checks if the certificate is expired
func CheckForExpiredCert(ctx context.Context, pk GeneratedPKI) error {
	block, _ := pem.Decode(pk.Cert)
	if block == nil {
		return fmt.Errorf("failed to decode pem data")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	if cert.NotAfter.Before(now()) {
		return fmt.Errorf("the Zarf agent certificate is expired as of %s, run `zarf tool update-creds agent` to update", cert.NotAfter)
	}

	remainingTime := cert.NotAfter.Sub(now())
	totalTime := cert.NotAfter.Sub(cert.NotBefore)
	certHas20PercentRemainingTime := (float64(remainingTime) / float64(totalTime)) > 0.2

	if !certHas20PercentRemainingTime {
		logger.From(ctx).Warn("the Zarf agent certificate is expiring soon, run `zarf tools update-creds` to update", "expiration", cert.NotAfter)
	}
	return nil
}
