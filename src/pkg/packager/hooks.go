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
	"github.com/defenseunicorns/zarf/src/types/hooks"
)

// GetAllClusterHooks searches the zarf namespace for all hooks and stores them within the Packager
func (p *Packager) getAllClusterHooks() error {

	// Get all secrets with the hook label
	hookSecrets, err := p.cluster.Kube.GetSecretsWithLabel(cluster.ZarfNamespaceName, hooks.HookSecretPrefix)
	if err != nil {
		return fmt.Errorf("unable to get hook secrets")
	}
	for _, hookSecret := range hookSecrets.Items {
		hookConfig := hooks.HookConfig{}

		// Get any data from the hook secret
		err := json.Unmarshal(hookSecret.Data["data"], &hookConfig)
		if err != nil {
			return fmt.Errorf("unable to load the hook configuration for the %s hook: %w", hookSecret.Name, err)
		}

		p.hooks[hookSecret.Name] = hookConfig
	}

	return nil
}

func (p *Packager) runPreDeployHooks() error {
	return p.runPackageHooks(hooks.BeforePackage)
}

func (p *Packager) runPostDeployHooks() error {
	return p.runPackageHooks(hooks.AfterPackage)
}

func (p *Packager) runPreComponentHooks(component types.ZarfComponent) error {
	return p.runComponentHooks(hooks.BeforeComponent, component)
}

func (p *Packager) runPostComponentHooks(component types.ZarfComponent) error {
	return p.runComponentHooks(hooks.AfterComponent, component)
}

func (p *Packager) runPackageHooks(lifecycle hooks.HookLifecycle) error {

	// If we are not able to run the hooks, return early
	if p.cluster == nil || p.hooks == nil {
		return nil
	}

	for _, hookConfig := range p.hooks {
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

func (p *Packager) runComponentHooks(lifecycle hooks.HookLifecycle, component types.ZarfComponent) error {

	// If we are not able to run the hooks, return early
	if p.cluster == nil || p.hooks == nil {
		return nil
	}
	for _, hookConfig := range p.hooks {
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

func runInternalPackageHook(hook hooks.HookConfig) error {
	if hook.HookName == hooks.ECRCredentialsHook {
		if err := internalHook.AuthToECR(hook); err != nil {
			return err
		}
	}

	return nil
}

func runInternalComponentHook(hook hooks.HookConfig, component types.ZarfComponent) error {
	if hook.HookName == hooks.ECRRepositoryHook {
		if err := internalHook.CreateTheECRRepos(hook, component.Images); err != nil {
			return err
		}
	}
	return nil
}
