package k8s

import (
	"context"

	"github.com/defenseunicorns/zarf/cli/internal/message"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetService returns a Kubernetes service resource in the provided namespace with the given name.
func GetService(namespace string, serviceName string) (*corev1.Service, error) {
	message.Debugf("k8s.GetService(%s, %s)", namespace, serviceName)
	clientset := getClientset()
	return clientset.CoreV1().Services(namespace).Get(context.Background(), serviceName, metav1.GetOptions{})
}

// GetServicesByLabelExists returns a list of matched services given a set of labels.  TO search all namespaces, pass "" in the namespace arg
func GetServicesByLabelExists(namespace string, label string) (*corev1.ServiceList, error) {
	message.Debugf("k8s.GetServicesByLabelExists(%s, %s)", namespace, label)
	clientset := getClientset()

	// Creat the selector and add the requirement
	labelSelector, _ := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{{
			Key:      label,
			Operator: metav1.LabelSelectorOpExists,
		}},
	})

	// Run the query with the selector and return as a ServiceList
	return clientset.CoreV1().Services(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector.String()})
}
