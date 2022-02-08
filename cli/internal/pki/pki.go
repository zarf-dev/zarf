package pki

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/k8s"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
	"github.com/sirupsen/logrus"
)

// Based off of https://github.com/dmcgowan/quicktls/blob/master/main.go

// Use 2048 because we are aiming for low-resource / max-compatibility
const rsaBits = 2048
const org = "Zarf Cluster"

// 13 months is the max length allowed by browsers
const validFor = time.Hour * 24 * 375

func HandlePKI() {
	pkiConfig := config.GetState().TLS
	if pkiConfig.CertPublicPath == "" || pkiConfig.CertPrivatePath == "" {
		// No certs provided, so generate them with an ephemeral CA
		GeneratePKI()
	}
}

// GeneratePKI create a CA and signed server keypair
func GeneratePKI() {
	state := config.GetState()
	directory := "zarf-pki"

	_ = utils.CreateDirectory(directory, 0700)
	caFile := filepath.Join(directory, "zarf-ca.crt")
	ca, caKey, err := generateCA(caFile, validFor)
	if err != nil {
		logrus.Fatal(err)
	}

	hostCert := filepath.Join(directory, "zarf-server.crt")
	hostKey := filepath.Join(directory, "zarf-server.key")
	if err := generateCert(state.TLS.Host, hostCert, hostKey, ca, caKey, validFor); err != nil {
		logrus.Fatal(err)
	}

	publicKeyBlock := pem.Block{
		Type:  "CERTIFICATE",
		Bytes: ca.Raw,
	}

	publicKeyPem := string(pem.EncodeToMemory(&publicKeyBlock))

	state.TLS.CertPublicPath = directory + "/zarf-server.crt"
	state.TLS.CertPrivatePath = directory + "/zarf-server.key"

	addCAToTrustStore(caFile)

	fmt.Println("Ephemeral CA below and saved to " + caFile + "\n")
	fmt.Println(publicKeyPem)

	if err := config.WriteState(state); err != nil {
		logrus.Debug(err)
		logrus.Fatal("Unable to save the zarf state file.")
	}
}

func InjectServerCert() {
	k8s.ReplaceTLSSecret("kube-system", "tls-pem")
}

func addCAToTrustStore(caFilePath string) {
	logrus.Info("Adding Ephemeral CA to the host root trust store")

	rhelBinary := "update-ca-trust"
	debianBinary := "update-ca-certificates"

	if utils.VerifyBinary(rhelBinary) {
		utils.CreatePathAndCopy(caFilePath, "/etc/pki/ca-trust/source/anchors/zarf-ca.crt")
		_, err := utils.ExecCommand(true, nil, rhelBinary, "extract")
		if err != nil {
			logrus.Debug(err)
			logrus.Warn("Error adding the ephemeral CA to the RHEL root trust")
		}
	} else if utils.VerifyBinary(debianBinary) {
		utils.CreatePathAndCopy(caFilePath, "/usr/local/share/ca-certificates/extra/zarf-ca.crt")
		_, err := utils.ExecCommand(true, nil, debianBinary)
		if err != nil {
			logrus.Debug(err)
			logrus.Warn("Error adding the ephemeral CA to the trust store")
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
		logrus.Fatalf("failed to generate serial number: %s", err)
	}

	return &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{org},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
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
func generateCA(caFile string, validFor time.Duration) (*x509.Certificate, *rsa.PrivateKey, error) {
	template := newCertificate(validFor)
	template.IsCA = true
	template.KeyUsage |= x509.KeyUsageCertSign
	template.Subject.CommonName = "Zarf Private Certificate Authority"

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

	certOut, err := os.Create(caFile)
	if err != nil {
		return nil, nil, err
	}
	defer certOut.Close()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return nil, nil, err
	}

	return ca, priv, nil
}

// generateCert generates a new certificate for the given host using the
// provided certificate authority. The cert and key files are stored in the
// the provided files.
func generateCert(host string, certFile string, keyFile string, ca *x509.Certificate, caKey *rsa.PrivateKey, validFor time.Duration) error {
	template := newCertificate(validFor)

	// Only use SANs to keep golang happy, https://go-review.googlesource.com/c/go/+/231379
	if ip := net.ParseIP(host); ip != nil {
		template.IPAddresses = append(template.IPAddresses, ip)
	} else {
		template.DNSNames = append(template.DNSNames, host)
	}

	privateKey, err := newPrivateKey()
	if err != nil {
		return err
	}

	return generateFromTemplate(certFile, keyFile, template, ca, privateKey, caKey)
}

// generateFromTemplate generates a certificate from the given template and signed by
// the given parent, storing the results in a certificate and key file.
func generateFromTemplate(certFile, keyFile string, template, parent *x509.Certificate, key *rsa.PrivateKey, parentKey *rsa.PrivateKey) error {
	derBytes, err := x509.CreateCertificate(rand.Reader, template, parent, key.Public(), parentKey)
	if err != nil {
		return err
	}

	certOut, err := os.Create(certFile)
	if err != nil {
		return err
	}
	err = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	if err != nil {
		return err
	}
	certOut.Close()

	return savePrivateKey(key, keyFile)
}

// savePrivateKey saves the private key to a PEM file
func savePrivateKey(key *rsa.PrivateKey, keyFile string) error {
	keyOut, err := os.OpenFile(keyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer keyOut.Close()

	keyBytes := x509.MarshalPKCS1PrivateKey(key)
	err = pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyBytes})
	if err != nil {
		return err
	}

	return nil
}
