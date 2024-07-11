// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package composer contains functions for composing components within Zarf packages.
package composer

import (
	"fmt"

	"github.com/defenseunicorns/zarf/src/types"
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
	c.Actions = append(c.Actions, override.Actions...)
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
