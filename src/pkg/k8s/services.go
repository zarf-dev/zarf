// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package k8s provides a client for interacting with a Kubernetes cluster.
package k8s

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"

	"github.com/defenseunicorns/pkg/helpers"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// See https://regex101.com/r/OWVfAO/1.
const serviceURLPattern = `^(?P<name>[^\.]+)\.(?P<namespace>[^\.]+)\.svc\.cluster\.local$`

// ServiceInfo contains information necessary for connecting to a cluster service.
type ServiceInfo struct {
	Namespace string
	Name      string
	Port      int
}

// ReplaceService deletes and re-creates a service.
func (k *K8s) ReplaceService(service *corev1.Service) (*corev1.Service, error) {
	if err := k.DeleteService(service.Namespace, service.Name); err != nil {
		return nil, err
	}

	return k.CreateService(service)
}

// GenerateService returns a K8s service struct without writing to the cluster.
func (k *K8s) GenerateService(namespace, name string) *corev1.Service {
	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: make(Labels),
			Labels:      make(Labels),
		},
	}

	return service
}

// DeleteService removes a service from the cluster by namespace and name.
func (k *K8s) DeleteService(namespace, name string) error {
	return k.Clientset.CoreV1().Services(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
}

// CreateService creates the given service in the cluster.
func (k *K8s) CreateService(service *corev1.Service) (*corev1.Service, error) {
	createOptions := metav1.CreateOptions{}
	return k.Clientset.CoreV1().Services(service.Namespace).Create(context.TODO(), service, createOptions)
}

// GetService returns a Kubernetes service resource in the provided namespace with the given name.
func (k *K8s) GetService(namespace, serviceName string) (*corev1.Service, error) {
	return k.Clientset.CoreV1().Services(namespace).Get(context.TODO(), serviceName, metav1.GetOptions{})
}

// GetServices returns a list of services in the provided namespace.  To search all namespaces, pass "" in the namespace arg.
func (k *K8s) GetServices(namespace string) (*corev1.ServiceList, error) {
	return k.Clientset.CoreV1().Services(namespace).List(context.TODO(), metav1.ListOptions{})
}

// GetServicesByLabel returns a list of matched services given a label and value.  To search all namespaces, pass "" in the namespace arg.
func (k *K8s) GetServicesByLabel(namespace, label, value string) (*corev1.ServiceList, error) {
	// Create the selector and add the requirement
	labelSelector, _ := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: Labels{
			label: value,
		},
	})

	// Run the query with the selector and return as a ServiceList
	return k.Clientset.CoreV1().Services(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector.String()})
}

// GetServicesByLabelExists returns a list of matched services given a label.  To search all namespaces, pass "" in the namespace arg.
func (k *K8s) GetServicesByLabelExists(namespace, label string) (*corev1.ServiceList, error) {
	// Create the selector and add the requirement
	labelSelector, _ := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{{
			Key:      label,
			Operator: metav1.LabelSelectorOpExists,
		}},
	})

	// Run the query with the selector and return as a ServiceList
	return k.Clientset.CoreV1().Services(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector.String()})
}

// ServiceInfoFromNodePortURL takes a nodePortURL and parses it to find the service info for connecting to the cluster. The string is expected to follow the following format:
// Example nodePortURL: 127.0.0.1:{PORT}.
func (k *K8s) ServiceInfoFromNodePortURL(nodePortURL string) (*ServiceInfo, error) {
	// Attempt to parse as normal, if this fails add a scheme to the URL (docker registries don't use schemes)
	parsedURL, err := url.Parse(nodePortURL)
	if err != nil {
		parsedURL, err = url.Parse("scheme://" + nodePortURL)
		if err != nil {
			return nil, err
		}
	}

	// Match hostname against localhost ip/hostnames
	hostname := parsedURL.Hostname()
	if hostname != helpers.IPV4Localhost && hostname != "localhost" {
		return nil, fmt.Errorf("node port services should be on localhost")
	}

	// Get the node port from the nodeportURL.
	nodePort, err := strconv.Atoi(parsedURL.Port())
	if err != nil {
		return nil, err
	}
	if nodePort < 30000 || nodePort > 32767 {
		return nil, fmt.Errorf("node port services should use the port range 30000-32767")
	}

	services, err := k.GetServices("")
	if err != nil {
		return nil, err
	}

	for _, svc := range services.Items {
		if svc.Spec.Type == "NodePort" {
			for _, port := range svc.Spec.Ports {
				if int(port.NodePort) == nodePort {
					return &ServiceInfo{
						Namespace: svc.Namespace,
						Name:      svc.Name,
						Port:      int(port.Port),
					}, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("no matching node port services found")
}

// ServiceInfoFromServiceURL takes a serviceURL and parses it to find the service info for connecting to the cluster. The string is expected to follow the following format:
// Example serviceURL: http://{SERVICE_NAME}.{NAMESPACE}.svc.cluster.local:{PORT}.
func ServiceInfoFromServiceURL(serviceURL string) (*ServiceInfo, error) {
	parsedURL, err := url.Parse(serviceURL)
	if err != nil {
		return nil, err
	}

	// Get the remote port from the serviceURL.
	remotePort, err := strconv.Atoi(parsedURL.Port())
	if err != nil {
		return nil, err
	}

	// Match hostname against local cluster service format.
	pattern := regexp.MustCompile(serviceURLPattern)
	get, err := helpers.MatchRegex(pattern, parsedURL.Hostname())

	// If incomplete match, return an error.
	if err != nil {
		return nil, err
	}

	return &ServiceInfo{
		Namespace: get("namespace"),
		Name:      get("name"),
		Port:      remotePort,
	}, nil
}
