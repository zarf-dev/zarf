// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package injectorcontroller contains the controller logic for monitoring registry proxy pods.
package injectorcontroller

import (
	"context"
	"time"

	"github.com/zarf-dev/zarf/src/internal/healthchecks"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/state"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/cli-utils/pkg/object"
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
	cluster *cluster.Cluster
}

// New creates a new Controller instance
func New(c *cluster.Cluster) *Controller {
	return &Controller{
		cluster: c,
	}
}

// Start begins polling for registry proxy pod status changes every 5 seconds
func (c *Controller) Start(ctx context.Context) error {
	l := logger.From(ctx)
	l.Info("starting injector controller", "controller", ControllerName, "namespace", Namespace, "pollingInterval", PollingInterval.String())

	ticker := time.NewTicker(PollingInterval)
	defer ticker.Stop()

	payloadCMNames := []string{}
	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{
			"zarf-injector": "payload",
		},
	})
	if err != nil {
		return err
	}
	cmList, err := c.cluster.Clientset.CoreV1().ConfigMaps(state.ZarfNamespaceName).List(ctx, metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	for _, cm := range cmList.Items {
		payloadCMNames = append(payloadCMNames, cm.Name)
	}
	if err != nil {
		return err
	}

	if err := c.pollPods(ctx, payloadCMNames); err != nil {
		l.Error("initial pod check failed", "error", err, "controller", ControllerName)
	}

	for {
		select {
		case <-ctx.Done():
			l.Info("stopping injector controller", "controller", ControllerName)
			return ctx.Err()
		case <-ticker.C:
			if err := c.pollPods(ctx, payloadCMNames); err != nil {
				l.Error("error polling pods", "error", err, "controller", ControllerName)
				// Continue polling even on error
			}
		}
	}
}

// pollPods checks all registry proxy pods for ErrImagePull status
func (c *Controller) pollPods(ctx context.Context, cmNames []string) error {
	logger.From(ctx).Info("starting pod poll")
	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app": DaemonSetName,
		},
	}

	listOptions := metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(&labelSelector),
	}

	podList, err := c.cluster.Clientset.CoreV1().Pods(Namespace).List(ctx, listOptions)
	if err != nil {
		return err
	}

	for _, pod := range podList.Items {
		c.checkPodStatus(ctx, &pod, cmNames)
	}

	return nil
}

// checkPodStatus examines the pod status for ErrImagePull conditions
func (c *Controller) checkPodStatus(ctx context.Context, pod *corev1.Pod, payloadCMNames []string) {
	l := logger.From(ctx)

	// Check container statuses for ErrImagePull
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.State.Waiting != nil && (containerStatus.State.Waiting.Reason == "ErrImagePull" || containerStatus.State.Waiting.Reason == "ImagePullBackOff") {
			l.Info("registry proxy pod has ErrImagePull status",
				"controller", ControllerName,
				"pod", pod.Name,
				"namespace", pod.Namespace,
				"container", containerStatus.Name,
				"reason", containerStatus.State.Waiting.Reason,
				"message", containerStatus.State.Waiting.Message,
			)
			shasum := "d4a6fe82b6c7bc7305dddab60c548304d3faceba66636675ba9c9448ddf36817"
			err := c.cluster.RunInjection(ctx, true, payloadCMNames, shasum, state.IPFamilyIPv4)
			if err != nil {
				l.Error("this is the err", "err", err)
			}
			objs := []object.ObjMetadata{
				{
					GroupKind: schema.GroupKind{
						Group: "apps",
						Kind:  "DaemonSet",
					},
					Namespace: "zarf",
					Name:      "zarf-registry-proxy",
				},
			}
			err = healthchecks.WaitForReady(ctx, c.cluster.Watcher, objs)
			if err != nil {
				l.Error("this is the err", "err", err)
			}
			err = c.cluster.StopInjection(ctx, true)
			if err != nil {
				l.Error("this is the err", "err", err)
			}
		}
	}

}
