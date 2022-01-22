package k8s

import (
	"context"
	"github.com/defenseunicorns/zarf/cli/types"
	"sort"
	"time"

	"github.com/defenseunicorns/zarf/cli/internal/message"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const waitLimit = 30

// WaitForPodsAndContainers holds execution up to 30 seconds waiting for health pods and containers (if specified)
func WaitForPodsAndContainers(target types.ZarfContainerTarget, waitForAllPods bool) []string {

	clientSet := getClientset()

	message.Debugf("Waiting for ready pod %s/%s", target.Namespace, target.Selector)
	for count := 0; count < waitLimit; count++ {

		pods, err := clientSet.CoreV1().Pods(target.Namespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: target.Selector,
		})
		if err != nil {
			message.Error(err, "Unable to find matching pods")
			break
		}

		var readyPods []string

		// Reverse sort by creation time
		sort.Slice(pods.Items, func(i, j int) bool {
			return pods.Items[i].CreationTimestamp.After(pods.Items[j].CreationTimestamp.Time)
		})

		if len(pods.Items) > 0 {
			for _, pod := range pods.Items {
				message.Debugf("Testing pod %s", pod.Name)

				// Handle container targetting
				if target.Container != "" {
					message.Debugf("Testing for container")
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

					// Check the status of regular containers for a runing match
					for _, container := range pod.Status.ContainerStatuses {
						isRunning := container.State.Running != nil
						if isRunning && container.Name == target.Container {
							readyPods = append(readyPods, pod.Name)
						}
					}

				} else {
					status := pod.Status.Phase
					message.Debugf("Testing for pod only, phase: %s", status)
					// Regular status checking without a container
					if status == corev1.PodRunning {
						readyPods = append(readyPods, pod.Name)
					}
				}

			}
			message.Debug("Ready pods", readyPods)
			somePodsReady := len(readyPods) > 0
			allPodsReady := len(pods.Items) == len(readyPods)

			if allPodsReady || somePodsReady && !waitForAllPods {
				return readyPods
			}

		}

		time.Sleep(3 * time.Second)
	}

	message.Warn("Pod lookup timeout exceeded")

	return []string{}
}
