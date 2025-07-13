// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package injectorcontroller contains the controller logic for monitoring registry proxy pods.
package injectorcontroller

import (
	"context"
	"time"

	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/state"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// DaemonSetName is the name of the registry proxy daemonset
	DaemonSetName = "zarf-registry-proxy"
	// Namespace is the namespace where the registry proxy runs
	Namespace = state.ZarfNamespaceName
	// ControllerName is the name used for logging
	ControllerName = "injector-controller"
	// PollingInterval is how often to check pod statuses
	PollingInterval = 5 * time.Second
)

// Controller watches registry proxy pods for ErrImagePull status
type Controller struct {
	clientset kubernetes.Interface
}

// New creates a new Controller instance
func New(clientset kubernetes.Interface) *Controller {
	return &Controller{
		clientset: clientset,
	}
}

// Start begins polling for registry proxy pod status changes every 5 seconds
func (c *Controller) Start(ctx context.Context) error {
	l := logger.From(ctx)
	l.Info("starting injector controller", "controller", ControllerName, "namespace", Namespace, "pollingInterval", PollingInterval.String())

	ticker := time.NewTicker(PollingInterval)
	defer ticker.Stop()

	if err := c.pollPods(ctx); err != nil {
		l.Error("initial pod check failed", "error", err, "controller", ControllerName)
	}

	for {
		select {
		case <-ctx.Done():
			l.Info("stopping injector controller", "controller", ControllerName)
			return ctx.Err()
		case <-ticker.C:
			if err := c.pollPods(ctx); err != nil {
				l.Error("error polling pods", "error", err, "controller", ControllerName)
				// Continue polling even on error
			}
		}
	}
}

// pollPods checks all registry proxy pods for ErrImagePull status
func (c *Controller) pollPods(ctx context.Context) error {
	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app": DaemonSetName,
		},
	}

	listOptions := metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(&labelSelector),
	}

	podList, err := c.clientset.CoreV1().Pods(Namespace).List(ctx, listOptions)
	if err != nil {
		return err
	}

	for _, pod := range podList.Items {
		c.checkPodStatus(ctx, &pod)
	}

	return nil
}

// checkPodStatus examines the pod status for ErrImagePull conditions
func (c *Controller) checkPodStatus(ctx context.Context, pod *corev1.Pod) {
	l := logger.From(ctx)

	// Check container statuses for ErrImagePull
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.State.Waiting != nil && containerStatus.State.Waiting.Reason == "ErrImagePull" {
			l.Info("registry proxy pod has ErrImagePull status",
				"controller", ControllerName,
				"pod", pod.Name,
				"namespace", pod.Namespace,
				"container", containerStatus.Name,
				"reason", containerStatus.State.Waiting.Reason,
				"message", containerStatus.State.Waiting.Message,
			)
		}
	}

	// Check init container statuses for ErrImagePull
	for _, initContainerStatus := range pod.Status.InitContainerStatuses {
		if initContainerStatus.State.Waiting != nil && initContainerStatus.State.Waiting.Reason == "ErrImagePull" {
			l.Info("registry proxy pod init container has ErrImagePull status",
				"controller", ControllerName,
				"pod", pod.Name,
				"namespace", pod.Namespace,
				"initContainer", initContainerStatus.Name,
				"reason", initContainerStatus.State.Waiting.Reason,
				"message", initContainerStatus.State.Waiting.Message,
			)
		}
	}
}
