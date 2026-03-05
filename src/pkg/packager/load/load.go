// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package load takes a ZarfPackageConfig, composes imports, and validates the con
package load

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	// CachePath is used to cache layers from skeleton package pulls
	CachePath string
	// IsInteractive decides if Zarf can interactively prompt users through the CLI
	IsInteractive bool
	// SkipVersionCheck skips version requirement validation
	SkipVersionCheck bool
	types.RemoteOptions
}

// DefinitionResult contains the loaded package definition and resources that must persist
// until package assembly is complete.
type DefinitionResult struct {
	Pkg     v1alpha1.ZarfPackage
	tempDir string
}

// TempDir returns the path to the temporary directory used during package definition loading.
// This directory contains transformed files that must persist until package assembly is complete.
func (r *DefinitionResult) TempDir() string {
	return r.tempDir
}

// Cleanup removes any temporary resources created during package definition loading.
// This should be called after package assembly is complete.
func (r *DefinitionResult) Cleanup() error {
	if r.tempDir != "" {
		return os.RemoveAll(r.tempDir)
	}
	return nil
}

// PackageDefinition returns a validated package definition after flavors, imports, variables, and values are applied.
// The returned DefinitionResult must have its Cleanup method called after package assembly is complete.
func PackageDefinition(ctx context.Context, packagePath string, opts DefinitionOptions) (*DefinitionResult, error) {
	l := logger.From(ctx)
	start := time.Now()
	l.Debug("start layout.LoadPackage",
		"path", packagePath,
		"flavor", opts.Flavor,
		"setVariables", opts.SetVariables,
	)

	pkgPath, err := layout.ResolvePackagePath(packagePath)
	if err != nil {
		return nil, err
	}

	b, err := os.ReadFile(pkgPath.ManifestFile)
	if err != nil {
		return nil, err
	}
	pkg, err := pkgcfg.Parse(ctx, b)
	if err != nil {
		return nil, err
	}
	pkg.Metadata.Architecture = config.GetArch(pkg.Metadata.Architecture)

	var tempDir string
	pkg, err = resolveImports(ctx, pkg, pkgPath.ManifestFile, pkg.Metadata.Architecture, opts.Flavor, []string{}, opts.CachePath, opts.SkipVersionCheck, opts.RemoteOptions, &tempDir)
	if err != nil {
		// Clean up temp directory if it was created
		if tempDir != "" {
			err = errors.Join(err, os.RemoveAll(tempDir))
		}
		return nil, err
	}

	// Helper to clean up temp dir on subsequent errors
	cleanupOnError := func() {
		if tempDir != "" {
			err = errors.Join(err, os.RemoveAll(tempDir))
		}
	}

	if len(pkg.Values.Files) > 0 && !feature.IsEnabled(feature.Values) {
		cleanupOnError()
		return nil, fmt.Errorf("creating package with Values files, but \"%s\" feature is not enabled."+
			" Run again with --features=\"%s=true\"", feature.Values, feature.Values)
	}

	if opts.SetVariables != nil {
		pkg, _, err = fillActiveTemplate(ctx, pkg, opts.SetVariables, opts.IsInteractive)
		if err != nil {
			cleanupOnError()
			return nil, err
		}
	}
	err = validate(ctx, pkg, pkgPath.ManifestFile, opts.SetVariables, opts.Flavor, opts.SkipRequiredValues)
	if err != nil {
		cleanupOnError()
		return nil, err
	}
	l.Debug("done layout.LoadPackage", "duration", time.Since(start))
	return &DefinitionResult{Pkg: pkg, tempDir: tempDir}, nil
}

func validate(ctx context.Context, pkg v1alpha1.ZarfPackage, packagePath string, setVariables map[string]string, flavor string, skipRequiredValues bool) error {
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

	if err := validateValuesSchema(ctx, pkg, packagePath, validateValuesSchemaOptions{skipRequired: skipRequiredValues}); err != nil {
		return err
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

	// Resolve values file paths relative to the package directory (unless already absolute)
	valueFilePaths := make([]string, len(pkg.Values.Files))
	for i, vf := range pkg.Values.Files {
		if filepath.IsAbs(vf) {
			valueFilePaths[i] = vf
		} else {
			valueFilePaths[i] = filepath.Join(pkgPath.BaseDir, vf)
		}
	}

	vals, err := value.ParseFiles(ctx, valueFilePaths, value.ParseFilesOptions{})
	if err != nil {
		return fmt.Errorf("failed to parse values files for validation: %w", err)
	}

	// Resolve declared schema path relative to package root (unless already absolute)
	schemaPath := pkg.Values.Schema
	if !filepath.IsAbs(schemaPath) {
		schemaPath = filepath.Join(pkgPath.BaseDir, pkg.Values.Schema)
	}
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
