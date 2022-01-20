package k8s

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetService returns a Kubernetes service resource in the provided namespace with the given name.
func GetService(namespace string, serviceName string) (*corev1.Service, error) {
	clientset := getClientset()
	return clientset.CoreV1().Services(namespace).Get(context.Background(), serviceName, metav1.GetOptions{})
}
