package k8s

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetNodes returns a list of nodes from the k8s cluster.
func (k *K8sClient) GetNodes() (*corev1.NodeList, error) {
	metaOptions := metav1.ListOptions{}
	return k.Clientset.CoreV1().Nodes().List(context.TODO(), metaOptions)
}
