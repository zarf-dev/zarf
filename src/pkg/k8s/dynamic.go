// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package k8s provides a client for interacting with a Kubernetes cluster.
package k8s

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
)

// AddLabelsAndAnnotations adds the provided labels and annotations to the specified K8s resource
func (k *K8s) AddLabelsAndAnnotations(ctx context.Context, resourceNamespace, resourceName string, groupKind schema.GroupKind, labels, annotations map[string]string) error {
	return k.updateLabelsAndAnnotations(ctx, resourceNamespace, resourceName, groupKind, labels, annotations, false)
}

// RemoveLabelsAndAnnotations removes the provided labels and annotations to the specified K8s resource
func (k *K8s) RemoveLabelsAndAnnotations(ctx context.Context, resourceNamespace, resourceName string, groupKind schema.GroupKind, labels, annotations map[string]string) error {
	return k.updateLabelsAndAnnotations(ctx, resourceNamespace, resourceName, groupKind, labels, annotations, true)
}

// updateLabelsAndAnnotations updates the provided labels and annotations to the specified K8s resource
func (k *K8s) updateLabelsAndAnnotations(ctx context.Context, resourceNamespace, resourceName string, groupKind schema.GroupKind, labels, annotations map[string]string, isRemove bool) error {
	dynamicClient := dynamic.NewForConfigOrDie(k.RestConfig)

	discoveryClient := discovery.NewDiscoveryClientForConfigOrDie(k.RestConfig)

	groupResources, err := restmapper.GetAPIGroupResources(discoveryClient)
	if err != nil {
		return err
	}
	mapper := restmapper.NewDiscoveryRESTMapper(groupResources)

	mapping, err := mapper.RESTMapping(groupKind)
	if err != nil {
		return err
	}

	deployedResource, err := dynamicClient.Resource(mapping.Resource).Namespace(resourceNamespace).Get(ctx, resourceName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// Pull the existing labels from the rendered resource
	deployedLabels := deployedResource.GetLabels()
	if deployedLabels == nil {
		// Ensure label map exists to avoid nil panic
		deployedLabels = make(map[string]string)
	}
	for key, value := range labels {
		if isRemove {
			delete(deployedLabels, key)
		} else {
			deployedLabels[key] = value
		}
	}

	deployedResource.SetLabels(deployedLabels)

	// Pull the existing annotations from the rendered resource
	deployedAnnotations := deployedResource.GetAnnotations()
	if deployedAnnotations == nil {
		// Ensure label map exists to avoid nil panic
		deployedAnnotations = make(map[string]string)
	}
	for key, value := range annotations {
		if isRemove {
			delete(deployedAnnotations, key)
		} else {
			deployedAnnotations[key] = value
		}
	}

	deployedResource.SetAnnotations(deployedAnnotations)

	_, err = dynamicClient.Resource(mapping.Resource).Namespace(resourceNamespace).Update(ctx, deployedResource, metav1.UpdateOptions{})
	return err
}
