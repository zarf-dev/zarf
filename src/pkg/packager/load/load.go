// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package load takes a ZarfPackageConfig, composes imports, and validates the con
package load

import (
	"context"
	"fmt"
	"os"
	"slices"
	"time"

	goyaml "github.com/goccy/go-yaml"

	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	internalv1alpha1 "github.com/zarf-dev/zarf/src/internal/api/v1alpha1"
	internalv1beta1 "github.com/zarf-dev/zarf/src/internal/api/v1beta1"
	"github.com/zarf-dev/zarf/src/internal/pkgcfg"
	"github.com/zarf-dev/zarf/src/pkg/feature"
	"github.com/zarf-dev/zarf/src/pkg/interactive"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/types"
)

// DefinitionOptions are the optional parameters to load.PackageDefinition.
type DefinitionOptions struct {
	Flavor       string
	SetVariables map[string]string
	// CachePath is used to cache layers from skeleton package pulls
	CachePath string
	// IsInteractive decides if Zarf can interactively prompt users through the CLI
	IsInteractive bool
	// SkipVersionCheck skips version requirement validation
	// TODO: implement version requirements for v1beta1
	SkipVersionCheck bool
	types.RemoteOptions
}

type valuePlan struct {
	// files are ordered for merge; later paths win.
	files []string
	// schemas are ordered by precedence; earlier paths win.
	schemas []string
}

type resolution struct {
	definition      api.PackageDefinition
	packageRoot     string
	values          valuePlan
	remoteResources []remoteResource
}

// PackageDefinition returns a structurally validated package definition after
// flavors, imports, and variables are applied. It deliberately does not read
// package resource contents; callers that need values, schemas, charts, or
// manifests must use Package.
func PackageDefinition(ctx context.Context, packagePath string, opts DefinitionOptions) (api.PackageDefinition, error) {
	resolved, err := resolve(ctx, packagePath, opts)
	if err != nil {
		return api.PackageDefinition{}, err
	}
	return resolved.definition, nil
}

func resolve(ctx context.Context, packagePath string, opts DefinitionOptions) (resolution, error) {
	l := logger.From(ctx)
	start := time.Now()
	l.Debug("start layout.LoadPackage",
		"path", packagePath,
		"flavor", opts.Flavor,
		"setVariables", opts.SetVariables,
	)

	pkgPath, err := layout.ResolvePackagePath(packagePath)
	if err != nil {
		return resolution{}, err
	}

	b, err := os.ReadFile(pkgPath.ManifestFile)
	if err != nil {
		return resolution{}, err
	}

	version, err := pkgcfg.SelectVersion(ctx, b)
	if err != nil {
		return resolution{}, err
	}

	var defined resolution
	switch version {
	case v1beta1.APIVersion:
		pkg, err := pkgcfg.ParseAs(ctx, b, pkgcfg.V1Beta1)
		if err != nil {
			return resolution{}, err
		}
		if err := validatePackageSchemaV1Beta1(pkg.Metadata.Name, b); err != nil {
			return resolution{}, err
		}
		defined, err = v1beta1Resolution(ctx, pkg, pkgPath, opts)
		if err != nil {
			return resolution{}, err
		}
	case v1alpha1.APIVersion:
		pkg, err := pkgcfg.ParseAs(ctx, b, pkgcfg.V1Alpha1)
		if err != nil {
			return resolution{}, err
		}
		if err := validatePackageSchemaV1Alpha1(pkg.Metadata.Name, b, opts.SetVariables); err != nil {
			return resolution{}, err
		}
		defined, err = v1alpha1Resolution(ctx, pkg, pkgPath, opts)
		if err != nil {
			return resolution{}, err
		}
	default:
		return resolution{}, fmt.Errorf("unrecognized API version")
	}

	l.Debug("done layout.LoadPackage", "duration", time.Since(start))
	return defined, nil
}

func v1alpha1Resolution(ctx context.Context, pkg v1alpha1.ZarfPackage, pkgPath layout.PackagePath, opts DefinitionOptions) (resolution, error) {
	pkg.Metadata.Architecture = config.GetArch(pkg.Metadata.Architecture)
	var err error
	opts.CachePath, err = utils.ResolveCachePath(opts.CachePath)
	if err != nil {
		return resolution{}, err
	}
	var importedSchemas []string
	pkg, importedSchemas, err = resolveImports(ctx, pkg, pkgPath.ManifestFile, pkg.Metadata.Architecture, opts.Flavor, []string{}, opts.CachePath, opts.SkipVersionCheck, opts.RemoteOptions)
	if err != nil {
		return resolution{}, err
	}

	if len(pkg.Values.Files) > 0 && !feature.IsEnabled(feature.Values) {
		return resolution{}, fmt.Errorf("creating package with Values files, but \"%s\" feature is not enabled."+
			" Run again with --features=\"%s=true\"", feature.Values, feature.Values)
	}

	if opts.SetVariables != nil {
		pkg, _, err = fillActiveTemplate(ctx, pkg, opts.SetVariables, opts.IsInteractive)
		if err != nil {
			return resolution{}, err
		}
	}
	if err := validateV1alpha1(ctx, pkg, pkgPath.ManifestFile, opts.Flavor); err != nil {
		return resolution{}, err
	}
	return resolution{
		definition:  api.NewPackageDefinitionFromV1alpha1(pkg),
		packageRoot: pkgPath.BaseDir,
		values: valuePlan{
			files:   pkg.Values.Files,
			schemas: schemaSources(pkg.Values.Schema, importedSchemas),
		},
	}, nil
}

func v1beta1Resolution(ctx context.Context, pkg v1beta1.Package, pkgPath layout.PackagePath, opts DefinitionOptions) (resolution, error) {
	pkg.Metadata.Architecture = config.GetArch(pkg.Metadata.Architecture)
	cachePath, err := utils.ResolveCachePath(opts.CachePath)
	if err != nil {
		return resolution{}, err
	}

	pkg, importedSchemas, remoteResources, err := resolveImportsV1Beta1(ctx, pkg, pkgPath, pkg.Metadata.Architecture, opts.Flavor, opts.RemoteOptions, cachePath)
	if err != nil {
		return resolution{}, err
	}

	if err := validateV1Beta1(ctx, pkg, pkgPath.ManifestFile, opts.Flavor); err != nil {
		return resolution{}, err
	}

	return resolution{
		definition:      api.NewPackageDefinitionFromV1beta1(pkg),
		packageRoot:     pkgPath.BaseDir,
		remoteResources: remoteResources,
		values: valuePlan{
			files:   pkg.Values.Files,
			schemas: schemaSources(pkg.Values.Schema, importedSchemas),
		},
	}, nil
}

func schemaSources(parent string, imported []string) []string {
	sources := make([]string, 0, len(imported)+1)
	if parent != "" {
		sources = append(sources, parent)
	}
	return append(sources, imported...)
}

func validateV1alpha1(ctx context.Context, pkg v1alpha1.ZarfPackage, packagePath string, flavor string) error {
	l := logger.From(ctx)
	start := time.Now()
	l.Debug("start layout.Validate",
		"pkg", pkg.Metadata.Name,
		"packagePath", packagePath,
		"flavor", flavor,
	)

	if !hasFlavoredComponent(pkg, flavor) {
		l.Warn("flavor not used in package", "flavor", flavor)
	}
	if err := internalv1alpha1.ValidatePackage(pkg); err != nil {
		return fmt.Errorf("package validation failed: %w", err)
	}

	l.Debug("done layout.Validate",
		"pkg", pkg.Metadata.Name,
		"path", packagePath,
		"duration", time.Since(start),
	)

	return nil
}

// validateV1Beta1 validates a v1beta1 package before it is converted down to v1alpha1.
func validateV1Beta1(ctx context.Context, pkg v1beta1.Package, packagePath string, flavor string) error {
	l := logger.From(ctx)
	start := time.Now()
	l.Debug("start v1beta1 validate",
		"pkg", pkg.Metadata.Name,
		"packagePath", packagePath,
		"flavor", flavor,
	)

	if !hasFlavoredComponentV1Beta1(pkg, flavor) {
		l.Warn("flavor not used in package", "flavor", flavor)
	}
	if validationErrs := internalv1beta1.ValidatePackage(pkg); len(validationErrs) > 0 {
		return fmt.Errorf("package validation failed:\n%w", validationErrs)
	}

	// Validate after import just in case
	resolvedPackage, err := goyaml.Marshal(pkg)
	if err != nil {
		return fmt.Errorf("unable to marshal resolved package: %w", err)
	}
	if err := validatePackageSchemaV1Beta1(pkg.Metadata.Name, resolvedPackage); err != nil {
		return err
	}

	l.Debug("done v1beta1 validate",
		"pkg", pkg.Metadata.Name,
		"path", packagePath,
		"duration", time.Since(start),
	)
	return nil
}

func validatePackageSchemaV1Alpha1(pkgName string, b []byte, setVariables map[string]string) error {
	findings, err := lint.ValidatePackageSchemaBytesV1Alpha1(b, setVariables)
	if err != nil {
		return fmt.Errorf("unable to check schema: %w", err)
	}
	if len(findings) == 0 {
		return nil
	}
	return &lint.LintError{
		PackageName: pkgName,
		Findings:    findings,
	}
}

func validatePackageSchemaV1Beta1(pkgName string, b []byte) error {
	findings, err := lint.ValidatePackageSchemaBytesV1Beta1(b)
	if err != nil {
		return fmt.Errorf("unable to check schema: %w", err)
	}
	if len(findings) == 0 {
		return nil
	}
	return &lint.LintError{
		PackageName: pkgName,
		Findings:    findings,
	}
}

func hasFlavoredComponent(pkg v1alpha1.ZarfPackage, flavor string) bool {
	return slices.ContainsFunc(pkg.Components, func(comp v1alpha1.ZarfComponent) bool {
		return comp.Only.Flavor == flavor
	})
}

func hasFlavoredComponentV1Beta1(pkg v1beta1.Package, flavor string) bool {
	return slices.ContainsFunc(pkg.Components, func(comp v1beta1.Component) bool {
		return comp.Selector.Flavor == flavor
	})
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
