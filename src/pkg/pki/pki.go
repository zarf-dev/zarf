// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package pki provides a simple way to generate a CA and signed server keypair.
package pki

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"time"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/zarf/src/pkg/k8s"
	"github.com/defenseunicorns/zarf/src/pkg/message"
)

// Based off of https://github.com/dmcgowan/quicktls/blob/master/main.go

// Use 2048 because we are aiming for low-resource / max-compatibility.
const rsaBits = 2048
const org = "Zarf Cluster"

// 13 months is the max length allowed by browsers.
const validFor = time.Hour * 24 * 375

// GeneratePKI create a CA and signed server keypair.
func GeneratePKI(host string, dnsNames ...string) k8s.GeneratedPKI {
	results := k8s.GeneratedPKI{}

	ca, caKey, err := generateCA(validFor)
	if err != nil {
		message.Fatal(err, "Unable to generate the ephemeral CA")
	}

	hostCert, hostKey, err := generateCert(host, ca, caKey, validFor, dnsNames...)
	if err != nil {
		message.Fatalf(err, "Unable to generate the cert for %s", host)
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

	return results
}

// newCertificate creates a new template.
func newCertificate(validFor time.Duration) *x509.Certificate {
	notBefore := time.Now()
	notAfter := notBefore.Add(validFor)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		message.Fatalf(err, "failed to generate the certificate serial number")
	}

	return &x509.Certificate{
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
}

// newPrivateKey creates a new private key.
func newPrivateKey() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, rsaBits)
}

// generateCA creates a new CA certificate, saves the certificate
// and returns the x509 certificate and crypto private key. This
// private key should never be saved to disk, but rather used to
// immediately generate further certificates.
func generateCA(validFor time.Duration) (*x509.Certificate, *rsa.PrivateKey, error) {
	template := newCertificate(validFor)
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
func generateCert(host string, ca *x509.Certificate, caKey *rsa.PrivateKey, validFor time.Duration, dnsNames ...string) (*x509.Certificate, *rsa.PrivateKey, error) {
	template := newCertificate(validFor)

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
