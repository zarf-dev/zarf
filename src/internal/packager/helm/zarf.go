// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helm contains operations for working with helm charts.
package helm

import (
	"fmt"

	"github.com/defenseunicorns/zarf/src/internal/packager/git"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

// UpdateZarfRegistryValues updates the Zarf registry deployment with the new state values
func (h *HelmCfg) UpdateZarfRegistryValues() error {
	pushUser, err := utils.GetHtpasswdString(h.State.RegistryInfo.PushUsername, h.State.RegistryInfo.PushPassword)
	if err != nil {
		return fmt.Errorf("error generating htpasswd string: %w", err)
	}

	pullUser, err := utils.GetHtpasswdString(h.State.RegistryInfo.PullUsername, h.State.RegistryInfo.PullPassword)
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
func (h *HelmCfg) UpdateZarfGiteaValues() error {
	giteaValues := map[string]interface{}{
		"gitea": map[string]interface{}{
			"admin": map[string]interface{}{
				"username": h.State.GitServer.PushUsername,
				"password": h.State.GitServer.PushPassword,
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

	g := git.New(h.State.GitServer)
	err = g.CreateReadOnlyUser()
	if err != nil {
		return fmt.Errorf("unable to create the new Gitea read only user: %w", err)
	}

	return nil
}
