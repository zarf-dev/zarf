// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package load takes a ZarfPackageConfig, composes imports, and validates the con
package load

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/internal/pkgcfg"
	"github.com/zarf-dev/zarf/src/pkg/interactive"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/utils"
)

// DefinitionOptions are the optional parameters to load.PackageDefinition
type DefinitionOptions struct {
	Flavor       string
	SetVariables map[string]string
	CachePath    string
}

// PackageDefinition returns a validated package definition after flavors, imports, and variables are applied.
func PackageDefinition(ctx context.Context, packagePath string, opts DefinitionOptions) (v1alpha1.ZarfPackage, error) {
	l := logger.From(ctx)
	start := time.Now()
	l.Debug("start layout.LoadPackage",
		"path", packagePath,
		"flavor", opts.Flavor,
		"setVariables", opts.SetVariables)

	// Load PackageConfig from disk
	b, err := os.ReadFile(filepath.Join(packagePath, layout.ZarfYAML))
	if err != nil {
		return v1alpha1.ZarfPackage{}, err
	}
	pkg, err := pkgcfg.Parse(ctx, b)
	if err != nil {
		return v1alpha1.ZarfPackage{}, err
	}
	pkg.Metadata.Architecture = config.GetArch(pkg.Metadata.Architecture)
	pkg, err = resolveImports(ctx, pkg, packagePath, pkg.Metadata.Architecture, opts.Flavor, []string{}, opts.CachePath)
	if err != nil {
		return v1alpha1.ZarfPackage{}, err
	}
	if opts.SetVariables != nil {
		pkg, _, err = fillActiveTemplate(ctx, pkg, opts.SetVariables)
		if err != nil {
			return v1alpha1.ZarfPackage{}, err
		}
	}
	err = validate(ctx, pkg, packagePath, opts.SetVariables, opts.Flavor)
	if err != nil {
		return v1alpha1.ZarfPackage{}, err
	}
	l.Debug("done layout.LoadPackage", "duration", time.Since(start))
	return pkg, nil
}

func validate(ctx context.Context, pkg v1alpha1.ZarfPackage, packagePath string, setVariables map[string]string, flavor string) error {
	l := logger.From(ctx)
	start := time.Now()
	l.Debug("start layout.Validate",
		"pkg", pkg.Metadata.Name,
		"packagePath", packagePath,
		"flavor", flavor,
		"setVariables", setVariables,
	)

	if !hasFlavoredComponent(pkg, flavor) {
		l.Warn("flavor not used in package", "flavor", flavor)
	}
	if err := lint.ValidatePackage(pkg); err != nil {
		return fmt.Errorf("package validation failed: %w", err)
	}
	findings, err := lint.ValidatePackageSchemaAtPath(packagePath, setVariables)
	if err != nil {
		return fmt.Errorf("unable to check schema: %w", err)
	}
	if len(findings) != 0 {
		return &lint.LintError{
			PackageName: pkg.Metadata.Name,
			Findings:    findings,
		}
	}

	l.Debug("done layout.Validate",
		"pkg", pkg.Metadata.Name,
		"path", packagePath,
		"findings", findings,
		"duration", time.Since(start),
	)

	return nil
}

func hasFlavoredComponent(pkg v1alpha1.ZarfPackage, flavor string) bool {
	for _, comp := range pkg.Components {
		if comp.Only.Flavor == flavor {
			return true
		}
	}
	return false
}

func fillActiveTemplate(ctx context.Context, pkg v1alpha1.ZarfPackage, setVariables map[string]string) (v1alpha1.ZarfPackage, []string, error) {
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
				setVal, err := interactive.PromptVariable(ctx, v1alpha1.InteractiveVariable{
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
	if err := reloadComponentTemplatesInPackage(&pkg); err != nil {
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

// reloadComponentTemplate appends ###ZARF_COMPONENT_NAME### for the component, assigns value, and reloads
// Any instance of ###ZARF_COMPONENT_NAME### within a component will be replaced with that components name
func reloadComponentTemplate(component *v1alpha1.ZarfComponent) error {
	mappings := map[string]string{}
	mappings[v1alpha1.ZarfComponentName] = component.Name
	err := utils.ReloadYamlTemplate(component, mappings)
	if err != nil {
		return err
	}
	return nil
}

// reloadComponentTemplatesInPackage appends ###ZARF_COMPONENT_NAME###  for each component, assigns value, and reloads
func reloadComponentTemplatesInPackage(zarfPackage *v1alpha1.ZarfPackage) error {
	// iterate through components to and find all ###ZARF_COMPONENT_NAME, assign to component Name and value
	for i := range zarfPackage.Components {
		if err := reloadComponentTemplate(&zarfPackage.Components[i]); err != nil {
			return err
		}
	}
	return nil
}
