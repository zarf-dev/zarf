// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helm contains operations for working with helm charts.
package helm

import (
	"context"
	"fmt"

	"github.com/defenseunicorns/zarf/src/internal/packager/template"
	"github.com/defenseunicorns/zarf/src/pkg/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/k8s"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/variables"
	"github.com/defenseunicorns/zarf/src/types"
	"helm.sh/helm/v3/pkg/action"
)

// UpdateZarfRegistryValues updates the Zarf registry deployment with the new state values
func (h *Helm) UpdateZarfRegistryValues() error {
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

	h.chart = types.ZarfChart{
		Namespace:   "zarf",
		ReleaseName: "zarf-docker-registry",
	}

	err = h.UpdateReleaseValues(registryValues)
	if err != nil {
		return fmt.Errorf("error updating the release values: %w", err)
	}

	return nil
}

// UpdateZarfAgentValues updates the Zarf agent deployment with the new state values
func (h *Helm) UpdateZarfAgentValues(ctx context.Context) error {
	spinner := message.NewProgressSpinner("Gathering information to update Zarf Agent TLS")
	defer spinner.Stop()

	err := h.createActionConfig(cluster.ZarfNamespaceName, spinner)
	if err != nil {
		return fmt.Errorf("unable to initialize the K8s client: %w", err)
	}

	// Get the current agent image from one of its pods.
	pods := h.cluster.WaitForPodsAndContainers(
		ctx,
		k8s.PodLookup{
			Namespace: cluster.ZarfNamespaceName,
			Selector:  "app=agent-hook",
		},
		nil,
	)

	var currentAgentImage transform.Image
	if len(pods) > 0 && len(pods[0].Spec.Containers) > 0 {
		currentAgentImage, err = transform.ParseImageRef(pods[0].Spec.Containers[0].Image)
		if err != nil {
			return fmt.Errorf("unable to parse current agent image reference: %w", err)
		}
	} else {
		return fmt.Errorf("unable to get current agent pod")
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
			h.chart = types.ZarfChart{
				Namespace:   "zarf",
				ReleaseName: release.Name,
			}
			h.variableConfig.SetConstants([]variables.Constant{
				{
					Name:  "AGENT_IMAGE",
					Value: currentAgentImage.Path,
				},
				{
					Name:  "AGENT_IMAGE_TAG",
					Value: currentAgentImage.Tag,
				},
			})
			applicationTemplates, err := template.GetZarfTemplates("zarf-agent", h.state)
			if err != nil {
				return fmt.Errorf("error setting up the templates: %w", err)
			}
			h.variableConfig.SetApplicationTemplates(applicationTemplates)

			err = h.UpdateReleaseValues(map[string]interface{}{})
			if err != nil {
				return fmt.Errorf("error updating the release values: %w", err)
			}
		}
	}

	spinner = message.NewProgressSpinner("Cleaning up Zarf Agent pods after update")
	defer spinner.Stop()

	// Force pods to be recreated to get the updated secret.
	err = h.cluster.DeletePods(
		ctx,
		k8s.PodLookup{
			Namespace: cluster.ZarfNamespaceName,
			Selector:  "app=agent-hook",
		},
	)
	if err != nil {
		return fmt.Errorf("error recycling pods for the Zarf Agent: %w", err)
	}

	spinner.Success()

	return nil
}
