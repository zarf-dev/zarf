package k8s

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetNodes() (*corev1.NodeList, error) {
	clientset := getClientset()

	metaOptions := metav1.ListOptions{}
	return clientset.CoreV1().Nodes().List(context.TODO(), metaOptions)
}
