package k8s

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const waitLimit = 10

func WaitForPods(namespace string, selector string) []string {

	clientSet := connect()
	logContext := logrus.WithFields(logrus.Fields{
		"Namespace": namespace,
		"Selector":  selector,
	})

	for count := 0; count < waitLimit; count++ {
		logContext.Info("Looking up K8s pod")

		pods, err := clientSet.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: selector,
		})
		if err != nil {
			logContext.Warn("Unable to find matching pods", err.Error())
			break
		}

		var readyPods []string

		if len(pods.Items) > 0 {
			for _, pod := range pods.Items {
				if pod.Status.Phase == "Running" {
					readyPods = append(readyPods, pod.Name)
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
