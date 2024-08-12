// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package creator contains functions for creating Zarf packages.
package creator

import (
	"fmt"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/interactive"
	"github.com/zarf-dev/zarf/src/pkg/utils"
)

// FillActiveTemplate merges user-specified variables into the configuration templates of a zarf.yaml.
func FillActiveTemplate(pkg v1alpha1.ZarfPackage, setVariables map[string]string) (v1alpha1.ZarfPackage, []string, error) {
	templateMap := map[string]string{}
	warnings := []string{}

	promptAndSetTemplate := func(templatePrefix string, deprecated bool) error {
		yamlTemplates, err := utils.FindYamlTemplates(&pkg, templatePrefix, "###")
		if err != nil {
			return err
		}

		for key := range yamlTemplates {
			if deprecated {
				warnings = append(warnings, fmt.Sprintf(lang.PkgValidateTemplateDeprecation, key, key, key))
			}

			_, present := setVariables[key]
			if !present && !config.CommonOptions.Confirm {
				setVal, err := interactive.PromptVariable(v1alpha1.InteractiveVariable{
					Variable: v1alpha1.Variable{Name: key},
				})
				if err != nil {
					return err
				}
				setVariables[key] = setVal
			} else if !present {
				return fmt.Errorf("template %q must be '--set' when using the '--confirm' flag", key)
			}
		}

		for key, value := range setVariables {
			templateMap[fmt.Sprintf("%s%s###", templatePrefix, key)] = value
		}

		return nil
	}

	// update the component templates on the package
	if err := ReloadComponentTemplatesInPackage(&pkg); err != nil {
		return v1alpha1.ZarfPackage{}, nil, err
	}

	if err := promptAndSetTemplate(v1alpha1.ZarfPackageTemplatePrefix, false); err != nil {
		return v1alpha1.ZarfPackage{}, nil, err
	}
	// [DEPRECATION] Set the Package Variable syntax as well for backward compatibility
	if err := promptAndSetTemplate(v1alpha1.ZarfPackageVariablePrefix, true); err != nil {
		return v1alpha1.ZarfPackage{}, nil, err
	}

	// Add special variable for the current package architecture
	templateMap[v1alpha1.ZarfPackageArch] = pkg.Metadata.Architecture

	if err := utils.ReloadYamlTemplate(&pkg, templateMap); err != nil {
		return v1alpha1.ZarfPackage{}, nil, err
	}

	return pkg, warnings, nil
}

// ReloadComponentTemplate appends ###ZARF_COMPONENT_NAME### for the component, assigns value, and reloads
// Any instance of ###ZARF_COMPONENT_NAME### within a component will be replaced with that components name
func ReloadComponentTemplate(component *v1alpha1.ZarfComponent) error {
	mappings := map[string]string{}
	mappings[v1alpha1.ZarfComponentName] = component.Name
	err := utils.ReloadYamlTemplate(component, mappings)
	if err != nil {
		return err
	}
	return nil
}

// ReloadComponentTemplatesInPackage appends ###ZARF_COMPONENT_NAME###  for each component, assigns value, and reloads
func ReloadComponentTemplatesInPackage(zarfPackage *v1alpha1.ZarfPackage) error {
	// iterate through components to and find all ###ZARF_COMPONENT_NAME, assign to component Name and value
	for i := range zarfPackage.Components {
		if err := ReloadComponentTemplate(&zarfPackage.Components[i]); err != nil {
			return err
		}
	}

	return nil
}
