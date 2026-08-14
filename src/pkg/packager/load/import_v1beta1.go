// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package load

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/defenseunicorns/pkg/oci"
	goyaml "github.com/goccy/go-yaml"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
	"github.com/zarf-dev/zarf/src/types"
)

// FIXME: These constants should be shared
const componentConfigMediaType = "application/vnd.zarf.component.config.v1+json"

const (
	componentResourceMountPathAnnotation = "dev.zarf.mountPath"
	componentResourceKindAnnotation      = "dev.zarf.resourceKind"
)

// remoteResource is a blob needed by a remotely imported component. Its import
// and mount paths are artifact-relative, never source filesystem paths.
type remoteResource struct {
	remote     *zoci.Remote
	descriptor ocispec.Descriptor
	importRoot string
	mountPath  string
}

// importedValues collects the values files and schemas declared by imported component configs
// so they can be merged into the package definition once all imports are resolved.
type importedValues struct {
	files   []string
	schemas []string
}

// resolveImportsV1Beta1 resolves component config imports into a v1beta1 package definition.
// Each package component may import one or more ZarfComponentConfig files; filtering compatible components also happens here
// FIXME: no point to this separation
func resolveImportsV1Beta1(ctx context.Context, pkg v1beta1.Package, pkgPath layout.PackagePath, arch, flavor string) (v1beta1.Package, []string, error) {
	pkg, schemas, _, err := resolveImportsV1Beta1WithRemote(ctx, pkg, pkgPath, arch, flavor, types.RemoteOptions{}, "")
	return pkg, schemas, err
}

func resolveImportsV1Beta1WithRemote(ctx context.Context, pkg v1beta1.Package, pkgPath layout.PackagePath, arch, flavor string, remoteOptions types.RemoteOptions, cachePath string) (v1beta1.Package, []string, []remoteResource, error) {
	l := logger.From(ctx)
	start := time.Now()
	l.Debug("start resolveImportsV1Beta1", "pkg", pkg.Metadata.Name, "arch", arch, "flavor", flavor)

	baseDir := pkgPath.BaseDir

	var components []v1beta1.Component
	var vals importedValues
	var resources []remoteResource
	for _, component := range pkg.Components {
		if !compatibleComponentV1Beta1(component.Selector, arch, flavor) {
			continue
		}
		mergedSpec, compVals, compResources, err := resolveComponentSpecImports(ctx, component.ComponentSpec, baseDir, arch, component.Selector.Architecture, flavor, []string{filepath.Clean(pkgPath.ManifestFile)}, true, remoteOptions, cachePath)
		if err != nil {
			return v1beta1.Package{}, nil, nil, fmt.Errorf("component %q: %w", component.Name, err)
		}
		component.ComponentSpec = mergedSpec
		components = append(components, component)
		vals.files = append(vals.files, compVals.files...)
		vals.schemas = append(vals.schemas, compVals.schemas...)
		resources = append(resources, compResources...)
	}
	pkg.Components = components

	// Imported value files come first so the package's own files take precedence (later files win).
	valuesFiles := append(vals.files, pkg.Values.Files...)
	pkg.Values.Files = dedupePaths(valuesFiles)

	l.Debug("done resolveImportsV1Beta1", "pkg", pkg.Metadata.Name, "components", len(pkg.Components), "duration", time.Since(start))
	return pkg, dedupePaths(vals.schemas), resources, nil
}

// ResolveComponentConfigImports resolves local imports in a v1beta1 component config.
func ResolveComponentConfigImports(ctx context.Context, component v1beta1.ComponentConfig, componentPath, arch, flavor string) (v1beta1.ComponentConfig, error) {
	componentPath = filepath.Clean(componentPath)
	resolvedSpec, importedVals, _, err := resolveComponentSpecImports(ctx, component.Component, filepath.Dir(componentPath), arch, component.Component.Selector.Architecture, flavor, []string{componentPath}, false, types.RemoteOptions{}, "")
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
func resolveComponentSpecImports(ctx context.Context, spec v1beta1.ComponentSpec, specDir, arch, remoteArch, flavor string, importStack []string, allowRemote bool, remoteOptions types.RemoteOptions, cachePath string) (v1beta1.ComponentSpec, importedValues, []remoteResource, error) {
	if err := validateComponentImportV1Beta1(spec.Import, allowRemote); err != nil {
		return v1beta1.ComponentSpec{}, importedValues{}, nil, err
	}
	if len(spec.Import.Local) == 0 && len(spec.Import.Remote) == 0 {
		// End of this import chain: there are no deeper imported values to inherit.
		return spec, importedValues{}, nil, nil
	}

	directImport, err := selectImportVariant(ctx, spec.Import, specDir, arch, remoteArch, flavor, importStack, remoteOptions, cachePath)
	if err != nil {
		return v1beta1.ComponentSpec{}, importedValues{}, nil, err
	}

	resolvedImportSpec, inheritedValues, inheritedResources, err := resolveComponentSpecImports(ctx, directImport.config.Component, directImport.dir, arch, arch, flavor, append(importStack, directImport.path), false, remoteOptions, cachePath)
	if err != nil {
		return v1beta1.ComponentSpec{}, importedValues{}, nil, err
	}

	relDir := directImport.relativeToParent
	resolvedImportSpec = fixPathsV1Beta1(resolvedImportSpec, relDir)

	vals := mergeImportedValues(directImport.config.Values, inheritedValues, relDir)

	merged := mergeComponentSpec(resolvedImportSpec, spec)
	merged.Import = v1beta1.ComponentImport{}
	return merged, vals, append(directImport.resources, inheritedResources...), nil
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
	config           v1beta1.ComponentConfig
	dir              string
	relativeToParent string
	path             string
	resources        []remoteResource
	remote           bool
}

// selectImportVariant loads every local import entry and selects the single one compatible with the
// active target. Entries are treated as variants: exactly one must be compatible with the target.
func selectImportVariant(ctx context.Context, imp v1beta1.ComponentImport, specDir, arch, remoteArch, flavor string, importStack []string, remoteOptions types.RemoteOptions, cachePath string) (loadedComponentConfig, error) {
	var loaded []loadedComponentConfig
	for _, entry := range imp.Local {
		path := filepath.Clean(filepath.Join(specDir, entry.Path))
		if slices.Contains(importStack, path) {
			return loadedComponentConfig{}, fmt.Errorf("component config %s imported in cycle", filepath.ToSlash(path))
		}
		config, err := ComponentConfig(path)
		if err != nil {
			return loadedComponentConfig{}, err
		}
		loaded = append(loaded, loadedComponentConfig{config: config, dir: filepath.Dir(path), relativeToParent: filepath.Dir(entry.Path), path: path})
	}
	for _, entry := range imp.Remote {
		loadedComponent, err := remoteComponentConfig(ctx, entry.URL, remoteOptions, cachePath)
		if err != nil {
			return loadedComponentConfig{}, err
		}
		if slices.Contains(importStack, loadedComponent.path) {
			return loadedComponentConfig{}, fmt.Errorf("component config %s imported in cycle", loadedComponent.path)
		}
		loadedComponent.remote = true
		loaded = append(loaded, loadedComponent)
	}

	var compatible []loadedComponentConfig
	for _, lc := range loaded {
		candidateArch := arch
		if lc.remote {
			candidateArch = remoteArch
		}
		if compatibleComponentV1Beta1(lc.config.Component.Selector, candidateArch, flavor) {
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

func remoteComponentConfig(ctx context.Context, importURL string, remoteOptions types.RemoteOptions, cachePath string) (loadedComponentConfig, error) {
	plainHTTP, err := negotiateImportPlainHTTP(ctx, importURL, remoteOptions)
	if err != nil {
		return loadedComponentConfig{}, err
	}
	mods := []oci.Modifier{oci.WithPlainHTTP(plainHTTP), oci.WithInsecureSkipVerify(remoteOptions.InsecureSkipTLSVerify)}
	if cachePath != "" {
		cacheModifier, err := zoci.GetOCICacheModifier(ctx, cachePath)
		if err != nil {
			return loadedComponentConfig{}, err
		}
		mods = append(mods, cacheModifier)
	}
	remote, err := zoci.NewRemote(ctx, importURL, ocispec.Platform{}, mods...)
	if err != nil {
		return loadedComponentConfig{}, err
	}
	root, err := remote.ResolveRoot(ctx)
	if err != nil {
		return loadedComponentConfig{}, err
	}
	manifest, err := remote.FetchRoot(ctx)
	if err != nil {
		return loadedComponentConfig{}, err
	}
	if manifest.Config.MediaType != componentConfigMediaType {
		return loadedComponentConfig{}, fmt.Errorf("remote import %q is not a v1beta1 component artifact", importURL)
	}
	configBytes, err := remote.FetchLayer(ctx, manifest.Config)
	if err != nil {
		return loadedComponentConfig{}, err
	}
	config, err := componentConfigFromBytes(importURL, configBytes)
	if err != nil {
		return loadedComponentConfig{}, err
	}
	if len(config.Component.Import.Local) > 0 || len(config.Component.Import.Remote) > 0 {
		return loadedComponentConfig{}, fmt.Errorf("remote component imports are not yet supported")
	}
	if len(config.Component.ImageArchives) > 0 {
		return loadedComponentConfig{}, fmt.Errorf("remote component image archives are not yet supported")
	}
	importRoot := path.Join(".zarf", "remote-components", strings.ReplaceAll(root.Digest.String(), ":", "-"))
	resources := make([]remoteResource, 0, len(manifest.Layers))
	seenMountPaths := make(map[string]struct{}, len(manifest.Layers))
	for _, descriptor := range manifest.Layers {
		mountPath := descriptor.Annotations[componentResourceMountPathAnnotation]
		if !validRemoteMountPath(mountPath) || descriptor.Annotations[componentResourceKindAnnotation] == "" {
			return loadedComponentConfig{}, fmt.Errorf("remote component %q has an invalid resource layer", importURL)
		}
		if _, exists := seenMountPaths[mountPath]; exists {
			return loadedComponentConfig{}, fmt.Errorf("remote component %q has duplicate resource layers", importURL)
		}
		seenMountPaths[mountPath] = struct{}{}
		resources = append(resources, remoteResource{remote: remote, descriptor: descriptor, importRoot: importRoot, mountPath: mountPath})
	}
	return loadedComponentConfig{config: config, dir: importRoot, relativeToParent: importRoot, path: importURL + "@" + root.Digest.String(), resources: resources}, nil
}

func validRemoteMountPath(mountPath string) bool {
	return mountPath != "" && !path.IsAbs(mountPath) && path.Clean(mountPath) == mountPath && mountPath != "." && !strings.HasPrefix(mountPath, "../") && !strings.Contains(mountPath, "/../")
}

// ComponentConfig reads and schema-validates a v1beta1 ZarfComponentConfig file.
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
	return componentConfigFromBytes(path, b)
}

func componentConfigFromBytes(path string, b []byte) (v1beta1.ComponentConfig, error) {
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

func validateComponentImportV1Beta1(imp v1beta1.ComponentImport, allowRemote bool) error {
	if !allowRemote && len(imp.Remote) > 0 {
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
