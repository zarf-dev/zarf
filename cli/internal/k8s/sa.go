package k8s

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetAllServiceAccounts() (*corev1.ServiceAccountList, error) {
	return GetServiceAccounts(corev1.NamespaceAll)
}

func GetServiceAccounts(namespace string) (*corev1.ServiceAccountList, error) {
	clientset := getClientset()

	metaOptions := metav1.ListOptions{}
	return clientset.CoreV1().ServiceAccounts(namespace).List(context.TODO(), metaOptions)
}

func GetServiceAccount(namespace string, name string) (*corev1.ServiceAccount, error) {
	clientset := getClientset()

	metaOptions := metav1.GetOptions{}
	return clientset.CoreV1().ServiceAccounts(namespace).Get(context.TODO(), name, metaOptions)
}

func SaveServiceAccount(svcAccount *corev1.ServiceAccount) (*corev1.ServiceAccount, error) {
	clientset := getClientset()

	metaOptions := metav1.UpdateOptions{}
	return clientset.CoreV1().ServiceAccounts(svcAccount.Namespace).Update(context.TODO(), svcAccount, metaOptions)
}
