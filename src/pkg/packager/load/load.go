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
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	internalTypes "github.com/zarf-dev/zarf/src/internal/api/types"
	internalv1alpha1 "github.com/zarf-dev/zarf/src/internal/api/v1alpha1"
	internalv1beta1 "github.com/zarf-dev/zarf/src/internal/api/v1beta1"
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

// DefinedPackage is the result of loading and resolving a package definition.
// ImportedSchemas is transient assembly state — child schema paths collected during
// import resolution that must be passed to AssemblePackage for merging.
type DefinedPackage struct {
	Pkg             v1alpha1.ZarfPackage
	ImportedSchemas []string
}

// PackageDefinition returns a validated package definition after flavors, imports, variables, and values are applied.
// It dispatches on the manifest's apiVersion; v1beta1 packages are converted down to v1alpha1 so the
// rest of Zarf continues to operate on a single internal type.
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

	genPkg, err := pkgcfg.ParseMultiDoc(ctx, b)
	if err != nil {
		return DefinedPackage{}, err
	}

	var defined DefinedPackage
	switch genPkg.Build.OriginalAPIVersion {
	case v1beta1.APIVersion:
		defined, err = v1beta1PackageDefinition(ctx, genPkg, pkgPath, opts)
	case v1alpha1.APIVersion:
		defined, err = v1alpha1PackageDefinition(ctx, genPkg, pkgPath, opts)
	default:
		return DefinedPackage{}, fmt.Errorf("unrecognized API version")
	}
	if err != nil {
		return DefinedPackage{}, err
	}

	l.Debug("done layout.LoadPackage", "duration", time.Since(start))
	return defined, nil
}

func v1alpha1PackageDefinition(ctx context.Context, g internalTypes.Package, pkgPath layout.PackagePath, opts DefinitionOptions) (DefinedPackage, error) {
	pkg := internalv1alpha1.ConvertFromGeneric(g)
	pkg.Metadata.Architecture = config.GetArch(pkg.Metadata.Architecture)
	var err error
	opts.CachePath, err = utils.ResolveCachePath(opts.CachePath)
	if err != nil {
		return DefinedPackage{}, err
	}
	var importedSchemas []string
	pkg, importedSchemas, err = resolveImports(ctx, pkg, pkgPath.ManifestFile, pkg.Metadata.Architecture, opts.Flavor, []string{}, opts.CachePath, opts.SkipVersionCheck, opts.RemoteOptions)
	if err != nil {
		return DefinedPackage{}, err
	}

	if len(pkg.Values.Files) > 0 && !feature.IsEnabled(feature.Values) {
		return DefinedPackage{}, fmt.Errorf("creating package with Values files, but \"%s\" feature is not enabled."+
			" Run again with --features=\"%s=true\"", feature.Values, feature.Values)
	}

	if opts.SetVariables != nil {
		pkg, _, err = fillActiveTemplate(ctx, pkg, opts.SetVariables, opts.IsInteractive)
		if err != nil {
			return DefinedPackage{}, err
		}
	}
	if err := validate(ctx, pkg, pkgPath.ManifestFile, opts.SetVariables, opts.Flavor, opts.SkipRequiredValues, opts.SkipValuesSchemaValidation); err != nil {
		return DefinedPackage{}, err
	}
	return DefinedPackage{Pkg: pkg, ImportedSchemas: importedSchemas}, nil
}

func v1beta1PackageDefinition(ctx context.Context, g internalTypes.Package, pkgPath layout.PackagePath, opts DefinitionOptions) (DefinedPackage, error) {
	pkg := internalv1beta1.ConvertFromGeneric(g)
	pkg.Metadata.Architecture = config.GetArch(pkg.Metadata.Architecture)

	pkg, importedSchemas, err := resolveImportsV1Beta1(ctx, pkg, pkgPath.ManifestFile, pkg.Metadata.Architecture, opts.Flavor)
	if err != nil {
		return DefinedPackage{}, err
	}

	if err := validateV1Beta1(ctx, pkg, pkgPath.ManifestFile, opts.Flavor); err != nil {
		return DefinedPackage{}, err
	}

	v1alpha1Pkg := internalv1alpha1.ConvertFromGeneric(internalv1beta1.ConvertToGeneric(pkg))
	return DefinedPackage{Pkg: v1alpha1Pkg, ImportedSchemas: importedSchemas}, nil
}

func validate(ctx context.Context, pkg v1alpha1.ZarfPackage, packagePath string, setVariables map[string]string, flavor string, skipRequiredValues bool, skipSchemaValidation bool) error {
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
		if err := validateValuesSchema(ctx, pkg, packagePath, validateValuesSchemaOptions{skipRequired: skipRequiredValues}); err != nil {
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

// validateV1Beta1 validates a v1beta1 package before it is converted down to v1alpha1.
// Non-schema v1beta1 rules will grow in src/internal/api/v1beta1 over time; for now this is schema-only.
func validateV1Beta1(ctx context.Context, pkg v1beta1.Package, packagePath string, flavor string) error {
	l := logger.From(ctx)
	start := time.Now()
	l.Debug("start v1beta1 validate",
		"pkg", pkg.Metadata.Name,
		"packagePath", packagePath,
		"flavor", flavor,
	)

	findings, err := lint.ValidatePackageSchemaAtPathV1Beta1(packagePath)
	if err != nil {
		return fmt.Errorf("unable to check schema: %w", err)
	}
	if len(findings) != 0 {
		return &lint.LintError{
			PackageName: pkg.Metadata.Name,
			Findings:    findings,
		}
	}

	// FIXME: need to add validate for v1beta1

	l.Debug("done v1beta1 validate",
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

func validateValuesSchema(ctx context.Context, pkg v1alpha1.ZarfPackage, packagePath string, opts validateValuesSchemaOptions) error {
	// Skip validation if no schema or values files are provided
	if pkg.Values.Schema == "" || len(pkg.Values.Files) == 0 {
		return nil
	}

	l := logger.From(ctx)

	pkgPath, err := layout.ResolvePackagePath(packagePath)
	if err != nil {
		return err
	}

	// Resolve values file paths relative to the package directory
	valueFilePaths := make([]string, len(pkg.Values.Files))
	for i, vf := range pkg.Values.Files {
		valueFilePaths[i] = filepath.Join(pkgPath.BaseDir, vf)
	}

	vals, err := value.ParseFiles(ctx, valueFilePaths, value.ParseFilesOptions{})
	if err != nil {
		return fmt.Errorf("failed to parse values files for validation: %w", err)
	}

	// Resolve declared schema path relative to package root
	schemaPath := filepath.Join(pkgPath.BaseDir, pkg.Values.Schema)
	if err := vals.Validate(ctx, schemaPath, value.ValidateOptions{SkipRequired: opts.skipRequired}); err != nil {
		return fmt.Errorf("values validation failed: %w", err)
	}

	l.Debug("values validated against schema", "schemaPath", schemaPath)
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
