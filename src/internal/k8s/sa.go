package k8s

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetAllServiceAccounts returns a list of services accounts for all namespaces.
func GetAllServiceAccounts() (*corev1.ServiceAccountList, error) {
	return GetServiceAccounts(corev1.NamespaceAll)
}

// GetServiceAccounts returns a list of service accounts in a given namespace
func GetServiceAccounts(namespace string) (*corev1.ServiceAccountList, error) {
	clientset, err := getClientset()
	if err != nil {
		return nil, err
	}

	metaOptions := metav1.ListOptions{}
	return clientset.CoreV1().ServiceAccounts(namespace).List(context.TODO(), metaOptions)
}

// GetServiceAccount reutrns a single service account by namespace and name.
func GetServiceAccount(namespace, name string) (*corev1.ServiceAccount, error) {
	clientset, err := getClientset()
	if err != nil {
		return nil, err
	}

	metaOptions := metav1.GetOptions{}
	return clientset.CoreV1().ServiceAccounts(namespace).Get(context.TODO(), name, metaOptions)
}

// SaveServiceAccount updates the given service account in the cluster
func SaveServiceAccount(svcAccount *corev1.ServiceAccount) (*corev1.ServiceAccount, error) {
	clientset, err := getClientset()
	if err != nil {
		return nil, err
	}

	metaOptions := metav1.UpdateOptions{}
	return clientset.CoreV1().ServiceAccounts(svcAccount.Namespace).Update(context.TODO(), svcAccount, metaOptions)
}
