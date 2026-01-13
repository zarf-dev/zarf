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
	hostCert, hostKey, err := generateServerCert(host, ca, caKey, notAfter, dnsNames...)

	if err != nil {
		return GeneratedPKI{}, fmt.Errorf("unable to generate the cert for %s: %w", host, err)
	}
	results.CA = encodeCertToPEM(ca)
	results.Cert = encodeCertToPEM(hostCert)
	results.Key = encodeKeyToPEM(hostKey)
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

// encodeCertToPEM encodes a certificate to PEM format
func encodeCertToPEM(cert *x509.Certificate) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	})
}

// encodeKeyToPEM encodes an RSA private key to PEM format
func encodeKeyToPEM(key *rsa.PrivateKey) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
}

// parseCAFromPEM parses CA certificate and private key from PEM data
func parseCAFromPEM(caCertPEM, caKeyPEM []byte) (*x509.Certificate, *rsa.PrivateKey, error) {
	// Parse CA certificate
	caBlock, _ := pem.Decode(caCertPEM)
	if caBlock == nil {
		return nil, nil, fmt.Errorf("failed to decode CA certificate")
	}
	caCert, err := x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	// Parse CA private key
	caKeyBlock, _ := pem.Decode(caKeyPEM)
	if caKeyBlock == nil {
		return nil, nil, fmt.Errorf("failed to decode CA private key")
	}
	caKey, err := x509.ParsePKCS1PrivateKey(caKeyBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse CA private key: %w", err)
	}

	return caCert, caKey, nil
}

// createAndSignCertificate creates and signs a certificate with the given template
func createAndSignCertificate(template *x509.Certificate, ca *x509.Certificate, caKey *rsa.PrivateKey) (*x509.Certificate, *rsa.PrivateKey, error) {
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

// GenerateCA creates a CA certificate and returns the PEM-encoded certificate and private key
func GenerateCA(subject string) ([]byte, []byte, error) {
	notAfter := now().Add(validFor)
	ca, caKey, err := generateCA(notAfter)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to generate CA: %w", err)
	}

	ca.Subject.CommonName = subject
	return encodeCertToPEM(ca), encodeKeyToPEM(caKey), nil
}

// GenerateServerCert creates a server certificate signed by the provided CA
func GenerateServerCert(caCertPEM, caKeyPEM []byte, commonName string, dnsNames []string) ([]byte, []byte, error) {
	caCert, caKey, err := parseCAFromPEM(caCertPEM, caKeyPEM)
	if err != nil {
		return nil, nil, err
	}

	notAfter := now().Add(validFor)
	serverCert, serverKey, err := generateServerCert(commonName, caCert, caKey, notAfter, dnsNames...)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to generate server certificate: %w", err)
	}

	return encodeCertToPEM(serverCert), encodeKeyToPEM(serverKey), nil
}

// GenerateClientCert creates a client certificate signed by the provided CA
func GenerateClientCert(caCertPEM, caKeyPEM []byte, commonName string) ([]byte, []byte, error) {
	caCert, caKey, err := parseCAFromPEM(caCertPEM, caKeyPEM)
	if err != nil {
		return nil, nil, err
	}

	notAfter := now().Add(validFor)
	clientCert, clientKey, err := generateClientCert(commonName, caCert, caKey, notAfter)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to generate client certificate: %w", err)
	}

	return encodeCertToPEM(clientCert), encodeKeyToPEM(clientKey), nil
}

// CertType defines the type of certificate to generate
type CertType int

// The different types of Certs that can be generated
const (
	CertTypeServer CertType = iota
	CertTypeClient
)

// generateTypedCert generates a certificate with the specified type and configuration
func generateTypedCert(certType CertType, commonName string, ca *x509.Certificate, caKey *rsa.PrivateKey, notAfter time.Time, dnsNames ...string) (*x509.Certificate, *rsa.PrivateKey, error) {
	template, err := newCertificate(notAfter)
	if err != nil {
		return nil, nil, err
	}

	template.Subject.CommonName = commonName

	switch certType {
	case CertTypeServer:
		template.IPAddresses = append(template.IPAddresses, net.ParseIP(helpers.IPV4Localhost))
		template.IPAddresses = append(template.IPAddresses, net.ParseIP("::1"))

		// Only use SANs to keep golang happy
		if ip := net.ParseIP(commonName); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, commonName)
			template.DNSNames = append(template.DNSNames, dnsNames...)
		}
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}

	case CertTypeClient:
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	}

	return createAndSignCertificate(template, ca, caKey)
}

// generateServerCert generates a server certificate with server auth extended key usage
func generateServerCert(host string, ca *x509.Certificate, caKey *rsa.PrivateKey, notAfter time.Time, dnsNames ...string) (*x509.Certificate, *rsa.PrivateKey, error) {
	return generateTypedCert(CertTypeServer, host, ca, caKey, notAfter, dnsNames...)
}

// generateClientCert generates a client certificate with client auth extended key usage
func generateClientCert(commonName string, ca *x509.Certificate, caKey *rsa.PrivateKey, notAfter time.Time) (*x509.Certificate, *rsa.PrivateKey, error) {
	return generateTypedCert(CertTypeClient, commonName, ca, caKey, notAfter)
}

// parseCertFromPEM parses a certificate from PEM data
func parseCertFromPEM(certData []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(certData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode pem data")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return cert, nil
}

// GetRemainingCertLifePercentage gives back the percentage of the given certificates total lifespan that it has left before it's expired
func GetRemainingCertLifePercentage(certData []byte) (float64, error) {
	cert, err := parseCertFromPEM(certData)
	if err != nil {
		return 0, err
	}

	currentTime := now()
	totalLifetime := cert.NotAfter.Sub(cert.NotBefore)
	remainingTime := cert.NotAfter.Sub(currentTime)

	// If certificate is expired, return 0
	if remainingTime <= 0 {
		return 0, nil
	}

	percentage := (float64(remainingTime) / float64(totalLifetime)) * 100
	return percentage, nil
}

// CheckForExpiredCert checks if the certificate is expired
func CheckForExpiredCert(ctx context.Context, pk GeneratedPKI) error {
	cert, err := parseCertFromPEM(pk.Cert)
	if err != nil {
		return err
	}

	if cert.NotAfter.Before(now()) {
		return fmt.Errorf("the Zarf agent certificate is expired as of %s, run `zarf tools update-creds agent` to update", cert.NotAfter)
	}

	remainingLife, err := GetRemainingCertLifePercentage(pk.Cert)
	if err != nil {
		return err
	}

	if remainingLife < 20 {
		logger.From(ctx).Warn("the Zarf agent certificate is expiring soon, run `zarf tools update-creds agent` to update", "expiration", cert.NotAfter)
	}
	return nil
}

// GenerateMTLSCerts generates a complete set of mTLS certificates including CA, server cert, and client cert.
// Returns two GeneratedPKI structs: one for the server (containing server cert, key, and CA) and one for the client (containing client cert, key, and CA).
func GenerateMTLSCerts(serverDNSNames []string, serverCommonName string, clientCommonName string) (server GeneratedPKI, client GeneratedPKI, err error) {
	caCert, caKey, err := GenerateCA("Zarf Registry CA")
	if err != nil {
		return GeneratedPKI{}, GeneratedPKI{}, fmt.Errorf("failed to generate CA certificate: %w", err)
	}

	serverCert, serverKey, err := GenerateServerCert(caCert, caKey, serverCommonName, serverDNSNames)
	if err != nil {
		return GeneratedPKI{}, GeneratedPKI{}, fmt.Errorf("failed to generate server certificate: %w", err)
	}

	clientCert, clientKey, err := GenerateClientCert(caCert, caKey, clientCommonName)
	if err != nil {
		return GeneratedPKI{}, GeneratedPKI{}, fmt.Errorf("failed to generate client certificate: %w", err)
	}

	serverPKI := GeneratedPKI{
		CA:   caCert,
		Cert: serverCert,
		Key:  serverKey,
	}

	clientPKI := GeneratedPKI{
		CA:   caCert,
		Cert: clientCert,
		Key:  clientKey,
	}

	return serverPKI, clientPKI, nil
}
