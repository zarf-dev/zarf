package k8s

import (
	"context"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/message"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ReplaceService deletes and re-creates a service
func ReplaceService(service *corev1.Service) (*corev1.Service, error) {
	message.Debugf("k8s.ReplaceService(%#v)", service)

	if err := DeleteService(service.Namespace, service.Name); err != nil {
		return nil, err
	}

	return CreateService(service)
}

// GenerateService returns a K8s service struct without writing to the cluster
func GenerateService(namespace, name string) *corev1.Service {
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

// DeleteService removes a service from the cluster by namespace and name.
func DeleteService(namespace, name string) error {
	message.Debugf("k8s.DeleteService(%s, %s)", namespace, name)
	clientset, err := getClientset()
	if err != nil {
		return err
	}
	return clientset.CoreV1().Services(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
}

// CreateService creates the given service in the cluster.
func CreateService(service *corev1.Service) (*corev1.Service, error) {
	message.Debugf("k8s.CreateService(%#v)", service)
	clientset, err := getClientset()
	if err != nil {
		return nil, err
	}
	createOptions := metav1.CreateOptions{}
	return clientset.CoreV1().Services(service.Namespace).Create(context.TODO(), service, createOptions)
}

// GetService returns a Kubernetes service resource in the provided namespace with the given name.
func GetService(namespace, serviceName string) (*corev1.Service, error) {
	message.Debugf("k8s.GetService(%s, %s)", namespace, serviceName)
	clientset, err := getClientset()
	if err != nil {
		return nil, err
	}
	return clientset.CoreV1().Services(namespace).Get(context.TODO(), serviceName, metav1.GetOptions{})
}

// GetServicesByLabel returns a list of matched services given a label and value.  To search all namespaces, pass "" in the namespace arg
func GetServicesByLabel(namespace, label, value string) (*corev1.ServiceList, error) {
	message.Debugf("k8s.GetServicesByLabel(%s, %s)", namespace, label)
	clientset, err := getClientset()
	if err != nil {
		return nil, err
	}

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
func GetServicesByLabelExists(namespace, label string) (*corev1.ServiceList, error) {
	message.Debugf("k8s.GetServicesByLabelExists(%s, %s)", namespace, label)
	clientset, err := getClientset()
	if err != nil {
		return nil, err
	}

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
