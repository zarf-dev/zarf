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

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

// Based off of https://github.com/dmcgowan/quicktls/blob/master/main.go

type GeneratedPKI struct {
	CA   []byte
	Cert []byte
	Key  []byte
}

// Use 2048 because we are aiming for low-resource / max-compatibility
const rsaBits = 2048
const org = "Zarf Cluster"

// 13 months is the max length allowed by browsers
const validFor = time.Hour * 24 * 375

// GeneratePKI create a CA and signed server keypair
func GeneratePKI(conf *types.TLSConfig) GeneratedPKI {

	results := GeneratedPKI{}

	ca, caKey, err := generateCA(validFor)
	if err != nil {
		message.Fatal(err, "Unable to generate the ephemeral CA")
	}

	hostCert, hostKey, err := generateCert(conf.Host, ca, caKey, validFor)
	if err != nil {
		message.Fatalf(err, "Unable to generate the cert for %s", conf.Host)
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

func AddCAToTrustStore(caFilePath string) {
	message.Info("Adding Ephemeral CA to the host root trust store")

	rhelBinary := "update-ca-trust"
	debianBinary := "update-ca-certificates"

	if utils.VerifyBinary(rhelBinary) {
		utils.CreatePathAndCopy(caFilePath, "/etc/pki/ca-trust/source/anchors/zarf-ca.crt")
		_, err := utils.ExecCommand(true, nil, rhelBinary, "extract")
		if err != nil {
			message.Error(err, "Error adding the ephemeral CA to the RHEL root trust")
		}
	} else if utils.VerifyBinary(debianBinary) {
		utils.CreatePathAndCopy(caFilePath, "/usr/local/share/ca-certificates/extra/zarf-ca.crt")
		_, err := utils.ExecCommand(true, nil, debianBinary)
		if err != nil {
			message.Error(err, "Error adding the ephemeral CA to the trust store")
		}
	}
}

// newCertificate creates a new template
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

// newPrivateKey creates a new private key
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
func generateCert(host string, ca *x509.Certificate, caKey *rsa.PrivateKey, validFor time.Duration) (*x509.Certificate, *rsa.PrivateKey, error) {
	template := newCertificate(validFor)

	template.IPAddresses = append(template.IPAddresses, net.ParseIP(config.IPV4Localhost))

	// Only use SANs to keep golang happy, https://go-review.googlesource.com/c/go/+/231379
	if ip := net.ParseIP(host); ip != nil {
		template.IPAddresses = append(template.IPAddresses, ip)
	} else {
		template.DNSNames = append(template.DNSNames, host)
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
