package k8s

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/types"
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
	clientset, err := getClientset()
	if err != nil {
		return nil, err
	}

	return clientset.CoreV1().Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

func GetSecretsWithLabel(namespace, labelSelector string) (*corev1.SecretList, error) {
	message.Debugf("k8s.getSecretsWithLabel(%s, %s)", namespace, labelSelector)
	clientset, err := getClientset()
	if err != nil {
		return nil, err
	}

	listOptions := metav1.ListOptions{LabelSelector: labelSelector}
	return clientset.CoreV1().Secrets(namespace).List(context.TODO(), listOptions)
}

func GenerateSecret(namespace, name string, secretType corev1.SecretType) *corev1.Secret {
	message.Debugf("k8s.GenerateSecret(%s, %s)", namespace, name)

	return &corev1.Secret{
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
		Type: secretType,
		Data: map[string][]byte{},
	}
}

func GenerateRegistryPullCreds(namespace, name string) *corev1.Secret {
	message.Debugf("k8s.GenerateRegistryPullCreds(%s, %s)", namespace, name)

	secretDockerConfig := GenerateSecret(namespace, name, corev1.SecretTypeDockerConfigJson)

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

func GenerateTLSSecret(namespace, name string, conf types.GeneratedPKI) (*corev1.Secret, error) {
	message.Debugf("k8s.GenerateTLSSecret(%s, %s, %s)", namespace, name, message.JsonValue(conf))

	if _, err := tls.X509KeyPair(conf.Cert, conf.Key); err != nil {
		return nil, err
	}

	secretTLS := GenerateSecret(namespace, name, corev1.SecretTypeTLS)
	secretTLS.Data[corev1.TLSCertKey] = conf.Cert
	secretTLS.Data[corev1.TLSPrivateKeyKey] = conf.Key

	return secretTLS, nil
}

func ReplaceTLSSecret(namespace, name string, conf types.GeneratedPKI) error {
	message.Debugf("k8s.ReplaceTLSSecret(%s, %s, %s)", namespace, name, message.JsonValue(conf))

	secret, err := GenerateTLSSecret(namespace, name, conf)
	if err != nil {
		return err
	}

	return ReplaceSecret(secret)
}

func ReplaceSecret(secret *corev1.Secret) error {
	message.Debugf("k8s.ReplaceSecret(%s, %s)", secret.Namespace, secret.Name)

	if _, err := CreateNamespace(secret.Namespace, nil); err != nil {
		return fmt.Errorf("unable to create or read the namespace: %w", err)
	}

	if err := DeleteSecret(secret); err != nil {
		return err
	}

	return CreateSecret(secret)
}

func DeleteSecret(secret *corev1.Secret) error {
	message.Debugf("k8s.DeleteSecret(%s, %s)", secret.Namespace, secret.Name)
	clientset, err := getClientset()
	if err != nil {
		return err
	}

	namespaceSecrets := clientset.CoreV1().Secrets(secret.Namespace)

	err = namespaceSecrets.Delete(context.TODO(), secret.Name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("error deleting the secret: %w", err)
	}

	return nil
}

func CreateSecret(secret *corev1.Secret) error {
	message.Debugf("k8s.CreateSecret(%s, %s)", secret.Namespace, secret.Name)
	clientset, err := getClientset()
	if err != nil {
		return err
	}

	namespaceSecrets := clientset.CoreV1().Secrets(secret.Namespace)

	// create the given secret
	if _, err := namespaceSecrets.Create(context.TODO(), secret, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("unable to create the secret: %w", err)
	}

	return nil
}
