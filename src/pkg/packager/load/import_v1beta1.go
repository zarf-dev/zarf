// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package load

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/defenseunicorns/pkg/helpers/v2"
	goyaml "github.com/goccy/go-yaml"
	"oras.land/oras-go/v2/registry"

	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/component"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/types"
)

// importedValues collects the values files and schemas declared by imported component configs
// so they can be merged into the package definition once all imports are resolved.
type importedValues struct {
	files   []string
	schemas []string
}

var componentPull = component.Pull

// resolveImportsV1Beta1 resolves component config imports into a v1beta1 package definition.
// Each package component may import one or more ZarfComponentConfig files; filtering compatible components also happens here.
func resolveImportsV1Beta1(ctx context.Context, pkg v1beta1.Package, pkgPath layout.PackagePath, arch, flavor, cachePath string, remoteOptions types.RemoteOptions) (v1beta1.Package, []string, error) {
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
		mergedSpec, compVals, err := resolveComponentSpecImports(ctx, component.ComponentSpec, baseDir, arch, flavor, []string{filepath.Clean(pkgPath.ManifestFile)}, cachePath, remoteOptions)
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
func resolveComponentSpecImports(ctx context.Context, spec v1beta1.ComponentSpec, specDir, arch, flavor string, importStack []string, cachePath string, remoteOptions types.RemoteOptions) (v1beta1.ComponentSpec, importedValues, error) {
	if err := validateComponentImportV1Beta1(spec.Import); err != nil {
		return v1beta1.ComponentSpec{}, importedValues{}, err
	}
	if len(spec.Import.Local) == 0 && len(spec.Import.Remote) == 0 {
		return spec, importedValues{}, nil
	}

	selected, err := selectImportVariant(ctx, spec.Import, specDir, arch, flavor, importStack, cachePath, remoteOptions)
	if err != nil {
		return v1beta1.ComponentSpec{}, importedValues{}, err
	}

	baseSpec, baseVals, err := resolveComponentSpecImports(ctx, selected.config.Component, selected.dir, arch, flavor, append(importStack, selected.path), cachePath, remoteOptions)
	if err != nil {
		return v1beta1.ComponentSpec{}, importedValues{}, err
	}

	baseSpec = fixPathsV1Beta1(baseSpec, selected.rebaseDir)

	vals := importedValues{}
	for _, f := range selected.config.Values.Files {
		vals.files = append(vals.files, makePathRelativeTo(f, selected.rebaseDir))
	}
	if selected.config.Values.Schema != "" {
		vals.schemas = append(vals.schemas, makePathRelativeTo(selected.config.Values.Schema, selected.rebaseDir))
	}
	for _, f := range baseVals.files {
		vals.files = append(vals.files, makePathRelativeTo(f, selected.rebaseDir))
	}
	for _, s := range baseVals.schemas {
		vals.schemas = append(vals.schemas, makePathRelativeTo(s, selected.rebaseDir))
	}

	merged := mergeComponentSpec(baseSpec, spec)
	merged.Import = v1beta1.ComponentImport{}
	return merged, vals, nil
}

// loadedComponentConfig pairs a parsed component config with where it was read from.
type loadedComponentConfig struct {
	config    v1beta1.ComponentConfig
	dir       string
	path      string
	rebaseDir string
	// FIXME: evaluate this
	cleanup func() error
}

func noopCleanup() error {
	return nil
}

// selectImportVariant loads every import entry and selects the single one compatible with the
// active target. A single entry is always selected. When more than one entry is given they are treated
// as variants: exactly one must be compatible with the target.
func selectImportVariant(ctx context.Context, imp v1beta1.ComponentImport, specDir, arch, flavor string, importStack []string, cachePath string, remoteOptions types.RemoteOptions) (selected loadedComponentConfig, err error) {
	var loaded []loadedComponentConfig
	defer func() {
		if err != nil {
			for _, lc := range loaded {
				_ = lc.cleanup()
			}
		}
	}()

	for _, entry := range imp.Local {
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
		loaded = append(loaded, loadedComponentConfig{
			config:    config,
			dir:       filepath.Dir(path),
			path:      path,
			rebaseDir: filepath.Dir(entry.Path),
			cleanup:   noopCleanup,
		})
	}
	for _, entry := range imp.Remote {
		lc, err := loadRemoteComponentConfig(ctx, entry, specDir, importStack, cachePath, remoteOptions)
		if err != nil {
			return loadedComponentConfig{}, err
		}
		loaded = append(loaded, lc)
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
		selected = compatible[0]
		for _, lc := range loaded {
			if lc.path != selected.path {
				_ = lc.cleanup()
			}
		}
		return selected, nil
	default:
		return loadedComponentConfig{}, fmt.Errorf("multiple imported component variants are compatible with the package target")
	}
}

func loadRemoteComponentConfig(ctx context.Context, entry v1beta1.ComponentImportRemote, specDir string, importStack []string, cachePath string, remoteOptions types.RemoteOptions) (loadedComponentConfig, error) {
	ref, err := parseRemoteComponentImportURL(entry.URL)
	if err != nil {
		return loadedComponentConfig{}, err
	}
	path := ref.String()
	for _, seen := range importStack {
		if seen == path {
			return loadedComponentConfig{}, fmt.Errorf("component config %s imported in cycle", path)
		}
	}

	localPath, cleanup, err := componentPull(ctx, ref, component.PullOptions{
		CachePath:     cachePath,
		RemoteOptions: remoteOptions,
	})
	if err != nil {
		return loadedComponentConfig{}, err
	}
	if cleanup == nil {
		cleanup = noopCleanup
	}
	config, err := readComponentConfig(localPath)
	if err != nil {
		_ = cleanup()
		return loadedComponentConfig{}, err
	}
	pulledDir := filepath.Dir(localPath)
	stableDir, err := stableRemoteComponentDir(cachePath, path)
	if err != nil {
		_ = cleanup()
		return loadedComponentConfig{}, err
	}
	if err := copyDir(pulledDir, stableDir); err != nil {
		return loadedComponentConfig{}, errors.Join(err, cleanup())
	}
	if err := cleanup(); err != nil {
		return loadedComponentConfig{}, err
	}
	rebaseDir, err := filepath.Rel(specDir, stableDir)
	if err != nil {
		return loadedComponentConfig{}, err
	}
	return loadedComponentConfig{
		config:    config,
		dir:       stableDir,
		path:      path,
		rebaseDir: rebaseDir,
		cleanup:   noopCleanup,
	}, nil
}

func parseRemoteComponentImportURL(rawURL string) (registry.Reference, error) {
	if rawURL == "" {
		return registry.Reference{}, fmt.Errorf("remote import entry is missing a url")
	}
	if !strings.HasPrefix(rawURL, helpers.OCIURLPrefix) {
		return registry.Reference{}, fmt.Errorf("remote import url %q must start with %s", rawURL, helpers.OCIURLPrefix)
	}
	ref, err := registry.ParseReference(strings.TrimPrefix(rawURL, helpers.OCIURLPrefix))
	if err != nil {
		return registry.Reference{}, fmt.Errorf("unable to parse remote import url %q: %w", rawURL, err)
	}
	if ref.Reference == "" {
		return registry.Reference{}, fmt.Errorf("remote import url %q must include a tag or digest", rawURL)
	}
	return ref, nil
}

func remoteComponentImportCacheDir(cachePath, ref string) string {
	if cachePath == "" {
		cachePath = os.TempDir()
	}
	sum := sha256.Sum256([]byte(ref))
	return filepath.Join(cachePath, "component-imports", fmt.Sprintf("%x", sum[:]))
}

func stableRemoteComponentDir(cachePath, ref string) (string, error) {
	dir := remoteComponentImportCacheDir(cachePath, ref)
	if err := os.RemoveAll(dir); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return err
		}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(srcPath)
			if err != nil {
				return err
			}
			if err := os.Symlink(target, dstPath); err != nil {
				return err
			}
		case entry.IsDir():
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		case info.Mode().IsRegular():
			if err := copyFile(srcPath, dstPath, info.Mode()); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		_ = in.Close()
	}()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	return errors.Join(copyErr, closeErr)
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
