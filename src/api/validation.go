// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package api

import (
	"errors"
	"fmt"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
)

// Validate rejects fields that cannot be represented by the package's API version.
// An omitted apiVersion is assumed v1alpha1. This does not validate field values correctness
func (p Package) Validate() error {
	version := p.GetAPIVersion()
	if version != v1alpha1.APIVersion && version != v1beta1.APIVersion {
		return fmt.Errorf("unsupported package apiVersion %q", version)
	}

	var errs []error
	add := func(path string) {
		errs = append(errs, fmt.Errorf("%s is not supported in %s", path, version))
	}

	if version == v1beta1.APIVersion {
		if p.Kind == ZarfInitConfig {
			add("kind ZarfInitConfig")
		}
		for _, field := range []struct {
			name  string
			value string
		}{
			{"url", p.Metadata.URL},
			{"image", p.Metadata.Image},
			{"authors", p.Metadata.Authors},
			{"documentation", p.Metadata.Documentation},
			{"source", p.Metadata.Source},
			{"vendor", p.Metadata.Vendor},
		} {
			if field.value != "" {
				add("metadata." + field.name)
			}
		}
		if p.Metadata.YOLO {
			add("metadata.yolo")
		}
		if len(p.Build.DifferentialMissing) > 0 {
			add("build.differentialMissing")
		}
		if len(p.Variables) > 0 {
			add("variables")
		}
		if len(p.Constants) > 0 {
			add("constants")
		}
	}

	for i, component := range p.Components {
		path := fmt.Sprintf("components[%d]", i)
		switch version {
		case v1alpha1.APIVersion:
			if component.Service != "" {
				add(path + ".service")
			}
			if len(component.Import.Local) > 1 || len(component.Import.Remote) > 1 {
				add(path + ".import (multiple sources)")
			}
			for j, image := range component.Images {
				if image.Source != "" {
					add(fmt.Sprintf("%s.images[%d].source", path, j))
				}
			}
		case v1beta1.APIVersion:
			if component.Default {
				add(path + ".default")
			}
			if component.Group != "" {
				add(path + ".group")
			}
			if len(component.DataInjections) > 0 {
				add(path + ".dataInjections")
			}
			if len(component.HealthChecks) > 0 {
				add(path + ".healthChecks")
			}
			if len(component.Distros) > 0 {
				add(path + ".distros")
			}
			if component.Import.Name != "" {
				add(path + ".import.name")
			}
			for j, chart := range component.Charts {
				if chart.LegacyVersion != "" {
					add(fmt.Sprintf("%s.charts[%d].legacyVersion", path, j))
				}
				if len(chart.Variables) > 0 {
					add(fmt.Sprintf("%s.charts[%d].variables", path, j))
				}
			}
			for j, repository := range component.Repositories {
				if repository.LegacyURL != "" {
					add(fmt.Sprintf("%s.repositories[%d].legacyURL", path, j))
				}
			}
			for _, actionSet := range []struct {
				name string
				set  ActionSet
			}{
				{"onCreate", component.Actions.OnCreate},
				{"onDeploy", component.Actions.OnDeploy},
				{"onRemove", component.Actions.OnRemove},
			} {
				setPath := path + ".actions." + actionSet.name
				if len(actionSet.set.After) > 0 {
					add(setPath + ".after")
				}
				for _, actions := range []struct {
					name  string
					items []Action
				}{
					{"before", actionSet.set.Before},
					{"onSuccess", actionSet.set.OnSuccess},
					{"onFailure", actionSet.set.OnFailure},
				} {
					for j, action := range actions.items {
						actionPath := fmt.Sprintf("%s.%s[%d]", setPath, actions.name, j)
						if len(action.SetVariables) > 0 {
							add(actionPath + ".setVariables")
						}
						for k, value := range action.SetValues {
							if value.Value != nil {
								add(fmt.Sprintf("%s.setValues[%d].value", actionPath, k))
							}
						}
					}
				}
			}
		}
	}

	return errors.Join(errs...)
}
