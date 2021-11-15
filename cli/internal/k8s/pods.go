package k8s

import (
	"context"
	"time"

	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const waitLimit = 30

func WaitForPodsAndContainers(target config.ZarfContainerTarget) []string {

	clientSet := getClientset()
	logContext := logrus.WithFields(logrus.Fields{
		"Namespace": target.Namespace,
		"Selector":  target.Selector,
		"Container": target.Container,
	})

	for count := 0; count < waitLimit; count++ {
		logContext.Info("Looking up K8s pod")

		pods, err := clientSet.CoreV1().Pods(target.Namespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: target.Selector,
		})
		if err != nil {
			logContext.Warn("Unable to find matching pods", err.Error())
			break
		}

		var readyPods []string

		if len(pods.Items) > 0 {
			for _, pod := range pods.Items {

				// Handle container targetting
				if target.Container != "" {
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
					// Regular status checking without a container
					if pod.Status.Phase == v1.PodRunning {
						readyPods = append(readyPods, pod.Name)
					}
				}

			}
			if len(pods.Items) == len(readyPods) {
				return readyPods
			}
		}

		time.Sleep(3 * time.Second)
	}

	logContext.Warn("Pod lookup timeout exceeded")

	return []string{}
}
