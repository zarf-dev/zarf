// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package k8s provides a client for interacting with a Kubernetes cluster.
package k8s

import (
	"context"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const waitLimit = 30

// GeneratePod creates a new pod without adding it to the k8s cluster.
func (k *K8s) GeneratePod(name, namespace string) *corev1.Pod {
	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    make(Labels),
		},
	}

	return pod
}

// DeletePod removes a pod from the cluster by namespace & name.
func (k *K8s) DeletePod(namespace string, name string) error {
	deleteGracePeriod := int64(0)
	deletePolicy := metav1.DeletePropagationForeground
	err := k.Clientset.CoreV1().Pods(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{
		GracePeriodSeconds: &deleteGracePeriod,
		PropagationPolicy:  &deletePolicy,
	})

	if err != nil {
		return err
	}

	for {
		// Keep checking for the pod to be deleted
		_, err := k.Clientset.CoreV1().Pods(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
}

// DeletePods removes a collection of pods from the cluster by pod lookup.
func (k *K8s) DeletePods(target PodLookup) error {
	deleteGracePeriod := int64(0)
	deletePolicy := metav1.DeletePropagationForeground
	return k.Clientset.CoreV1().Pods(target.Namespace).DeleteCollection(context.TODO(),
		metav1.DeleteOptions{
			GracePeriodSeconds: &deleteGracePeriod,
			PropagationPolicy:  &deletePolicy,
		},
		metav1.ListOptions{
			LabelSelector: target.Selector,
		},
	)
}

// CreatePod inserts the given pod into the cluster.
func (k *K8s) CreatePod(pod *corev1.Pod) (*corev1.Pod, error) {
	createOptions := metav1.CreateOptions{}
	return k.Clientset.CoreV1().Pods(pod.Namespace).Create(context.TODO(), pod, createOptions)
}

// GetAllPods returns a list of pods from the cluster for all namespaces.
func (k *K8s) GetAllPods() (*corev1.PodList, error) {
	return k.GetPods(corev1.NamespaceAll)
}

// GetPods returns a list of pods from the cluster by namespace.
func (k *K8s) GetPods(namespace string) (*corev1.PodList, error) {
	metaOptions := metav1.ListOptions{}
	return k.Clientset.CoreV1().Pods(namespace).List(context.TODO(), metaOptions)
}

// WaitForPodsAndContainers attempts to find pods matching the given selector and optional inclusion filter
// It will wait up to 90 seconds for the pods to be found and will return a list of matching pod names
// If the timeout is reached, an empty list will be returned.
func (k *K8s) WaitForPodsAndContainers(target PodLookup, include PodFilter) []corev1.Pod {
	for count := 0; count < waitLimit; count++ {

		pods, err := k.Clientset.CoreV1().Pods(target.Namespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: target.Selector,
		})
		if err != nil {
			k.Log("Unable to find matching pods: %w", err)
			break
		}

		k.Log("Found %d pods for target %#v", len(pods.Items), target)

		var readyPods = []corev1.Pod{}

		// Sort the pods from newest to oldest
		sort.Slice(pods.Items, func(i, j int) bool {
			return pods.Items[i].CreationTimestamp.After(pods.Items[j].CreationTimestamp.Time)
		})

		for _, pod := range pods.Items {
			k.Log("Testing pod %q", pod.Name)

			// If an include function is provided, only keep pods that return true
			if include != nil && !include(pod) {
				continue
			}

			// Handle container targeting
			if target.Container != "" {
				k.Log("Testing pod %q for container %q", pod.Name, target.Container)
				var matchesInitContainer bool

				// Check the status of initContainers for a running match
				for _, initContainer := range pod.Status.InitContainerStatuses {
					isRunning := initContainer.State.Running != nil
					if isRunning && initContainer.Name == target.Container {
						// On running match in initContainer break this loop
						matchesInitContainer = true
						readyPods = append(readyPods, pod)
						break
					}
				}

				// Don't check any further if there's already a match
				if matchesInitContainer {
					continue
				}

				// Check the status of regular containers for a running match
				for _, container := range pod.Status.ContainerStatuses {
					isRunning := container.State.Running != nil
					if isRunning && container.Name == target.Container {
						readyPods = append(readyPods, pod)
					}
				}
			} else {
				status := pod.Status.Phase
				k.Log("Testing pod %q phase, want (%q) got (%q)", pod.Name, corev1.PodRunning, status)
				// Regular status checking without a container
				if status == corev1.PodRunning {
					readyPods = append(readyPods, pod)
				}
			}
		}

		if len(readyPods) > 0 {
			return readyPods
		}

		time.Sleep(3 * time.Second)
	}

	k.Log("Pod lookup timeout exceeded")

	return []corev1.Pod{}
}

// FindPodContainerPort will find a pod's container port from a service and return it.
//
// Returns 0 if no port is found.
func (k *K8s) FindPodContainerPort(svc corev1.Service) int {
	selectorLabelsOfPods := MakeLabels(svc.Spec.Selector)
	pods := k.WaitForPodsAndContainers(PodLookup{
		Namespace: svc.Namespace,
		Selector:  selectorLabelsOfPods,
	}, nil)

	for _, pod := range pods {
		// Find the matching name on the port in the pod
		for _, container := range pod.Spec.Containers {
			for _, port := range container.Ports {
				if port.Name == svc.Spec.Ports[0].TargetPort.String() {
					return int(port.ContainerPort)
				}
			}
		}
	}

	return 0
}
