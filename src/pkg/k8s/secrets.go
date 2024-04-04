// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package k8s provides a client for interacting with a Kubernetes cluster.
package k8s

import (
	"context"
	"crypto/tls"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetSecret returns a Kubernetes secret.
func (k *K8s) GetSecret(namespace, name string) (*corev1.Secret, error) {
	return k.Clientset.CoreV1().Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

// GetSecretsWithLabel returns a list of Kubernetes secrets with the given label.
func (k *K8s) GetSecretsWithLabel(namespace, labelSelector string) (*corev1.SecretList, error) {
	listOptions := metav1.ListOptions{LabelSelector: labelSelector}
	return k.Clientset.CoreV1().Secrets(namespace).List(context.TODO(), listOptions)
}

// GenerateSecret returns a Kubernetes secret object without applying it to the cluster.
func (k *K8s) GenerateSecret(namespace, name string, secretType corev1.SecretType) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				zarfManagedByLabel: "zarf",
			},
		},
		Type: secretType,
		Data: map[string][]byte{},
	}
}

// GenerateTLSSecret returns a Kubernetes secret object without applying it to the cluster.
func (k *K8s) GenerateTLSSecret(namespace, name string, conf GeneratedPKI) (*corev1.Secret, error) {
	if _, err := tls.X509KeyPair(conf.Cert, conf.Key); err != nil {
		return nil, err
	}

	secretTLS := k.GenerateSecret(namespace, name, corev1.SecretTypeTLS)
	secretTLS.Data[corev1.TLSCertKey] = conf.Cert
	secretTLS.Data[corev1.TLSPrivateKeyKey] = conf.Key

	return secretTLS, nil
}

// CreateOrUpdateTLSSecret creates or updates a Kubernetes secret with a new TLS secret.
func (k *K8s) CreateOrUpdateTLSSecret(namespace, name string, conf GeneratedPKI) (*corev1.Secret, error) {
	secret, err := k.GenerateTLSSecret(namespace, name, conf)
	if err != nil {
		return secret, err
	}

	return k.CreateOrUpdateSecret(secret)
}

// DeleteSecret deletes a Kubernetes secret.
func (k *K8s) DeleteSecret(secret *corev1.Secret) error {
	namespaceSecrets := k.Clientset.CoreV1().Secrets(secret.Namespace)

	err := namespaceSecrets.Delete(context.TODO(), secret.Name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("error deleting the secret: %w", err)
	}

	return nil
}

// CreateOrUpdateSecret creates or updates a Kubernetes secret.
func (k *K8s) CreateOrUpdateSecret(secret *corev1.Secret) (createdSecret *corev1.Secret, err error) {

	namespaceSecrets := k.Clientset.CoreV1().Secrets(secret.Namespace)

	if _, err = k.GetSecret(secret.Namespace, secret.Name); err != nil {
		// create the given secret
		if createdSecret, err = namespaceSecrets.Create(context.TODO(), secret, metav1.CreateOptions{}); err != nil {
			return createdSecret, fmt.Errorf("unable to create the secret: %w", err)
		}
	} else {
		// update the given secret
		if createdSecret, err = namespaceSecrets.Update(context.TODO(), secret, metav1.UpdateOptions{}); err != nil {
			return createdSecret, fmt.Errorf("unable to update the secret: %w", err)
		}
	}

	return createdSecret, nil
}
