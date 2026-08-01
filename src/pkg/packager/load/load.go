// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package load takes a ZarfPackageConfig, composes imports, and validates the con
package load

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	internalv1alpha1 "github.com/zarf-dev/zarf/src/internal/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/internal/pkgcfg"
	"github.com/zarf-dev/zarf/src/pkg/feature"
	"github.com/zarf-dev/zarf/src/pkg/interactive"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/value"
	"github.com/zarf-dev/zarf/src/types"
)

// DefinitionOptions are the optional parameters to load.PackageDefinition
type DefinitionOptions struct {
	Flavor       string
	SetVariables map[string]string
	// SkipRequiredValues ignores values schema validation errors when a "required" field is empty. Used when a package
	// value should be supplied at deploy-time and doesn't have a default set in the package values.
	SkipRequiredValues bool
	// SkipValuesSchemaValidation skips schema validation for the package values entirely.
	SkipValuesSchemaValidation bool
	// CachePath is used to cache layers from skeleton package pulls
	CachePath string
	// IsInteractive decides if Zarf can interactively prompt users through the CLI
	IsInteractive bool
	// SkipVersionCheck skips version requirement validation
	SkipVersionCheck bool
	types.RemoteOptions
}

// ResolvedValues contains the values and schema resolved from the package and its imports.
type ResolvedValues struct {
	Values           value.Values
	HasValues        bool
	Schema           json.RawMessage
	SchemaOutputPath string
}

// DefinedPackage is the result of loading and resolving a package definition.
type DefinedPackage struct {
	Pkg            v1alpha1.ZarfPackage
	ResolvedValues ResolvedValues
}

// PackageDefinition returns a validated package definition after flavors, imports, variables, and values are applied.
func PackageDefinition(ctx context.Context, packagePath string, opts DefinitionOptions) (DefinedPackage, error) {
	l := logger.From(ctx)
	start := time.Now()
	l.Debug("start layout.LoadPackage",
		"path", packagePath,
		"flavor", opts.Flavor,
		"setVariables", opts.SetVariables,
	)

	pkgPath, err := layout.ResolvePackagePath(packagePath)
	if err != nil {
		return DefinedPackage{}, err
	}

	b, err := os.ReadFile(pkgPath.ManifestFile)
	if err != nil {
		return DefinedPackage{}, err
	}
	pkg, err := pkgcfg.Parse(ctx, b)
	if err != nil {
		return DefinedPackage{}, err
	}
	pkg.Metadata.Architecture = config.GetArch(pkg.Metadata.Architecture)
	opts.CachePath, err = utils.ResolveCachePath(opts.CachePath)
	if err != nil {
		return DefinedPackage{}, err
	}
	resources := importResources{}
	pkg, err = resolveImports(ctx, pkg, pkgPath.ManifestFile, pkg.Metadata.Architecture, opts.Flavor, []string{}, opts.CachePath, opts.SkipVersionCheck, opts.RemoteOptions, &resources)
	if err != nil {
		return DefinedPackage{}, err
	}
	resolvedValues, err := resources.resolve(ctx, pkg.Values.Schema)
	if err != nil {
		return DefinedPackage{}, err
	}
	if resolvedValues.HasValues {
		pkg.Values.Files = []string{layout.ValuesYAML}
	} else {
		pkg.Values.Files = nil
	}
	if len(resolvedValues.Schema) > 0 {
		pkg.Values.Schema = layout.ValuesSchema
	} else {
		pkg.Values.Schema = ""
	}

	if resolvedValues.HasValues && !feature.IsEnabled(feature.Values) {
		return DefinedPackage{}, fmt.Errorf("creating package with Values files, but \"%s\" feature is not enabled."+
			" Run again with --features=\"%s=true\"", feature.Values, feature.Values)
	}

	if opts.SetVariables != nil {
		pkg, _, err = fillActiveTemplate(ctx, pkg, opts.SetVariables, opts.IsInteractive)
		if err != nil {
			return DefinedPackage{}, err
		}
	}
	err = validate(ctx, pkg, pkgPath.ManifestFile, opts.SetVariables, opts.Flavor, opts.SkipRequiredValues, opts.SkipValuesSchemaValidation, resolvedValues)
	if err != nil {
		return DefinedPackage{}, err
	}
	l.Debug("done layout.LoadPackage", "duration", time.Since(start))
	return DefinedPackage{Pkg: pkg, ResolvedValues: resolvedValues}, nil
}

func validate(ctx context.Context, pkg v1alpha1.ZarfPackage, packagePath string, setVariables map[string]string, flavor string, skipRequiredValues bool, skipSchemaValidation bool, resolvedValues ResolvedValues) error {
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
	if err := internalv1alpha1.ValidatePackage(pkg); err != nil {
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

	if !skipSchemaValidation {
		if err := validateValuesSchema(ctx, resolvedValues, validateValuesSchemaOptions{skipRequired: skipRequiredValues}); err != nil {
			return err
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

type validateValuesSchemaOptions struct {
	skipRequired bool
}

func validateValuesSchema(ctx context.Context, resolvedValues ResolvedValues, opts validateValuesSchemaOptions) error {
	if len(resolvedValues.Schema) == 0 || !resolvedValues.HasValues {
		return nil
	}

	l := logger.From(ctx)

	if err := resolvedValues.Values.ValidateDocument(ctx, resolvedValues.Schema, value.ValidateOptions{SkipRequired: opts.skipRequired}); err != nil {
		return fmt.Errorf("values validation failed: %w", err)
	}

	l.Debug("values validated against schema", "schema", "in-memory")
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

func fillActiveTemplate(ctx context.Context, pkg v1alpha1.ZarfPackage, setVariables map[string]string, isInteractive bool) (v1alpha1.ZarfPackage, []string, error) {
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
			if !present && isInteractive {
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
