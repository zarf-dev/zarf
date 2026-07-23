// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package load

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	goyaml "github.com/goccy/go-yaml"

	"github.com/zarf-dev/zarf/src/api/v1beta1"
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

// resolveComponentSpecImports merges any imported component configs into spec. spec is the override
// (head); the selected imported config is the base. Returned paths are relative to specDir.
func resolveComponentSpecImports(ctx context.Context, spec v1beta1.ComponentSpec, specDir, arch, flavor string, importStack []string) (v1beta1.ComponentSpec, importedValues, error) {
	if err := validateComponentImportV1Beta1(spec.Import); err != nil {
		return v1beta1.ComponentSpec{}, importedValues{}, err
	}
	if len(spec.Import.Local) == 0 {
		return spec, importedValues{}, nil
	}

	selected, err := selectImportVariant(spec.Import.Local, specDir, arch, flavor, importStack)
	if err != nil {
		return v1beta1.ComponentSpec{}, importedValues{}, err
	}

	// Recurse into the selected config's own imports, then rebase its resolved spec to specDir.
	baseSpec, baseVals, err := resolveComponentSpecImports(ctx, selected.config.Component, selected.dir, arch, flavor, append(importStack, selected.path))
	if err != nil {
		return v1beta1.ComponentSpec{}, importedValues{}, err
	}

	relDir := filepath.Dir(selected.entry.Path)
	baseSpec = fixPathsV1Beta1(baseSpec, relDir)

	vals := importedValues{}
	for _, f := range selected.config.Values.Files {
		vals.files = append(vals.files, makePathRelativeTo(f, relDir))
	}
	if selected.config.Values.Schema != "" {
		vals.schemas = append(vals.schemas, makePathRelativeTo(selected.config.Values.Schema, relDir))
	}
	for _, f := range baseVals.files {
		vals.files = append(vals.files, makePathRelativeTo(f, relDir))
	}
	for _, s := range baseVals.schemas {
		vals.schemas = append(vals.schemas, makePathRelativeTo(s, relDir))
	}

	merged := mergeComponentSpec(baseSpec, spec)
	merged.Import = v1beta1.ComponentImport{}
	return merged, vals, nil
}

// loadedComponentConfig pairs a parsed component config with where it was read from.
type loadedComponentConfig struct {
	config v1beta1.ComponentConfig
	entry  v1beta1.ComponentImportLocal
	dir    string
	path   string
}

// selectImportVariant loads every local import entry and selects the single one compatible with the
// active target. A single entry is always selected. When more than one entry is given they are treated
// as variants: exactly one must be compatible with the target.
func selectImportVariant(entries []v1beta1.ComponentImportLocal, specDir, arch, flavor string, importStack []string) (loadedComponentConfig, error) {
	var loaded []loadedComponentConfig
	for _, entry := range entries {
		path := filepath.Clean(filepath.Join(specDir, entry.Path))
		for _, seen := range importStack {
			if seen == path {
				return loadedComponentConfig{}, fmt.Errorf("component config %s imported in cycle", filepath.ToSlash(path))
			}
		}
		config, err := readComponentConfig(path)
		if err != nil {
			return loadedComponentConfig{}, err
		}
		loaded = append(loaded, loadedComponentConfig{config: config, entry: entry, dir: filepath.Dir(path), path: path})
	}

	if len(loaded) == 1 {
		return loaded[0], nil
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

// readComponentConfig reads a ZarfComponentConfig file directly. v1beta1 packages only ever import
// v1beta1 component configs, so the bytes are decoded into the native type without conversion.
func readComponentConfig(path string) (v1beta1.ComponentConfig, error) {
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
	if config.Kind != "" && config.Kind != v1beta1.ZarfComponentConfig {
		return v1beta1.ComponentConfig{}, fmt.Errorf("imported file %q is not a %s", path, v1beta1.ZarfComponentConfig)
	}
	return config, nil
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
