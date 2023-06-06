// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"encoding/json"
	"fmt"

	"github.com/defenseunicorns/zarf/src/internal/cluster"
	internalHook "github.com/defenseunicorns/zarf/src/internal/packager/hook"
	"github.com/defenseunicorns/zarf/src/types"
)

// GetAllClusterHooks searches the zarf namespace for all hooks and stores them within the Packager
func (p *Packager) getAllClusterHooks() error {

	// Get all secrets with the hook label
	hookSecrets, err := p.cluster.Kube.GetSecretsWithLabel(cluster.ZarfNamespaceName, types.HookSecretPrefix)
	if err != nil {
		return fmt.Errorf("unable to get hook secrets")
	}
	for _, hookSecret := range hookSecrets.Items {
		hookConfig := types.HookConfig{}

		// Get any data from the hook secret
		err := json.Unmarshal(hookSecret.Data["data"], &hookConfig)
		if err != nil {
			return fmt.Errorf("unable to load the hook configuration for the %s hook: %w", hookSecret.Name, err)
		}

		p.pluginHooks[hookSecret.Name] = hookConfig
	}

	return nil
}

func (p *Packager) runPreDeployHooks() error {
	return p.runPackageHooks(types.BeforePackage)
}

func (p *Packager) runPostDeployHooks() error {
	return p.runPackageHooks(types.AfterPackage)
}

func (p *Packager) runPreComponentHooks(component types.ZarfComponent) error {
	return p.runComponentHooks(types.BeforeComponent, component)
}

func (p *Packager) runPostComponentHooks(component types.ZarfComponent) error {
	return p.runComponentHooks(types.AfterComponent, component)
}

func (p *Packager) runPackageHooks(lifecycle types.HookLifecycle) error {

	// If we are not able to run the hooks, return early
	if p.cluster == nil || p.pluginHooks == nil {
		return nil
	}

	for _, hookConfig := range p.pluginHooks {
		if hookConfig.Lifecycle == lifecycle {
			if hookConfig.Internal {
				if err := runInternalPackageHook(hookConfig); err != nil {
					return err
				}
			} else {
				if err := hookConfig.Run(); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (p *Packager) runComponentHooks(lifecycle types.HookLifecycle, component types.ZarfComponent) error {

	// If we are not able to run the hooks, return early
	if p.cluster == nil || p.pluginHooks == nil {
		return nil
	}
	for _, hookConfig := range p.pluginHooks {
		if hookConfig.Lifecycle == lifecycle {
			if hookConfig.Internal {
				if err := runInternalComponentHook(hookConfig, component); err != nil {
					return err
				}
			} else {
				if err := hookConfig.Run(); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func runInternalPackageHook(hook types.HookConfig) error {
	if hook.HookName == types.ECRCredentialsHook {
		if err := internalHook.AuthToECR(hook); err != nil {
			return err
		}
	}

	return nil
}

func runInternalComponentHook(hook types.HookConfig, component types.ZarfComponent) error {
	if hook.HookName == types.ECRRepositoryHook {
		if err := internalHook.CreateTheECRRepos(hook, component.Images); err != nil {
			return err
		}
	}
	return nil
}
