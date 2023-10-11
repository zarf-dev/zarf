// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helm contains operations for working with helm charts.
package helm

import (
	"fmt"

	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/internal/packager/git"
	"github.com/defenseunicorns/zarf/src/pkg/k8s"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"helm.sh/helm/v3/pkg/action"
)

// UpdateZarfRegistryValues updates the Zarf registry deployment with the new state values
func (h *Helm) UpdateZarfRegistryValues() error {
	pushUser, err := utils.GetHtpasswdString(h.Cfg.State.RegistryInfo.PushUsername, h.Cfg.State.RegistryInfo.PushPassword)
	if err != nil {
		return fmt.Errorf("error generating htpasswd string: %w", err)
	}

	pullUser, err := utils.GetHtpasswdString(h.Cfg.State.RegistryInfo.PullUsername, h.Cfg.State.RegistryInfo.PullPassword)
	if err != nil {
		return fmt.Errorf("error generating htpasswd string: %w", err)
	}

	registryValues := map[string]interface{}{
		"secrets": map[string]interface{}{
			"htpasswd": fmt.Sprintf("%s\n%s", pushUser, pullUser),
		},
	}

	h.Chart = types.ZarfChart{
		Namespace: "zarf",
	}
	h.ReleaseName = "zarf-docker-registry"

	err = h.UpdateReleaseValues(registryValues)
	if err != nil {
		return fmt.Errorf("error updating the release values: %w", err)
	}

	return nil
}

// UpdateZarfGiteaValues updates the Zarf git server deployment with the new state values
func (h *Helm) UpdateZarfGiteaValues() error {
	giteaValues := map[string]interface{}{
		"gitea": map[string]interface{}{
			"admin": map[string]interface{}{
				"username": h.Cfg.State.GitServer.PushUsername,
				"password": h.Cfg.State.GitServer.PushPassword,
			},
		},
	}

	h.Chart = types.ZarfChart{
		Namespace: "zarf",
	}
	h.ReleaseName = "zarf-gitea"

	err := h.UpdateReleaseValues(giteaValues)
	if err != nil {
		return fmt.Errorf("error updating the release values: %w", err)
	}

	g := git.New(h.Cfg.State.GitServer)
	err = g.CreateReadOnlyUser()
	if err != nil {
		return fmt.Errorf("unable to create the new Gitea read only user: %w", err)
	}

	return nil
}

// UpdateZarfAgentValues updates the Zarf git server deployment with the new state values
func (h *Helm) UpdateZarfAgentValues() error {
	spinner := message.NewProgressSpinner("Gathering information to update Zarf Agent TLS")
	defer spinner.Stop()

	err := h.createActionConfig(cluster.ZarfNamespaceName, spinner)
	if err != nil {
		return fmt.Errorf("unable to initialize the K8s client: %w", err)
	}

	// Get the current agent image from one of its pods.
	pods := h.Cluster.WaitForPodsAndContainers(k8s.PodLookup{
		Namespace: cluster.ZarfNamespaceName,
		Selector:  "app=agent-hook",
	}, nil)

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

	for _, lsRelease := range releases {
		// Update the Zarf Agent release with the new values
		if lsRelease.Chart.Name() == "raw-init-zarf-agent-zarf-agent" {
			h.Chart = types.ZarfChart{
				Namespace: "zarf",
			}
			h.ReleaseName = lsRelease.Name
			h.Component = types.ZarfComponent{
				Name: "zarf-agent",
			}
			h.Cfg.Pkg.Constants = []types.ZarfPackageConstant{
				{
					Name:  "AGENT_IMAGE",
					Value: currentAgentImage.Path,
				},
				{
					Name:  "AGENT_IMAGE_TAG",
					Value: currentAgentImage.Tag,
				},
			}

			err := h.UpdateReleaseValues(map[string]interface{}{})
			if err != nil {
				return fmt.Errorf("error updating the release values: %w", err)
			}
		}
	}

	spinner = message.NewProgressSpinner("Cleaning up Zarf Agent pods after update")
	defer spinner.Stop()

	// Force pods to be recreated to get the updated secret.
	pods = h.Cluster.WaitForPodsAndContainers(k8s.PodLookup{
		Namespace: cluster.ZarfNamespaceName,
		Selector:  "app=agent-hook",
	}, nil)

	for _, pod := range pods {
		h.Cluster.DeletePod(cluster.ZarfNamespaceName, pod.Name)
	}

	spinner.Success()

	return nil
}
