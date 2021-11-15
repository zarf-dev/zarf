package k8s

import (
	"context"
	"crypto/tls"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ReplaceTLSSecret(namespace string, name string, certPath string, keyPath string) {

	clientSet := getClientset()
	logContext := logrus.WithFields(logrus.Fields{
		"Namespace": namespace,
		"Name":      name,
		"Cert":      certPath,
	})
	namespaceSecrets := clientSet.CoreV1().Secrets(namespace)

	logContext.Info("Loading secret")

	err := namespaceSecrets.Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		logContext.Debug(err)
		logContext.Warn("Error deleting the secret")
	}

	tlsCert, err := readFile(certPath)
	if err != nil {
		logContext.Debug(err)
		logContext.Fatal("Unable to read the TLS public certificate")
	}
	tlsKey, err := readFile(keyPath)
	if err != nil {
		logContext.Debug(err)
		logContext.Fatal("Unable to read the TLS private key")
	}
	if _, err := tls.X509KeyPair(tlsCert, tlsKey); err != nil {
		logContext.Debug(err)
		logContext.Fatal("Unable to create the TLS keypair")
	}

	secretTLS := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{},
	}

	secretTLS.Data[corev1.TLSCertKey] = []byte(tlsCert)
	secretTLS.Data[corev1.TLSPrivateKeyKey] = []byte(tlsKey)

	_, err = namespaceSecrets.Create(context.TODO(), secretTLS, metav1.CreateOptions{})
	if err != nil {
		logContext.Debug(err)
		logContext.Fatal("Unable to create the secret", err)
	}
}
