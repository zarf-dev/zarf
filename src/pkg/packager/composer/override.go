// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package composer contains functions for composing components within Zarf packages.
package composer

import (
	"fmt"

	"github.com/zarf-dev/zarf/src/types"
)

func overrideMetadata(c *types.ZarfComponent, override types.ZarfComponent) error {
	c.Name = override.Name
	c.Default = override.Default
	c.Required = override.Required

	// Override description if it was provided.
	if override.Description != "" {
		c.Description = override.Description
	}

	if override.Only.LocalOS != "" {
		if c.Only.LocalOS != "" {
			return fmt.Errorf("component %q: \"only.localOS\" %q cannot be redefined as %q during compose", c.Name, c.Only.LocalOS, override.Only.LocalOS)
		}

		c.Only.LocalOS = override.Only.LocalOS
	}

	return nil
}

func overrideDeprecated(c *types.ZarfComponent, override types.ZarfComponent) {
	// Override cosign key path if it was provided.
	if override.DeprecatedCosignKeyPath != "" {
		c.DeprecatedCosignKeyPath = override.DeprecatedCosignKeyPath
	}

	c.DeprecatedGroup = override.DeprecatedGroup

	// Merge deprecated scripts for backwards compatibility with older zarf binaries.
	c.DeprecatedScripts.Before = append(c.DeprecatedScripts.Before, override.DeprecatedScripts.Before...)
	c.DeprecatedScripts.After = append(c.DeprecatedScripts.After, override.DeprecatedScripts.After...)

	if override.DeprecatedScripts.Retry {
		c.DeprecatedScripts.Retry = true
	}
	if override.DeprecatedScripts.ShowOutput {
		c.DeprecatedScripts.ShowOutput = true
	}
	if override.DeprecatedScripts.TimeoutSeconds > 0 {
		c.DeprecatedScripts.TimeoutSeconds = override.DeprecatedScripts.TimeoutSeconds
	}
}

func overrideActions(c *types.ZarfComponent, override types.ZarfComponent) {
	// Merge create actions.
	c.Actions.OnCreate.Defaults = override.Actions.OnCreate.Defaults
	c.Actions.OnCreate.Before = append(c.Actions.OnCreate.Before, override.Actions.OnCreate.Before...)
	c.Actions.OnCreate.After = append(c.Actions.OnCreate.After, override.Actions.OnCreate.After...)
	c.Actions.OnCreate.OnFailure = append(c.Actions.OnCreate.OnFailure, override.Actions.OnCreate.OnFailure...)
	c.Actions.OnCreate.OnSuccess = append(c.Actions.OnCreate.OnSuccess, override.Actions.OnCreate.OnSuccess...)

	// Merge deploy actions.
	c.Actions.OnDeploy.Defaults = override.Actions.OnDeploy.Defaults
	c.Actions.OnDeploy.Before = append(c.Actions.OnDeploy.Before, override.Actions.OnDeploy.Before...)
	c.Actions.OnDeploy.After = append(c.Actions.OnDeploy.After, override.Actions.OnDeploy.After...)
	c.Actions.OnDeploy.OnFailure = append(c.Actions.OnDeploy.OnFailure, override.Actions.OnDeploy.OnFailure...)
	c.Actions.OnDeploy.OnSuccess = append(c.Actions.OnDeploy.OnSuccess, override.Actions.OnDeploy.OnSuccess...)

	// Merge remove actions.
	c.Actions.OnRemove.Defaults = override.Actions.OnRemove.Defaults
	c.Actions.OnRemove.Before = append(c.Actions.OnRemove.Before, override.Actions.OnRemove.Before...)
	c.Actions.OnRemove.After = append(c.Actions.OnRemove.After, override.Actions.OnRemove.After...)
	c.Actions.OnRemove.OnFailure = append(c.Actions.OnRemove.OnFailure, override.Actions.OnRemove.OnFailure...)
	c.Actions.OnRemove.OnSuccess = append(c.Actions.OnRemove.OnSuccess, override.Actions.OnRemove.OnSuccess...)
}

func overrideResources(c *types.ZarfComponent, override types.ZarfComponent) {
	c.DataInjections = append(c.DataInjections, override.DataInjections...)
	c.Files = append(c.Files, override.Files...)
	c.Images = append(c.Images, override.Images...)
	c.Repos = append(c.Repos, override.Repos...)

	// Merge charts with the same name to keep them unique
	for _, overrideChart := range override.Charts {
		existing := false
		for idx := range c.Charts {
			if c.Charts[idx].Name == overrideChart.Name {
				if overrideChart.Namespace != "" {
					c.Charts[idx].Namespace = overrideChart.Namespace
				}
				if overrideChart.ReleaseName != "" {
					c.Charts[idx].ReleaseName = overrideChart.ReleaseName
				}
				c.Charts[idx].ValuesFiles = append(c.Charts[idx].ValuesFiles, overrideChart.ValuesFiles...)
				existing = true
			}
		}

		if !existing {
			c.Charts = append(c.Charts, overrideChart)
		}
	}

	// Merge manifests with the same name to keep them unique
	for _, overrideManifest := range override.Manifests {
		existing := false
		for idx := range c.Manifests {
			if c.Manifests[idx].Name == overrideManifest.Name {
				if overrideManifest.Namespace != "" {
					c.Manifests[idx].Namespace = overrideManifest.Namespace
				}
				c.Manifests[idx].Files = append(c.Manifests[idx].Files, overrideManifest.Files...)
				c.Manifests[idx].Kustomizations = append(c.Manifests[idx].Kustomizations, overrideManifest.Kustomizations...)

				existing = true
			}
		}

		if !existing {
			c.Manifests = append(c.Manifests, overrideManifest)
		}
	}
}
