package k8s

import (
	"context"

	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/message"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GenerateService(namespace string, name string) *corev1.Service {
	message.Debugf("k8s.GenerateService(%s, %s)", name, namespace)
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: make(map[string]string),
			Labels: map[string]string{
				// track the creation of this ns by zarf
				config.ZarfManagedByLabel: "zarf",
			},
		},
	}
}

func DeleteService(namespace string, name string) error {
	message.Debugf("k8s.DeleteService(%s, %s)", namespace, name)
	clientset := getClientset()
	return clientset.CoreV1().Services(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
}

func CreateService(service *corev1.Service) (*corev1.Service, error) {
	message.Debugf("k8s.CreateService(%v)", service)
	clientset := getClientset()
	createOptions := metav1.CreateOptions{}
	return clientset.CoreV1().Services(service.Namespace).Create(context.TODO(), service, createOptions)
}

// GetService returns a Kubernetes service resource in the provided namespace with the given name.
func GetService(namespace string, serviceName string) (*corev1.Service, error) {
	message.Debugf("k8s.GetService(%s, %s)", namespace, serviceName)
	clientset := getClientset()
	return clientset.CoreV1().Services(namespace).Get(context.TODO(), serviceName, metav1.GetOptions{})
}

// GetServicesByLabel returns a list of matched services given a label and value.  To search all namespaces, pass "" in the namespace arg
func GetServicesByLabel(namespace string, label string, value string) (*corev1.ServiceList, error) {
	message.Debugf("k8s.GetServicesByLabel(%s, %s)", namespace, label)
	clientset := getClientset()

	// Creat the selector and add the requirement
	labelSelector, _ := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{
			label: value,
		},
	})

	// Run the query with the selector and return as a ServiceList
	return clientset.CoreV1().Services(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector.String()})
}

// GetServicesByLabelExists returns a list of matched services given a label.  To search all namespaces, pass "" in the namespace arg
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
