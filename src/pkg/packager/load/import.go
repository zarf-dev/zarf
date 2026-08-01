// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package load

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mholt/archives"
	pkgvalidate "github.com/zarf-dev/zarf/src/internal/packager/requirements"
	"github.com/zarf-dev/zarf/src/internal/pkgcfg"
	"github.com/zarf-dev/zarf/src/pkg/archive"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/ocischeme"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/value"
	"github.com/zarf-dev/zarf/src/types"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/pkg/oci"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	ocistore "oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/registry"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
)

const maxOCISkeletonValuesLayerSize = 1 << 20

type importResources struct {
	values  []value.Source
	schemas []value.Source
}

func (r *importResources) resolve(ctx context.Context, schemaOutputPath string) (ResolvedValues, error) {
	values := deduplicateSources(r.values)
	schemas := deduplicateSources(r.schemas)
	resolved := ResolvedValues{HasValues: len(values) > 0, SchemaOutputPath: schemaOutputPath}
	if resolved.HasValues {
		parsed, err := value.ParseSources(ctx, values)
		if err != nil {
			return ResolvedValues{}, err
		}
		resolved.Values = parsed
	}
	if len(schemas) == 0 {
		return resolved, nil
	}
	if len(schemas) == 1 {
		if _, err := value.LoadValidatedSchemaSource(schemas[0]); err != nil {
			return ResolvedValues{}, err
		}
		resolved.Schema = append([]byte(nil), schemas[0].Data...)
		return resolved, nil
	}
	merged, err := value.MergeSchemaSources(schemas)
	if err != nil {
		return ResolvedValues{}, err
	}
	if err := value.ValidateSchemaDocument(merged); err != nil {
		return ResolvedValues{}, err
	}
	b, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return ResolvedValues{}, err
	}
	resolved.Schema = b
	return resolved, nil
}

func deduplicateSources(sources []value.Source) []value.Source {
	seen := map[string]bool{}
	result := make([]value.Source, 0, len(sources))
	for _, source := range sources {
		key := source.Key
		if key == "" {
			key = source.Name
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, source)
	}
	return result
}

func readSource(basePath, path string) (value.Source, error) {
	absPath := path
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(basePath, path)
	}
	b, err := os.ReadFile(absPath)
	if err != nil {
		return value.Source{}, fmt.Errorf("reading %q: %w", path, err)
	}
	return value.Source{Name: filepath.ToSlash(path), Key: filepath.Clean(absPath), Data: b}, nil
}

func readValueSource(basePath, path string) (value.Source, bool, error) {
	source, err := readSource(basePath, path)
	if errors.Is(err, os.ErrNotExist) {
		return value.Source{}, false, nil
	}
	if err != nil {
		return value.Source{}, false, err
	}
	return source, true, nil
}

// negotiateImportPlainHTTP decides the transport scheme for an `import: url:` OCI reference.
func negotiateImportPlainHTTP(ctx context.Context, importURL string, remoteOptions types.RemoteOptions) (bool, error) {
	if !remoteOptions.PlainHTTP {
		return false, nil
	}
	ref, err := registry.ParseReference(strings.TrimPrefix(importURL, helpers.OCIURLPrefix))
	if err != nil {
		return false, fmt.Errorf("unable to parse import url %q: %w", importURL, err)
	}
	plainHTTP, err := ocischeme.From(ctx).UsePlainHTTP(ctx, ref.Registry, ocischeme.ProbeOptions{InsecureSkipTLSVerify: remoteOptions.InsecureSkipTLSVerify})
	if err != nil {
		return false, fmt.Errorf("unable to resolve import %q: %w", importURL, err)
	}
	return plainHTTP, nil
}

func getComponentToImportName(component v1alpha1.ZarfComponent) string {
	if component.Import.Name != "" {
		return component.Import.Name
	}
	return component.Name
}

func resolveImports(ctx context.Context, pkg v1alpha1.ZarfPackage, packagePath, arch, flavor string, importStack []string, cachePath string, skipVersionCheck bool, remoteOptions types.RemoteOptions, resources *importResources) (v1alpha1.ZarfPackage, error) {
	l := logger.From(ctx)
	start := time.Now()

	pkgPath, err := layout.ResolvePackagePath(packagePath)
	if err != nil {
		return v1alpha1.ZarfPackage{}, err
	}
	schemaStart := len(resources.schemas)

	// Zarf imports merge in the top level package objects variables and constants
	// however, imports are defined at the component level.
	// Two packages can both import one another as long as the importing components are on a different chains.
	// To detect cyclic imports, the stack is checked to see if the package has already been imported on that chain.
	// Recursive calls only include components from the imported pkg that have the name of the component to import
	importStack = append(importStack, pkgPath.BaseDir)

	l.Debug("start layout.ResolveImports",
		"pkg", pkg.Metadata.Name,
		"path", pkgPath.ManifestFile,
		"arch", arch,
		"flavor", flavor,
		"importStack", len(importStack),
	)

	variables := pkg.Variables
	constants := pkg.Constants
	components := []v1alpha1.ZarfComponent{}

	for _, component := range pkg.Components {
		if !compatibleComponent(component, arch, flavor) {
			continue
		}

		// Skip as component does not have any imports.
		if component.Import.Path == "" && component.Import.URL == "" {
			components = append(components, component)
			continue
		}

		if err := validateComponentCompose(component); err != nil {
			return v1alpha1.ZarfPackage{}, fmt.Errorf("invalid imported definition for %s: %w", component.Name, err)
		}

		var importedPkg v1alpha1.ZarfPackage
		var importedRemote *zoci.Remote
		var importedRoot *oci.Manifest
		if component.Import.Path != "" {
			importPath := filepath.Join(pkgPath.BaseDir, component.Import.Path)
			for _, sp := range importStack {
				if sp == importPath {
					return v1alpha1.ZarfPackage{}, fmt.Errorf("package %s imported in cycle by %s in component %s", filepath.ToSlash(importPath), filepath.ToSlash(pkgPath.BaseDir), component.Name)
				}
			}

			importPkgPath, err := layout.ResolvePackagePath(importPath)
			if err != nil {
				return v1alpha1.ZarfPackage{}, fmt.Errorf("unable to access import package path %q: %w", importPath, err)
			}

			b, err := os.ReadFile(importPkgPath.ManifestFile)
			if err != nil {
				return v1alpha1.ZarfPackage{}, err
			}
			importedPkg, err = pkgcfg.Parse(ctx, b)
			if err != nil {
				return v1alpha1.ZarfPackage{}, err
			}
			var relevantComponents []v1alpha1.ZarfComponent
			for _, importedComponent := range importedPkg.Components {
				if importedComponent.Name == getComponentToImportName(component) {
					relevantComponents = append(relevantComponents, importedComponent)
				}
			}
			importedPkg.Components = relevantComponents
			importedPkg, err = resolveImports(ctx, importedPkg, importPkgPath.ManifestFile, arch, flavor, importStack, cachePath, skipVersionCheck, remoteOptions, resources)
			if err != nil {
				return v1alpha1.ZarfPackage{}, err
			}
		} else if component.Import.URL != "" {
			cacheModifier, err := zoci.GetOCICacheModifier(ctx, cachePath)
			if err != nil {
				return v1alpha1.ZarfPackage{}, err
			}
			plainHTTP, err := negotiateImportPlainHTTP(ctx, component.Import.URL, remoteOptions)
			if err != nil {
				return v1alpha1.ZarfPackage{}, err
			}
			remote, err := zoci.NewRemote(ctx, component.Import.URL, zoci.PlatformForSkeleton(),
				cacheModifier, oci.WithPlainHTTP(plainHTTP), oci.WithInsecureSkipVerify(remoteOptions.InsecureSkipTLSVerify))
			if err != nil {
				return v1alpha1.ZarfPackage{}, err
			}
			rootDesc, err := remote.ResolveRoot(ctx)
			if err != nil {
				if strings.Contains(err.Error(), "no matching manifest was found in the manifest list") {
					return v1alpha1.ZarfPackage{}, fmt.Errorf("package at %s exists but has not been published as a skeleton: %w", component.Import.URL, err)
				}
				return v1alpha1.ZarfPackage{}, err
			}
			root, err := remote.FetchManifest(ctx, rootDesc)
			if err != nil {
				return v1alpha1.ZarfPackage{}, err
			}
			importedRemote = remote
			importedRoot = root
			importedPkg, err = zoci.FetchZarfYAML(ctx, root, remote)
			if err != nil {
				return v1alpha1.ZarfPackage{}, err
			}

			if !skipVersionCheck {
				// Validate skeleton package compatibility before pulling its component or values layers.
				if err := pkgvalidate.ValidateVersionRequirements(importedPkg); err != nil {
					return v1alpha1.ZarfPackage{}, fmt.Errorf("package %s has unmet requirements: %w If you cannot upgrade Zarf you may skip this check with --skip-version-check. Unexpected behavior or errors may occur", component.Import.URL, err)
				}
			}

			if len(importedPkg.Values.Files) > 0 || importedPkg.Values.Schema != "" {
				remoteValues, err := fetchOCISkeletonValues(ctx, remote, root, rootDesc, component.Import.URL, cachePath, importedPkg)
				if err != nil {
					return v1alpha1.ZarfPackage{}, err
				}
				resources.values = append(resources.values, remoteValues.values...)
				resources.schemas = append(resources.schemas, remoteValues.schemas...)
			}
		}

		name := getComponentToImportName(component)
		found := []v1alpha1.ZarfComponent{}
		for _, component := range importedPkg.Components {
			if component.Name == name && compatibleComponent(component, arch, flavor) {
				found = append(found, component)
			}
		}
		if len(found) == 0 {
			return v1alpha1.ZarfPackage{}, fmt.Errorf("no compatible component named %s found", name)
		} else if len(found) > 1 {
			return v1alpha1.ZarfPackage{}, fmt.Errorf("multiple components named %s found", name)
		}
		importedComponent := found[0]

		importPath, err := fetchOCISkeleton(ctx, component, pkgPath.BaseDir, cachePath, importedRemote, importedRoot)
		if err != nil {
			return v1alpha1.ZarfPackage{}, err
		}

		// this is a special case for paths and imports where we do not want to join BaseDir and importPath
		// we check that the path is valid but ensure the value remains relative for fixing
		fileInfo, err := os.Stat(filepath.Join(pkgPath.BaseDir, importPath))
		if err != nil {
			return v1alpha1.ZarfPackage{}, fmt.Errorf("unable to access import path %q: %w", importPath, err)
		}
		if !fileInfo.IsDir() {
			importPath = filepath.Dir(importPath)
		}
		importedComponent = fixPaths(importedComponent, importPath, pkgPath.BaseDir)
		composed, err := overrideMetadata(importedComponent, component)
		if err != nil {
			return v1alpha1.ZarfPackage{}, err
		}
		composed = overrideDeprecated(composed, component)
		composed = overrideActions(composed, component)
		composed = overrideResources(composed, component)

		components = append(components, composed)
		variables = append(variables, importedPkg.Variables...)
		constants = append(constants, importedPkg.Constants...)
	}

	for _, valueFile := range pkg.Values.Files {
		source, exists, err := readValueSource(pkgPath.BaseDir, valueFile)
		if err != nil {
			return v1alpha1.ZarfPackage{}, err
		}
		if exists {
			resources.values = append(resources.values, source)
		}
	}
	if pkg.Values.Schema != "" {
		source, err := readSource(pkgPath.BaseDir, pkg.Values.Schema)
		if err != nil {
			return v1alpha1.ZarfPackage{}, err
		}
		resources.schemas = append(resources.schemas[:schemaStart], append([]value.Source{source}, resources.schemas[schemaStart:]...)...)
	}
	pkg.Components = components

	varMap := map[string]bool{}
	pkg.Variables = nil
	for _, v := range variables {
		if _, present := varMap[v.Name]; !present {
			pkg.Variables = append(pkg.Variables, v)
			varMap[v.Name] = true
		}
	}

	constMap := map[string]bool{}
	pkg.Constants = nil
	for _, c := range constants {
		if _, present := constMap[c.Name]; !present {
			pkg.Constants = append(pkg.Constants, c)
			constMap[c.Name] = true
		}
	}

	l.Debug("done layout.ResolveImports",
		"pkg", pkg.Metadata.Name,
		"components", len(pkg.Components),
		"duration", time.Since(start),
	)
	return pkg, nil
}

type skeletonValues struct {
	values  []value.Source
	schemas []value.Source
}

// fetchOCISkeletonValues reads canonical package values layers from an imported
// skeleton. OCI blobs remain in the cache.
func fetchOCISkeletonValues(ctx context.Context, remote *zoci.Remote, root *oci.Manifest, rootDesc ocispec.Descriptor, importURL, cachePath string, pkg v1alpha1.ZarfPackage) (skeletonValues, error) {
	cache := filepath.Join(cachePath, "oci")
	store, err := ocistore.New(cache)
	if err != nil {
		return skeletonValues{}, err
	}

	type layer struct {
		declared bool
		path     string
		name     string
		schema   bool
	}
	layers := []layer{
		{declared: len(pkg.Values.Files) > 0, path: layout.ValuesYAML, name: "values"},
		{declared: pkg.Values.Schema != "", path: layout.ValuesSchema, name: "values schema", schema: true},
	}
	result := skeletonValues{}
	for _, layer := range layers {
		if !layer.declared {
			continue
		}
		desc := root.Locate(layer.path)
		if oci.IsEmptyDescriptor(desc) {
			return skeletonValues{}, fmt.Errorf("imported skeleton %s declares %s but does not contain %q", importURL, layer.name, layer.path)
		}
		if desc.Size < 0 || desc.Size > maxOCISkeletonValuesLayerSize {
			return skeletonValues{}, fmt.Errorf("imported skeleton %s %s layer exceeds the %d byte limit", importURL, layer.name, maxOCISkeletonValuesLayerSize)
		}
		exists, err := store.Exists(ctx, desc)
		if err != nil {
			return skeletonValues{}, err
		}
		if !exists {
			if err := remote.CopyToTarget(ctx, []ocispec.Descriptor{desc}, store, remote.GetDefaultCopyOpts()); err != nil {
				return skeletonValues{}, err
			}
		}
		src, err := store.Fetch(ctx, desc)
		if err != nil {
			return skeletonValues{}, fmt.Errorf("unable to fetch %s for imported skeleton %s: %w", layer.path, importURL, err)
		}
		contents, readErr := io.ReadAll(io.LimitReader(src, maxOCISkeletonValuesLayerSize+1))
		closeErr := src.Close()
		if err := errors.Join(readErr, closeErr); err != nil {
			return skeletonValues{}, fmt.Errorf("unable to read %s for imported skeleton %s: %w", layer.path, importURL, err)
		}
		if len(contents) > maxOCISkeletonValuesLayerSize {
			return skeletonValues{}, fmt.Errorf("imported skeleton %s %s layer exceeds the %d byte limit", importURL, layer.name, maxOCISkeletonValuesLayerSize)
		}
		if int64(len(contents)) != desc.Size {
			return skeletonValues{}, fmt.Errorf("imported skeleton %s %s layer has inconsistent descriptor size: declared %d bytes, read %d", importURL, layer.name, desc.Size, len(contents))
		}
		source := value.Source{Name: layer.path, Key: rootDesc.Digest.String() + ":" + layer.path, Data: contents}
		if layer.schema {
			result.schemas = append(result.schemas, source)
		} else {
			result.values = append(result.values, source)
		}
	}
	return result, nil
}

func validateComponentCompose(c v1alpha1.ZarfComponent) error {
	errs := []error{}
	if strings.Contains(c.Import.Path, v1alpha1.ZarfPackageTemplatePrefix) || strings.Contains(c.Import.URL, v1alpha1.ZarfPackageTemplatePrefix) {
		errs = append(errs, errors.New("package templates are not supported for import path or URL"))
	}
	if c.Import.Path == "" && c.Import.URL == "" {
		errs = append(errs, errors.New("neither a path nor a URL was provided"))
	}
	if c.Import.Path != "" && c.Import.URL != "" {
		errs = append(errs, errors.New("both a path and a URL were provided"))
	}
	if c.Import.URL == "" && c.Import.Path != "" {
		if filepath.IsAbs(c.Import.Path) {
			errs = append(errs, errors.New("path cannot be an absolute path"))
		}
	}
	if c.Import.URL != "" && c.Import.Path == "" {
		ok := helpers.IsOCIURL(c.Import.URL)
		if !ok {
			errs = append(errs, errors.New("URL is not a valid OCI URL"))
		}
	}
	return errors.Join(errs...)
}

func compatibleComponent(c v1alpha1.ZarfComponent, arch, flavor string) bool {
	satisfiesArch := c.Only.Cluster.Architecture == "" || c.Only.Cluster.Architecture == arch
	satisfiesFlavor := c.Only.Flavor == "" || c.Only.Flavor == flavor
	return satisfiesArch && satisfiesFlavor
}

// TODO: Extract descriptor-pinned selected-layer materialization into zoci so this
// component-import path and pullOCI can share it without introducing a package cycle.
func fetchOCISkeleton(ctx context.Context, component v1alpha1.ZarfComponent, packagePath string, cachePath string, remote *zoci.Remote, manifest *oci.Manifest) (string, error) {
	if component.Import.URL == "" {
		return component.Import.Path, nil
	}
	if remote == nil || manifest == nil {
		return "", fmt.Errorf("missing resolved remote manifest for skeleton %s", component.Import.URL)
	}

	name := component.Name
	if component.Import.Name != "" {
		name = component.Import.Name
	}

	cache := filepath.Join(cachePath, "oci")
	if err := helpers.CreateDirectory(cache, helpers.ReadWriteExecuteUser); err != nil {
		return "", err
	}

	// The caller resolves the OCI reference once and passes its manifest through so
	// component layers remain pinned to the same immutable descriptor as zarf.yaml
	// and package-level values.
	componentDesc := manifest.Locate(filepath.Join(layout.ComponentsDir, fmt.Sprintf("%s.tar", name)))
	var tarball, dir string
	// If the descriptor for the component tarball was not found then all resources in the component are remote
	// In this case, we represent the component with an empty directory
	if oci.IsEmptyDescriptor(componentDesc) {
		h := sha256.New()
		h.Write([]byte(component.Import.URL + name))
		id := fmt.Sprintf("%x", h.Sum(nil))

		dir = filepath.Join(cache, "dirs", id)
	} else {
		tarball = filepath.Join(cache, "blobs", "sha256", componentDesc.Digest.Encoded())
		dir = filepath.Join(cache, "dirs", componentDesc.Digest.Encoded())
		store, err := ocistore.New(cache)
		if err != nil {
			return "", err
		}
		exists, err := store.Exists(ctx, componentDesc)
		if err != nil {
			return "", err
		}
		if !exists {
			err = remote.CopyToTarget(ctx, []ocispec.Descriptor{componentDesc}, store, remote.GetDefaultCopyOpts())
			if err != nil {
				return "", err
			}
		}
	}

	if err := helpers.CreateDirectory(dir, helpers.ReadWriteExecuteUser); err != nil {
		return "", err
	}

	abs, err := filepath.Abs(packagePath)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(abs, dir)
	if err != nil {
		return "", err
	}

	// If it is a remote component, there is nothing to extract
	if oci.IsEmptyDescriptor(componentDesc) {
		return rel, nil
	}

	decompressOpts := archive.DecompressOpts{
		OverwriteExisting: true,
		StripComponents:   1,
		Extractor:         archives.Tar{},
	}
	err = archive.Decompress(ctx, tarball, dir, decompressOpts)
	if err != nil {
		return "", fmt.Errorf("unable to extract archive %q: %w", tarball, err)
	}

	return rel, nil
}

func overrideMetadata(comp v1alpha1.ZarfComponent, override v1alpha1.ZarfComponent) (v1alpha1.ZarfComponent, error) {
	// Metadata
	comp.Name = override.Name
	comp.Default = override.Default
	comp.Required = override.Required

	// Override description if it was provided.
	if override.Description != "" {
		comp.Description = override.Description
	}

	// If the imported component has a flavor, mark the component with that flavor
	if override.Only.Flavor != "" {
		comp.Only.Flavor = override.Only.Flavor
	}

	if override.Only.LocalOS != "" {
		if comp.Only.LocalOS != "" {
			return v1alpha1.ZarfComponent{}, fmt.Errorf("component %q: \"only.localOS\" %q cannot be redefined as %q during compose", comp.Name, comp.Only.LocalOS, override.Only.LocalOS)
		}
		comp.Only.LocalOS = override.Only.LocalOS
	}
	return comp, nil
}

func overrideDeprecated(comp v1alpha1.ZarfComponent, override v1alpha1.ZarfComponent) v1alpha1.ZarfComponent {
	comp.DeprecatedGroup = override.DeprecatedGroup

	// Merge deprecated scripts for backwards compatibility with older zarf binaries.
	comp.DeprecatedScripts.Before = append(comp.DeprecatedScripts.Before, override.DeprecatedScripts.Before...)
	comp.DeprecatedScripts.After = append(comp.DeprecatedScripts.After, override.DeprecatedScripts.After...)

	if override.DeprecatedScripts.Retry {
		comp.DeprecatedScripts.Retry = true
	}
	if override.DeprecatedScripts.ShowOutput {
		comp.DeprecatedScripts.ShowOutput = true
	}
	if override.DeprecatedScripts.TimeoutSeconds > 0 {
		comp.DeprecatedScripts.TimeoutSeconds = override.DeprecatedScripts.TimeoutSeconds
	}
	return comp
}

func overrideActions(comp v1alpha1.ZarfComponent, override v1alpha1.ZarfComponent) v1alpha1.ZarfComponent {
	comp.Actions.OnCreate.Defaults = override.Actions.OnCreate.Defaults
	comp.Actions.OnCreate.Before = append(comp.Actions.OnCreate.Before, override.Actions.OnCreate.Before...)
	comp.Actions.OnCreate.After = append(comp.Actions.OnCreate.After, override.Actions.OnCreate.After...)
	comp.Actions.OnCreate.OnFailure = append(comp.Actions.OnCreate.OnFailure, override.Actions.OnCreate.OnFailure...)
	comp.Actions.OnCreate.OnSuccess = append(comp.Actions.OnCreate.OnSuccess, override.Actions.OnCreate.OnSuccess...)

	comp.Actions.OnDeploy.Defaults = override.Actions.OnDeploy.Defaults
	comp.Actions.OnDeploy.Before = append(comp.Actions.OnDeploy.Before, override.Actions.OnDeploy.Before...)
	comp.Actions.OnDeploy.After = append(comp.Actions.OnDeploy.After, override.Actions.OnDeploy.After...)
	comp.Actions.OnDeploy.OnFailure = append(comp.Actions.OnDeploy.OnFailure, override.Actions.OnDeploy.OnFailure...)
	comp.Actions.OnDeploy.OnSuccess = append(comp.Actions.OnDeploy.OnSuccess, override.Actions.OnDeploy.OnSuccess...)

	comp.Actions.OnRemove.Defaults = override.Actions.OnRemove.Defaults
	comp.Actions.OnRemove.Before = append(comp.Actions.OnRemove.Before, override.Actions.OnRemove.Before...)
	comp.Actions.OnRemove.After = append(comp.Actions.OnRemove.After, override.Actions.OnRemove.After...)
	comp.Actions.OnRemove.OnFailure = append(comp.Actions.OnRemove.OnFailure, override.Actions.OnRemove.OnFailure...)
	comp.Actions.OnRemove.OnSuccess = append(comp.Actions.OnRemove.OnSuccess, override.Actions.OnRemove.OnSuccess...)
	return comp
}

func overrideResources(comp v1alpha1.ZarfComponent, override v1alpha1.ZarfComponent) v1alpha1.ZarfComponent {
	comp.DataInjections = append(comp.DataInjections, override.DataInjections...)
	comp.Files = append(comp.Files, override.Files...)
	comp.Images = append(comp.Images, override.Images...)
	comp.Repos = append(comp.Repos, override.Repos...)

	// Merge charts with the same name to keep them unique
	for _, overrideChart := range override.Charts {
		existing := false
		for idx := range comp.Charts {
			if comp.Charts[idx].Name == overrideChart.Name {
				if overrideChart.Namespace != "" {
					comp.Charts[idx].Namespace = overrideChart.Namespace
				}
				if overrideChart.ReleaseName != "" {
					comp.Charts[idx].ReleaseName = overrideChart.ReleaseName
				}
				if overrideChart.Version != "" {
					comp.Charts[idx].Version = overrideChart.Version
				}
				if overrideChart.URL != "" {
					comp.Charts[idx].URL = overrideChart.URL
				}
				comp.Charts[idx].ValuesFiles = append(comp.Charts[idx].ValuesFiles, overrideChart.ValuesFiles...)
				comp.Charts[idx].TemplatedValuesFiles = append(comp.Charts[idx].TemplatedValuesFiles, overrideChart.TemplatedValuesFiles...)
				comp.Charts[idx].Variables = append(comp.Charts[idx].Variables, overrideChart.Variables...)
				comp.Charts[idx].Values = append(comp.Charts[idx].Values, overrideChart.Values...)
				existing = true
			}
		}

		if !existing {
			comp.Charts = append(comp.Charts, overrideChart)
		}
	}

	// Merge manifests with the same name to keep them unique
	for _, overrideManifest := range override.Manifests {
		existing := false
		for idx := range comp.Manifests {
			if comp.Manifests[idx].Name == overrideManifest.Name {
				if overrideManifest.Namespace != "" {
					comp.Manifests[idx].Namespace = overrideManifest.Namespace
				}
				comp.Manifests[idx].Files = append(comp.Manifests[idx].Files, overrideManifest.Files...)
				comp.Manifests[idx].Kustomizations = append(comp.Manifests[idx].Kustomizations, overrideManifest.Kustomizations...)

				existing = true
			}
		}

		if !existing {
			comp.Manifests = append(comp.Manifests, overrideManifest)
		}
	}

	comp.HealthChecks = append(comp.HealthChecks, override.HealthChecks...)
	comp.ImageArchives = append(comp.ImageArchives, override.ImageArchives...)

	return comp
}

func makePathRelativeTo(path, relativeTo string) string {
	if helpers.IsURL(path) {
		return path
	}
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.ToSlash(filepath.Join(relativeTo, path))
}

func fixPaths(child v1alpha1.ZarfComponent, relativeToHead, packagePath string) v1alpha1.ZarfComponent {
	for fileIdx, file := range child.Files {
		composed := makePathRelativeTo(file.Source, relativeToHead)
		child.Files[fileIdx].Source = composed
	}

	for idx, imageArchive := range child.ImageArchives {
		composed := makePathRelativeTo(imageArchive.Path, relativeToHead)
		child.ImageArchives[idx].Path = composed
	}

	for chartIdx, chart := range child.Charts {
		for valuesIdx, valuesFile := range chart.ValuesFiles {
			composed := makePathRelativeTo(valuesFile, relativeToHead)
			child.Charts[chartIdx].ValuesFiles[valuesIdx] = composed
		}
		for valuesIdx, valuesFile := range chart.TemplatedValuesFiles {
			composed := makePathRelativeTo(valuesFile, relativeToHead)
			child.Charts[chartIdx].TemplatedValuesFiles[valuesIdx] = composed
		}
		if child.Charts[chartIdx].LocalPath != "" {
			composed := makePathRelativeTo(chart.LocalPath, relativeToHead)
			child.Charts[chartIdx].LocalPath = composed
		}
	}

	for manifestIdx, manifest := range child.Manifests {
		for fileIdx, file := range manifest.Files {
			composed := makePathRelativeTo(file, relativeToHead)
			child.Manifests[manifestIdx].Files[fileIdx] = composed
		}
		for kustomizeIdx, kustomization := range manifest.Kustomizations {
			composed := makePathRelativeTo(kustomization, relativeToHead)
			// kustomizations can use non-standard urls, so we need to check if the composed path exists on the local filesystem
			invalid := helpers.InvalidPath(filepath.Join(packagePath, composed))
			if !invalid {
				child.Manifests[manifestIdx].Kustomizations[kustomizeIdx] = composed
			}
		}
	}

	for dataInjectionsIdx, dataInjection := range child.DataInjections {
		composed := makePathRelativeTo(dataInjection.Source, relativeToHead)
		child.DataInjections[dataInjectionsIdx].Source = composed
	}

	defaultDir := child.Actions.OnCreate.Defaults.Dir
	child.Actions.OnCreate.Before = fixActionPaths(child.Actions.OnCreate.Before, defaultDir, relativeToHead)
	child.Actions.OnCreate.After = fixActionPaths(child.Actions.OnCreate.After, defaultDir, relativeToHead)
	child.Actions.OnCreate.OnFailure = fixActionPaths(child.Actions.OnCreate.OnFailure, defaultDir, relativeToHead)
	child.Actions.OnCreate.OnSuccess = fixActionPaths(child.Actions.OnCreate.OnSuccess, defaultDir, relativeToHead)

	return child
}

// fixActionPaths takes a slice of actions and mutates the Dir to be relative to the head node
func fixActionPaths(actions []v1alpha1.ZarfComponentAction, defaultDir, relativeToHead string) []v1alpha1.ZarfComponentAction {
	for actionIdx, action := range actions {
		var composed string
		if action.Dir != nil {
			composed = makePathRelativeTo(*action.Dir, relativeToHead)
		} else {
			composed = makePathRelativeTo(defaultDir, relativeToHead)
		}
		actions[actionIdx].Dir = &composed
	}
	return actions
}
