package k8s

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetServiceAccounts() (*corev1.ServiceAccountList, error) {
	clientset := getClientset()

	metaOptions := metav1.ListOptions{}
	return clientset.CoreV1().ServiceAccounts(corev1.NamespaceAll).List(context.TODO(), metaOptions)
}
