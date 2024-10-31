// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helm contains operations for working with helm charts.
package helm

import (
	"context"
	"fmt"
	"time"

	"helm.sh/helm/v3/pkg/action"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/cli-utils/pkg/object"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/internal/healthchecks"
	"github.com/zarf-dev/zarf/src/internal/packager/template"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/pkg/utils"
)

// UpdateZarfRegistryValues updates the Zarf registry deployment with the new state values
func (h *Helm) UpdateZarfRegistryValues(ctx context.Context) error {
	pushUser, err := utils.GetHtpasswdString(h.state.RegistryInfo.PushUsername, h.state.RegistryInfo.PushPassword)
	if err != nil {
		return fmt.Errorf("error generating htpasswd string: %w", err)
	}
	pullUser, err := utils.GetHtpasswdString(h.state.RegistryInfo.PullUsername, h.state.RegistryInfo.PullPassword)
	if err != nil {
		return fmt.Errorf("error generating htpasswd string: %w", err)
	}
	registryValues := map[string]interface{}{
		"secrets": map[string]interface{}{
			"htpasswd": fmt.Sprintf("%s\n%s", pushUser, pullUser),
		},
	}
	h.chart = v1alpha1.ZarfChart{
		Namespace:   "zarf",
		ReleaseName: "zarf-docker-registry",
	}
	err = h.UpdateReleaseValues(ctx, registryValues)
	if err != nil {
		return fmt.Errorf("error updating the release values: %w", err)
	}

	objs := []object.ObjMetadata{
		{
			GroupKind: schema.GroupKind{
				Group: "apps",
				Kind:  "Deployment",
			},
			Namespace: "zarf",
			Name:      "zarf-docker-registry",
		},
	}
	waitCtx, waitCancel := context.WithTimeout(ctx, 60*time.Second)
	defer waitCancel()
	err = healthchecks.WaitForReady(waitCtx, h.cluster.Watcher, objs)
	if err != nil {
		return err
	}
	return nil
}

// UpdateZarfAgentValues updates the Zarf agent deployment with the new state values
func (h *Helm) UpdateZarfAgentValues(ctx context.Context) error {
	spinner := message.NewProgressSpinner("Gathering information to update Zarf Agent TLS")
	defer spinner.Stop()

	deployment, err := h.cluster.Clientset.AppsV1().Deployments(cluster.ZarfNamespaceName).Get(ctx, "agent-hook", metav1.GetOptions{})
	if err != nil {
		return err
	}
	agentImage, err := transform.ParseImageRef(deployment.Spec.Template.Spec.Containers[0].Image)
	if err != nil {
		return err
	}

	err = h.createActionConfig(ctx, cluster.ZarfNamespaceName, spinner)
	if err != nil {
		return err
	}

	// List the releases to find the current agent release name.
	listClient := action.NewList(h.actionConfig)
	releases, err := listClient.Run()
	if err != nil {
		return fmt.Errorf("unable to list helm releases: %w", err)
	}
	spinner.Success()

	for _, release := range releases {
		// Update the Zarf Agent release with the new values
		if release.Chart.Name() == "raw-init-zarf-agent-zarf-agent" {
			h.chart = v1alpha1.ZarfChart{
				Namespace:   "zarf",
				ReleaseName: release.Name,
			}
			h.variableConfig.SetConstants([]v1alpha1.Constant{
				{
					Name:  "AGENT_IMAGE",
					Value: agentImage.Path,
				},
				{
					Name:  "AGENT_IMAGE_TAG",
					Value: agentImage.Tag,
				},
			})
			applicationTemplates, err := template.GetZarfTemplates(ctx, "zarf-agent", h.state)
			if err != nil {
				return fmt.Errorf("error setting up the templates: %w", err)
			}
			h.variableConfig.SetApplicationTemplates(applicationTemplates)

			err = h.UpdateReleaseValues(ctx, map[string]interface{}{})
			if err != nil {
				return fmt.Errorf("error updating the release values: %w", err)
			}
		}
	}

	// Trigger a rolling update for the TLS secret update to take effect.
	// https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#updating-a-deployment
	spinner = message.NewProgressSpinner("Performing a rolling update for the Zarf Agent deployment")
	defer spinner.Stop()

	// Re-fetch the agent deployment before we update since the resourceVersion has changed after updating the Helm release values.
	// Avoids this error: https://github.com/kubernetes/kubernetes/issues/28149
	deployment, err = h.cluster.Clientset.AppsV1().Deployments(cluster.ZarfNamespaceName).Get(ctx, "agent-hook", metav1.GetOptions{})
	if err != nil {
		return err
	}
	if deployment.Spec.Template.Annotations == nil {
		deployment.Spec.Template.Annotations = map[string]string{}
	}
	deployment.Spec.Template.Annotations["zarf.dev/restartedAt"] = time.Now().UTC().Format(time.RFC3339)
	_, err = h.cluster.Clientset.AppsV1().Deployments(cluster.ZarfNamespaceName).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	objs := []object.ObjMetadata{
		{
			GroupKind: schema.GroupKind{
				Group: "apps",
				Kind:  "Deployment",
			},
			Namespace: cluster.ZarfNamespaceName,
			Name:      "agent-hook",
		},
	}
	waitCtx, waitCancel := context.WithTimeout(ctx, 60*time.Second)
	defer waitCancel()
	err = healthchecks.WaitForReady(waitCtx, h.cluster.Watcher, objs)
	if err != nil {
		return err
	}

	spinner.Success()
	return nil
}
