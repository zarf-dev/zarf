package k8s

import (
	"context"
	"crypto/tls"
	"fmt"

	"github.com/defenseunicorns/zarf/src/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (k *K8sClient) GetSecret(namespace, name string) (*corev1.Secret, error) {
	return k.Clientset.CoreV1().Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

func (k *K8sClient) GetSecretsWithLabel(namespace, labelSelector string) (*corev1.SecretList, error) {
	listOptions := metav1.ListOptions{LabelSelector: labelSelector}
	return k.Clientset.CoreV1().Secrets(namespace).List(context.TODO(), listOptions)
}

func (k *K8sClient) GenerateSecret(namespace, name string, secretType corev1.SecretType) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    k.Labels,
		},
		Type: secretType,
		Data: map[string][]byte{},
	}
}

func (k *K8sClient) GenerateTLSSecret(namespace, name string, conf types.GeneratedPKI) (*corev1.Secret, error) {
	if _, err := tls.X509KeyPair(conf.Cert, conf.Key); err != nil {
		return nil, err
	}

	secretTLS := k.GenerateSecret(namespace, name, corev1.SecretTypeTLS)
	secretTLS.Data[corev1.TLSCertKey] = conf.Cert
	secretTLS.Data[corev1.TLSPrivateKeyKey] = conf.Key

	return secretTLS, nil
}

func (k *K8sClient) ReplaceTLSSecret(namespace, name string, conf types.GeneratedPKI) error {
	secret, err := k.GenerateTLSSecret(namespace, name, conf)
	if err != nil {
		return err
	}

	return k.ReplaceSecret(secret)
}

func (k *K8sClient) ReplaceSecret(secret *corev1.Secret) error {
	if _, err := k.CreateNamespace(secret.Namespace, nil); err != nil {
		return fmt.Errorf("unable to create or read the namespace: %w", err)
	}

	if err := k.DeleteSecret(secret); err != nil {
		return err
	}

	return k.CreateSecret(secret)
}

func (k *K8sClient) DeleteSecret(secret *corev1.Secret) error {
	namespaceSecrets := k.Clientset.CoreV1().Secrets(secret.Namespace)

	err := namespaceSecrets.Delete(context.TODO(), secret.Name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("error deleting the secret: %w", err)
	}

	return nil
}

func (k *K8sClient) CreateSecret(secret *corev1.Secret) error {
	namespaceSecrets := k.Clientset.CoreV1().Secrets(secret.Namespace)

	// create the given secret
	if _, err := namespaceSecrets.Create(context.TODO(), secret, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("unable to create the secret: %w", err)
	}

	return nil
}
