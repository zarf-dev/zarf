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

// GeneratePod creates a new pod without adding it to the k8s cluster
func (k *Client) GeneratePod(name, namespace string) *corev1.Pod {
	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    k.Labels,
		},
	}
}

// DeletePod removees a pod from the cluster by namespace & name
func (k *Client) DeletePod(namespace string, name string) error {
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

// CreatePod inserts the given pod into the cluster
func (k *Client) CreatePod(pod *corev1.Pod) (*corev1.Pod, error) {
	createOptions := metav1.CreateOptions{}
	return k.Clientset.CoreV1().Pods(pod.Namespace).Create(context.TODO(), pod, createOptions)
}

// GetAllPods returns a list of pods from the cluster for all namesapces
func (k *Client) GetAllPods() (*corev1.PodList, error) {
	return k.GetPods(corev1.NamespaceAll)
}

// GetPods returns a list of pods from the cluster by namespace
func (k *Client) GetPods(namespace string) (*corev1.PodList, error) {
	metaOptions := metav1.ListOptions{}
	return k.Clientset.CoreV1().Pods(namespace).List(context.TODO(), metaOptions)
}

// WaitForPodsAndContainers holds execution up to 30 seconds waiting for health pods and containers (if specified)
func (k *Client) WaitForPodsAndContainers(target PodLookup, waitForAllPods bool) []string {
	for count := 0; count < waitLimit; count++ {

		pods, err := k.Clientset.CoreV1().Pods(target.Namespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: target.Selector,
		})
		if err != nil {
			k.Log("Unable to find matching pods: %w", err)
			break
		}

		var readyPods []string

		// Reverse sort by creation time
		sort.Slice(pods.Items, func(i, j int) bool {
			return pods.Items[i].CreationTimestamp.After(pods.Items[j].CreationTimestamp.Time)
		})

		if len(pods.Items) > 0 {
			for _, pod := range pods.Items {
				k.Log("Testing pod %s", pod.Name)
				k.Log("%#v", pod)

				// Handle container targetting
				if target.Container != "" {
					k.Log("Testing for container")
					var matchesInitContainer bool

					// Check the status of initContainers for a running match
					for _, initContainer := range pod.Status.InitContainerStatuses {
						isRunning := initContainer.State.Running != nil
						if isRunning && initContainer.Name == target.Container {
							// On running match in initContainer break this loop
							matchesInitContainer = true
							readyPods = append(readyPods, pod.Name)
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
							readyPods = append(readyPods, pod.Name)
						}
					}

				} else {
					status := pod.Status.Phase
					k.Log("Testing for pod only, phase: %s", status)
					// Regular status checking without a container
					if status == corev1.PodRunning {
						readyPods = append(readyPods, pod.Name)
					}
				}

			}

			k.Log("Ready pods", readyPods)
			somePodsReady := len(readyPods) > 0
			allPodsReady := len(pods.Items) == len(readyPods)

			if allPodsReady || somePodsReady && !waitForAllPods {
				return readyPods
			}
		}

		time.Sleep(3 * time.Second)
	}

	k.Log("Pod lookup timeout exceeded")

	return []string{}
}
