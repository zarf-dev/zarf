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

type v1beta1ImportResolution struct {
	pkg             v1beta1.Package
	schemas         []string
	remoteResources []remoteResource
}

// ComponentConfigImportResolution contains a component config with imports resolved and
// the imported values schema paths needed to preserve schema precedence during publication.
type ComponentConfigImportResolution struct {
	Component       v1beta1.ComponentConfig
	ImportedSchemas []string
	remoteResources []remoteResource
}

// resolveImportsV1Beta1 resolves component config imports into a v1beta1 package definition.
// Each package component may import one or more ZarfComponentConfig files; filtering compatible components also happens here.
func resolveImportsV1Beta1(ctx context.Context, pkg v1beta1.Package, pkgPath layout.PackagePath, arch, flavor string, remoteOptions types.RemoteOptions, cachePath string) (v1beta1ImportResolution, error) {
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
		mergedSpec, compVals, compResources, err := resolveComponentConfigSpecImports(ctx, component.ComponentSpec, baseDir, arch, flavor, []string{filepath.Clean(pkgPath.ManifestFile)}, remoteOptions, cachePath)
		if err != nil {
			return v1beta1ImportResolution{}, fmt.Errorf("component %q: %w", component.Name, err)
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
	return v1beta1ImportResolution{
		pkg:             pkg,
		schemas:         dedupePaths(vals.schemas),
		remoteResources: resources,
	}, nil
}

// ResolveComponentConfigImports resolves imports in a v1beta1 component config using
// the supplied registry options for remote component imports.
func ResolveComponentConfigImports(ctx context.Context, component v1beta1.ComponentConfig, componentPath string, remoteOptions types.RemoteOptions) (ComponentConfigImportResolution, error) {
	componentPath = filepath.Clean(componentPath)
	resolvedSpec, importedVals, remoteResources, err := resolveComponentConfigSpecImports(ctx, component.Component, filepath.Dir(componentPath), component.Metadata.Variant.Architecture, component.Metadata.Variant.Flavor, []string{componentPath}, remoteOptions, "")
	if err != nil {
		return ComponentConfigImportResolution{}, err
	}
	component.Component = resolvedSpec
	component.Values.Files = dedupePaths(append(importedVals.files, component.Values.Files...))
	return ComponentConfigImportResolution{
		Component:       component,
		ImportedSchemas: dedupePaths(importedVals.schemas),
		remoteResources: remoteResources,
	}, nil
}

// MaterializeResources makes remote component resources available on the filesystem while
// publishing a resolved component config. The caller must close the returned resource set.
func (r ComponentConfigImportResolution) MaterializeResources(ctx context.Context, componentPath string) (*ResourceSet, error) {
	return materializeResources(ctx, filepath.Dir(componentPath), r.remoteResources)
}

// resolveComponentConfigSpecImports merges component-config imports. Its target always
// comes from the root component config metadata, never a package-create override.
func resolveComponentConfigSpecImports(ctx context.Context, spec v1beta1.ComponentSpec, specDir, arch, flavor string, importStack []string, remoteOptions types.RemoteOptions, cachePath string) (v1beta1.ComponentSpec, importedValues, []remoteResource, error) {
	if err := validateComponentImportV1Beta1(spec.Import); err != nil {
		return v1beta1.ComponentSpec{}, importedValues{}, nil, err
	}
	// TODO, when resolving a remote component make sure that any maliciously crafted component configs will error
	if len(spec.Import.Local) == 0 && len(spec.Import.Remote) == 0 {
		// End of this import chain: there are no deeper imported values to inherit.
		return spec, importedValues{}, nil, nil
	}

	directImport, err := selectImportVariant(ctx, spec.Import, specDir, arch, flavor, importStack, remoteOptions, cachePath)
	if err != nil {
		return v1beta1.ComponentSpec{}, importedValues{}, nil, err
	}
	resolvedImportSpec, inheritedValues, inheritedResources, err := resolveComponentConfigSpecImports(ctx, directImport.config.Component, directImport.dir, arch, flavor, append(importStack, directImport.path), remoteOptions, cachePath)
	if err != nil {
		return v1beta1.ComponentSpec{}, importedValues{}, nil, err
	}

	relDir := directImport.relativeToParent
	resolvedImportSpec = fixPathsV1Beta1(resolvedImportSpec, relDir)
	vals := mergeImportedValues(directImport.config.Values, inheritedValues, relDir)
	for i := range inheritedResources {
		inheritedResources[i].importRoot = makePathRelativeTo(inheritedResources[i].importRoot, relDir)
	}
	resources := append(directImport.resources, inheritedResources...)
	merged := mergeComponentConfigSpec(resolvedImportSpec, spec)
	merged.Import = v1beta1.ComponentImport{}
	return merged, vals, resources, nil
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
}

// selectImportVariant loads every local import entry and selects the single one compatible with the
// active target. Entries are treated as variants: exactly one must be compatible with the target.
func selectImportVariant(ctx context.Context, imp v1beta1.ComponentImport, specDir, arch, flavor string, importStack []string, remoteOptions types.RemoteOptions, cachePath string) (loadedComponentConfig, error) {
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
		loadedComponent, err := remoteComponentConfig(ctx, entry.URL, arch, remoteOptions, cachePath)
		if err != nil {
			return loadedComponentConfig{}, err
		}
		if slices.Contains(importStack, loadedComponent.path) {
			return loadedComponentConfig{}, fmt.Errorf("component config %s imported in cycle", loadedComponent.path)
		}
		loaded = append(loaded, loadedComponent)
	}

	var compatible []loadedComponentConfig
	for _, lc := range loaded {
		if compatibleComponentConfigV1Beta1(lc.config.Metadata, arch, flavor) {
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

func remoteComponentConfig(ctx context.Context, importURL, arch string, remoteOptions types.RemoteOptions, cachePath string) (loadedComponentConfig, error) {
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
	remote, err := zoci.NewRemote(ctx, importURL, ocispec.Platform{Architecture: arch}, mods...)
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
	if manifest.Config.MediaType != layout.ZarfComponentConfigMediaType {
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
	if !metadataMatchesOCIPlatform(config.Metadata, root.Platform) {
		return loadedComponentConfig{}, fmt.Errorf("remote component %q metadata architecture does not match its OCI platform", importURL)
	}
	importRoot := path.Join(".zarf", "remote-components", strings.ReplaceAll(root.Digest.String(), ":", "-"))
	resources := make([]remoteResource, 0, len(manifest.Layers))
	seenMountPaths := make(map[string]struct{}, len(manifest.Layers))
	for _, descriptor := range manifest.Layers {
		// An import with only remote resources may have no layers and oras will then create this fake layer
		if descriptor.MediaType == ocispec.MediaTypeEmptyJSON {
			continue
		}
		mountPath := descriptor.Annotations[layout.ComponentResourceMountPathAnnotation]
		if !validRemoteMountPath(mountPath) {
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

// compatibleComponentConfigV1Beta1 reports whether a component config's metadata variant matches
// the active package-create target. Empty variant fields are generic.
func compatibleComponentConfigV1Beta1(metadata v1beta1.ComponentMetadata, arch, flavor string) bool {
	return (metadata.Variant.Architecture == "" || metadata.Variant.Architecture == arch) && (metadata.Variant.Flavor == "" || metadata.Variant.Flavor == flavor)
}

func metadataMatchesOCIPlatform(metadata v1beta1.ComponentMetadata, platform *ocispec.Platform) bool {
	if platform == nil {
		return metadata.Variant.Architecture == ""
	}
	return metadata.Variant.Architecture == platform.Architecture
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
