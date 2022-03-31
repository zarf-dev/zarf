package k8s

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/message"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DockerConfig struct {
	Auths DockerConfigEntry `json:"auths"`
}

type DockerConfigEntry map[string]DockerConfigEntryWithAuth

type DockerConfigEntryWithAuth struct {
	Auth string `json:"auth"`
}

func GetSecret(namespace, name string) (*corev1.Secret, error) {
	message.Debugf("k8s.getSecret(%s, %s)", namespace, name)
	clientSet := getClientset()
	return clientSet.CoreV1().Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

func GenerateRegistryPullCreds(namespace, name string) *corev1.Secret {
	message.Debugf("k8s.GenerateRegistryPullCreds(%s, %s)", namespace, name)

	secretDockerConfig := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				config.ZarfManagedByLabel: "zarf",
			},
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{},
	}

	// Auth field must be username:password and base64 encoded
	credential := config.GetSecret(config.StateRegistryPull)
	if credential == "" {
		message.Fatalf(nil, "Generate pull cred failed")
	}
	fieldValue := config.ZarfRegistryPullUser + ":" + credential
	authEncodedValue := base64.StdEncoding.EncodeToString([]byte(fieldValue))

	registry := config.GetRegistry()
	// Create the expected structure for the dockerconfigjson
	dockerConfigJSON := DockerConfig{
		Auths: DockerConfigEntry{
			registry: DockerConfigEntryWithAuth{
				Auth: authEncodedValue,
			},
		},
	}

	// Convert to JSON
	dockerConfigData, err := json.Marshal(dockerConfigJSON)
	if err != nil {
		message.Fatalf(err, "Unable to create the embedded registry secret")
	}

	// Add to the secret data
	secretDockerConfig.Data[".dockerconfigjson"] = dockerConfigData

	return secretDockerConfig
}

func GenerateTLSSecret(namespace, name, certPath, keyPath string) *corev1.Secret {
	message.Debugf("k8s.GenerateTLSSecret(%s, %s, %s, %s", namespace, name, certPath, keyPath)

	tlsCert, err := ioutil.ReadFile(certPath)
	if err != nil {
		message.Fatal(err, "Unable to read the TLS public certificate")
	}
	tlsKey, err := ioutil.ReadFile(keyPath)
	if err != nil {
		message.Fatal(err, "Unable to read the TLS private key")
	}
	if _, err := tls.X509KeyPair(tlsCert, tlsKey); err != nil {
		message.Fatal(err, "Unable to create the TLS keypair")
	}

	secretTLS := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				config.ZarfManagedByLabel: "zarf",
			},
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{},
	}

	secretTLS.Data[corev1.TLSCertKey] = tlsCert
	secretTLS.Data[corev1.TLSPrivateKeyKey] = tlsKey

	return secretTLS
}

func ReplaceTLSSecret(namespace, name, certPath, keyPath string) {
	message.Debugf("k8s.ReplaceTLSSecret(%s, %s)", namespace, name)

	tlsCert, err := ioutil.ReadFile(certPath)
	if err != nil {
		message.Fatalf(err, "Unable to read the TLS public certificate")
	}
	tlsKey, err := ioutil.ReadFile(keyPath)
	if err != nil {
		message.Fatalf(err, "Unable to read the TLS private key")
	}
	if _, err := tls.X509KeyPair(tlsCert, tlsKey); err != nil {
		message.Fatalf(err, "Unable to create the TLS keypair")
	}

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				config.ZarfManagedByLabel: "zarf",
			},
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{},
	}

	secret.Data[corev1.TLSCertKey] = tlsCert
	secret.Data[corev1.TLSPrivateKeyKey] = tlsKey

	if err := ReplaceSecret(secret); err != nil {
		message.Fatalf(err, "Unable to create the secret")
	}
}

func ReplaceSecret(secret *corev1.Secret) error {
	message.Debugf("k8s.ReplaceSecret(%v)", secret)

	if _, err := CreateNamespace(secret.Namespace, nil); err != nil {
		return fmt.Errorf("unable to create or read the namespace: %w", err)
	}

	if err := DeleteSecret(secret); err != nil {
		return err
	}

	return CreateSecret(secret)
}

func DeleteSecret(secret *corev1.Secret) error {
	message.Debugf("k8s.DeleteSecret(%v)", secret)
	clientSet := getClientset()

	namespaceSecrets := clientSet.CoreV1().Secrets(secret.Namespace)

	err := namespaceSecrets.Delete(context.TODO(), secret.Name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("error deleting the secret: %w", err)
	}

	return nil
}

func CreateSecret(secret *corev1.Secret) error {
	message.Debugf("k8s.CreateSecret(%v)", secret)
	clientSet := getClientset()

	namespaceSecrets := clientSet.CoreV1().Secrets(secret.Namespace)

	// create the given secret
	if _, err := namespaceSecrets.Create(context.TODO(), secret, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("unable to create the secret: %w", err)
	}

	return nil
}
