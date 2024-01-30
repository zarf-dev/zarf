// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package creator contains functions for creating Zarf packages.
package creator

import (
	"fmt"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/interactive"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

func FillActiveTemplate(pkg types.ZarfPackage, createOpts types.ZarfCreateOptions) (templatedPkg types.ZarfPackage, warnings []string, err error) {
	templatedPkg = pkg
	templateMap := map[string]string{}

	promptAndSetTemplate := func(templatePrefix string, deprecated bool) error {
		yamlTemplates, err := utils.FindYamlTemplates(templatedPkg, templatePrefix, "###")
		if err != nil {
			return err
		}

		for key := range yamlTemplates {
			if deprecated {
				warnings = append(warnings, fmt.Sprintf(lang.PkgValidateTemplateDeprecation, key, key, key))
			}

			_, present := createOpts.SetVariables[key]
			if !present && !config.CommonOptions.Confirm {
				setVal, err := interactive.PromptVariable(types.ZarfPackageVariable{
					Name: key,
				})
				if err != nil {
					return err
				}
				createOpts.SetVariables[key] = setVal
			} else if !present {
				// erroring out here
				return fmt.Errorf("template %q must be '--set' when using the '--confirm' flag", key)
			}
		}

		for key, value := range createOpts.SetVariables {
			templateMap[fmt.Sprintf("%s%s###", templatePrefix, key)] = value
		}

		return nil
	}

	// update the component templates on the package
	if err := ReloadComponentTemplatesInPackage(&templatedPkg); err != nil {
		return pkg, nil, err
	}

	if err := promptAndSetTemplate(types.ZarfPackageTemplatePrefix, false); err != nil {
		return pkg, nil, err
	}
	// [DEPRECATION] Set the Package Variable syntax as well for backward compatibility
	if err := promptAndSetTemplate(types.ZarfPackageVariablePrefix, true); err != nil {
		return pkg, nil, err
	}

	// Add special variable for the current package architecture
	templateMap[types.ZarfPackageArch] = templatedPkg.Build.Architecture

	if err := utils.ReloadYamlTemplate(&templatedPkg, templateMap); err != nil {
		return pkg, nil, err
	}

	return templatedPkg, warnings, nil
}

// ReloadComponentTemplate appends ###ZARF_COMPONENT_NAME### for the component, assigns value, and reloads
// Any instance of ###ZARF_COMPONENT_NAME### within a component will be replaced with that components name
func ReloadComponentTemplate(component *types.ZarfComponent) error {
	mappings := map[string]string{}
	mappings[types.ZarfComponentName] = component.Name
	err := utils.ReloadYamlTemplate(component, mappings)
	if err != nil {
		return err
	}
	return nil
}

// ReloadComponentTemplatesInPackage appends ###ZARF_COMPONENT_NAME###  for each component, assigns value, and reloads
func ReloadComponentTemplatesInPackage(zarfPackage *types.ZarfPackage) error {
	// iterate through components to and find all ###ZARF_COMPONENT_NAME, assign to component Name and value
	for i := range zarfPackage.Components {
		if err := ReloadComponentTemplate(&zarfPackage.Components[i]); err != nil {
			return err
		}
	}

	return nil
}
