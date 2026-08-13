// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package load

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	goyaml "github.com/goccy/go-yaml"

	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
)

// importedValues collects the values files and schemas declared by imported component configs
// so they can be merged into the package definition once all imports are resolved.
type importedValues struct {
	files   []string
	schemas []string
}

// resolveImportsV1Beta1 resolves local component config imports into a v1beta1 package definition.
// Each package component may import one or more ZarfComponentConfig files; filtering compatible components also happens here
func resolveImportsV1Beta1(ctx context.Context, pkg v1beta1.Package, pkgPath layout.PackagePath, arch, flavor string) (v1beta1.Package, []string, error) {
	l := logger.From(ctx)
	start := time.Now()
	l.Debug("start resolveImportsV1Beta1", "pkg", pkg.Metadata.Name, "arch", arch, "flavor", flavor)

	baseDir := pkgPath.BaseDir

	var components []v1beta1.Component
	var vals importedValues
	for _, component := range pkg.Components {
		if !compatibleComponentV1Beta1(component.Selector, arch, flavor) {
			continue
		}
		mergedSpec, compVals, err := resolveComponentSpecImports(ctx, component.ComponentSpec, baseDir, arch, flavor, []string{filepath.Clean(pkgPath.ManifestFile)})
		if err != nil {
			return v1beta1.Package{}, nil, fmt.Errorf("component %q: %w", component.Name, err)
		}
		component.ComponentSpec = mergedSpec
		components = append(components, component)
		vals.files = append(vals.files, compVals.files...)
		vals.schemas = append(vals.schemas, compVals.schemas...)
	}
	pkg.Components = components

	// Imported value files come first so the package's own files take precedence (later files win).
	valuesFiles := append(vals.files, pkg.Values.Files...)
	pkg.Values.Files = dedupePaths(valuesFiles)

	l.Debug("done resolveImportsV1Beta1", "pkg", pkg.Metadata.Name, "components", len(pkg.Components), "duration", time.Since(start))
	return pkg, dedupePaths(vals.schemas), nil
}

// ResolveComponentConfigImports resolves local imports in a v1beta1 component config.
func ResolveComponentConfigImports(ctx context.Context, component v1beta1.ComponentConfig, componentPath, arch, flavor string) (v1beta1.ComponentConfig, error) {
	componentPath = filepath.Clean(componentPath)
	resolvedSpec, importedVals, err := resolveComponentSpecImports(ctx, component.Component, filepath.Dir(componentPath), arch, flavor, []string{componentPath})
	if err != nil {
		return v1beta1.ComponentConfig{}, err
	}
	component.Component = resolvedSpec
	component.Values.Files = dedupePaths(append(importedVals.files, component.Values.Files...))
	if component.Values.Schema == "" && len(importedVals.schemas) > 0 {
		component.Values.Schema = importedVals.schemas[0]
	}
	return component, nil
}

// resolveComponentSpecImports merges component configs imported by spec. The returned spec and
// values paths are relative to specDir.
func resolveComponentSpecImports(ctx context.Context, spec v1beta1.ComponentSpec, specDir, arch, flavor string, importStack []string) (v1beta1.ComponentSpec, importedValues, error) {
	if err := validateComponentImportV1Beta1(spec.Import); err != nil {
		return v1beta1.ComponentSpec{}, importedValues{}, err
	}
	if len(spec.Import.Local) == 0 {
		// End of this import chain: there are no deeper imported values to inherit.
		return spec, importedValues{}, nil
	}

	directImport, err := selectImportVariant(spec.Import.Local, specDir, arch, flavor, importStack)
	if err != nil {
		return v1beta1.ComponentSpec{}, importedValues{}, err
	}

	resolvedImportSpec, inheritedValues, err := resolveComponentSpecImports(ctx, directImport.config.Component, directImport.dir, arch, flavor, append(importStack, directImport.path))
	if err != nil {
		return v1beta1.ComponentSpec{}, importedValues{}, err
	}

	relDir := filepath.Dir(directImport.entry.Path)
	resolvedImportSpec = fixPathsV1Beta1(resolvedImportSpec, relDir)

	vals := mergeImportedValues(directImport.config.Values, inheritedValues, relDir)

	merged := mergeComponentSpec(resolvedImportSpec, spec)
	merged.Import = v1beta1.ComponentImport{}
	return merged, vals, nil
}

// mergeImportedValues preserves each merge contract: values files are later-wins, schemas are earlier-wins.
func mergeImportedValues(directValues v1beta1.Values, inherited importedValues, relDir string) importedValues {
	vals := importedValues{}
	for _, f := range inherited.files {
		vals.files = append(vals.files, makePathRelativeTo(f, relDir))
	}
	for _, f := range directValues.Files {
		vals.files = append(vals.files, makePathRelativeTo(f, relDir))
	}
	if directValues.Schema != "" {
		vals.schemas = append(vals.schemas, makePathRelativeTo(directValues.Schema, relDir))
	}
	for _, s := range inherited.schemas {
		vals.schemas = append(vals.schemas, makePathRelativeTo(s, relDir))
	}
	return vals
}

// loadedComponentConfig pairs a parsed component config with where it was read from.
type loadedComponentConfig struct {
	config v1beta1.ComponentConfig
	entry  v1beta1.ComponentImportLocal
	dir    string
	path   string
}

// selectImportVariant loads every local import entry and selects the single one compatible with the
// active target. Entries are treated as variants: exactly one must be compatible with the target.
func selectImportVariant(entries []v1beta1.ComponentImportLocal, specDir, arch, flavor string, importStack []string) (loadedComponentConfig, error) {
	var loaded []loadedComponentConfig
	for _, entry := range entries {
		path := filepath.Clean(filepath.Join(specDir, entry.Path))
		if slices.Contains(importStack, path) {
			return loadedComponentConfig{}, fmt.Errorf("component config %s imported in cycle", filepath.ToSlash(path))
		}
		config, err := ComponentConfig(path)
		if err != nil {
			return loadedComponentConfig{}, err
		}
		loaded = append(loaded, loadedComponentConfig{config: config, entry: entry, dir: filepath.Dir(path), path: path})
	}

	var compatible []loadedComponentConfig
	for _, lc := range loaded {
		if compatibleComponentV1Beta1(lc.config.Component.Selector, arch, flavor) {
			compatible = append(compatible, lc)
		}
	}
	switch len(compatible) {
	case 0:
		return loadedComponentConfig{}, fmt.Errorf("no imported component variant is compatible with the package target")
	case 1:
		return compatible[0], nil
	default:
		return loadedComponentConfig{}, fmt.Errorf("multiple imported component variants are compatible with the package target")
	}
}

// ComponentConfig reads and schema-validates a v1beta1 ZarfComponentConfig file.
// FIXME: this should be a generic function in load
func ComponentConfig(path string) (v1beta1.ComponentConfig, error) {
	info, err := os.Stat(path)
	if err != nil {
		return v1beta1.ComponentConfig{}, fmt.Errorf("unable to access imported component config %q: %w", path, err)
	}
	if info.IsDir() {
		return v1beta1.ComponentConfig{}, fmt.Errorf("import path %q is a directory; v1beta1 imports must reference a component config file", path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return v1beta1.ComponentConfig{}, err
	}
	var config v1beta1.ComponentConfig
	if err := goyaml.Unmarshal(b, &config); err != nil {
		return v1beta1.ComponentConfig{}, fmt.Errorf("unable to parse imported component config %q: %w", path, err)
	}
	if config.Kind != v1beta1.ZarfComponentConfig {
		return v1beta1.ComponentConfig{}, fmt.Errorf("v1beta1 import %q must be a %s; components cannot be imported from packages", path, v1beta1.ZarfComponentConfig)
	}
	if err := validateComponentConfigSchemaV1Beta1(path, b); err != nil {
		return v1beta1.ComponentConfig{}, err
	}
	return config, nil
}

func validateComponentConfigSchemaV1Beta1(path string, b []byte) error {
	findings, err := lint.ValidateComponentConfigSchemaBytesV1Beta1(b)
	if err != nil {
		return fmt.Errorf("unable to check imported component config schema %q: %w", path, err)
	}
	if len(findings) == 0 {
		return nil
	}
	return &lint.LintError{
		PackageName: path,
		Findings:    findings,
	}
}

func validateComponentImportV1Beta1(imp v1beta1.ComponentImport) error {
	if len(imp.Remote) > 0 {
		return fmt.Errorf("remote component imports are not yet supported for v1beta1 packages")
	}
	for _, l := range imp.Local {
		if l.Path == "" {
			return fmt.Errorf("import entry is missing a path")
		}
		if filepath.IsAbs(l.Path) {
			return fmt.Errorf("import path %q cannot be absolute", l.Path)
		}
	}
	return nil
}

// compatibleComponentV1Beta1 reports whether a component target matches the active architecture and flavor.
// OS targeting is a deploy-time filter and is not evaluated here.
func compatibleComponentV1Beta1(selector v1beta1.ComponentSelector, arch, flavor string) bool {
	satisfiesArch := selector.Architecture == "" || selector.Architecture == arch
	satisfiesFlavor := selector.Flavor == "" || selector.Flavor == flavor
	return satisfiesArch && satisfiesFlavor
}

func dedupePaths(paths []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range paths {
		norm := makePathRelativeTo(p, ".")
		if seen[norm] {
			continue
		}
		seen[norm] = true
		out = append(out, norm)
	}
	return out
}
