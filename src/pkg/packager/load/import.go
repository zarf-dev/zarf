// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package load

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mholt/archives"
	pkgvalidate "github.com/zarf-dev/zarf/src/internal/packager/requirements"
	"github.com/zarf-dev/zarf/src/internal/pkgcfg"
	"github.com/zarf-dev/zarf/src/pkg/archive"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/types"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/pkg/oci"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	ocistore "oras.land/oras-go/v2/content/oci"

	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/convert"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
)

func getComponentToImportName(component api.Component) string {
	if component.Import.Name != "" {
		return component.Import.Name
	}
	return component.Name
}

func resolveImports(ctx context.Context, pkg api.Package, packagePath, arch, flavor string, importStack []string, cachePath string, skipVersionCheck bool, remoteOptions types.RemoteOptions) (api.Package, []string, error) {
	l := logger.From(ctx)
	start := time.Now()

	pkgPath, err := layout.ResolvePackagePath(packagePath)
	if err != nil {
		return api.Package{}, nil, err
	}

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

	var valuesFiles []string
	var importedSchemas []string
	variables := pkg.Variables
	constants := pkg.Constants
	components := []api.Component{}

	for _, component := range pkg.Components {
		if !compatibleComponent(component, arch, flavor) {
			continue
		}

		// Skip as component does not have any imports.
		if componentImportPath(component) == "" && componentImportURL(component) == "" {
			components = append(components, component)
			continue
		}

		if err := validateComponentCompose(component); err != nil {
			return api.Package{}, nil, fmt.Errorf("invalid imported definition for %s: %w", component.Name, err)
		}

		var importedPkg api.Package
		var innerSchemas []string
		if componentImportPath(component) != "" {
			importPath := filepath.Join(pkgPath.BaseDir, componentImportPath(component))
			for _, sp := range importStack {
				if sp == importPath {
					return api.Package{}, nil, fmt.Errorf("package %s imported in cycle by %s in component %s", filepath.ToSlash(importPath), filepath.ToSlash(pkgPath.BaseDir), component.Name)
				}
			}

			importPkgPath, err := layout.ResolvePackagePath(importPath)
			if err != nil {
				return api.Package{}, nil, fmt.Errorf("unable to access import package path %q: %w", importPath, err)
			}

			b, err := os.ReadFile(importPkgPath.ManifestFile)
			if err != nil {
				return api.Package{}, nil, err
			}
			parsed, err := pkgcfg.ParseAs(ctx, b, pkgcfg.V1Alpha1)
			if err != nil {
				return api.Package{}, nil, err
			}
			importedPkg = convert.PackageFromV1alpha1(parsed)
			var relevantComponents []api.Component
			for _, importedComponent := range importedPkg.Components {
				if importedComponent.Name == getComponentToImportName(component) {
					relevantComponents = append(relevantComponents, importedComponent)
				}
			}
			importedPkg.Components = relevantComponents
			importedPkg, innerSchemas, err = resolveImports(ctx, importedPkg, importPkgPath.ManifestFile, arch, flavor, importStack, cachePath, skipVersionCheck, remoteOptions)
			if err != nil {
				return api.Package{}, nil, err
			}
		} else if componentImportURL(component) != "" {
			remote, err := zoci.NewRemoteWithOptions(ctx, componentImportURL(component), zoci.PlatformForSkeleton(), zoci.RemoteClientOptions{
				CachePath:     cachePath,
				RemoteOptions: remoteOptions,
			})
			if err != nil {
				return api.Package{}, nil, err
			}
			_, err = remote.ResolveRoot(ctx)
			if err != nil {
				if strings.Contains(err.Error(), "no matching manifest was found in the manifest list") {
					return api.Package{}, nil, fmt.Errorf("package at %s exists but has not been published as a skeleton: %w", componentImportURL(component), err)
				}
				return api.Package{}, nil, err
			}
			importedPkg, err = remote.FetchZarfYAML(ctx)
			if err != nil {
				return api.Package{}, nil, err
			}

			if len(importedPkg.Values.Files) > 0 || importedPkg.Values.Schema != "" {
				return api.Package{}, nil, fmt.Errorf("imported skeleton %s declares values which are not yet supported", componentImportURL(component))
			}

			if !skipVersionCheck {
				// Validate skeleton package is compatible with new package
				if err := pkgvalidate.ValidateVersionRequirements(importedPkg); err != nil {
					return api.Package{}, nil, fmt.Errorf("package %s has unmet requirements: %w If you cannot upgrade Zarf you may skip this check with --skip-version-check. Unexpected behavior or errors may occur", componentImportURL(component), err)
				}
			}
		}

		name := getComponentToImportName(component)
		found := []api.Component{}
		for _, component := range importedPkg.Components {
			if component.Name == name && compatibleComponent(component, arch, flavor) {
				found = append(found, component)
			}
		}
		if len(found) == 0 {
			return api.Package{}, nil, fmt.Errorf("no compatible component named %s found", name)
		} else if len(found) > 1 {
			return api.Package{}, nil, fmt.Errorf("multiple components named %s found", name)
		}
		importedComponent := found[0]

		importPath, err := fetchOCISkeleton(ctx, component, pkgPath.BaseDir, cachePath, remoteOptions)
		if err != nil {
			return api.Package{}, nil, err
		}

		// this is a special case for paths and imports where we do not want to join BaseDir and importPath
		// we check that the path is valid but ensure the value remains relative for fixing
		fileInfo, err := os.Stat(filepath.Join(pkgPath.BaseDir, importPath))
		if err != nil {
			return api.Package{}, nil, fmt.Errorf("unable to access import path %q: %w", importPath, err)
		}
		if !fileInfo.IsDir() {
			importPath = filepath.Dir(importPath)
		}
		importedComponent = fixPaths(importedComponent, importPath, pkgPath.BaseDir)
		composed, err := overrideMetadata(importedComponent, component)
		if err != nil {
			return api.Package{}, nil, err
		}
		composed = overrideDeprecated(composed, component)
		composed = overrideActions(composed, component)
		composed = overrideResources(composed, component)

		components = append(components, composed)
		variables = append(variables, importedPkg.Variables...)
		constants = append(constants, importedPkg.Constants...)
		for _, v := range importedPkg.Values.Files {
			valuesFiles = append(valuesFiles, makePathRelativeTo(v, importPath))
		}
		if importedPkg.Values.Schema != "" {
			importedSchemas = append(importedSchemas, makePathRelativeTo(importedPkg.Values.Schema, importPath))
		}
		for _, s := range innerSchemas {
			importedSchemas = append(importedSchemas, makePathRelativeTo(s, importPath))
		}
	}

	valuesFiles = append(valuesFiles, pkg.Values.Files...)
	valuesFilesMap := map[string]bool{}
	pkg.Values.Files = nil
	for _, v := range valuesFiles {
		norm := v
		if !helpers.IsURL(v) && !filepath.IsAbs(v) {
			norm = filepath.ToSlash(filepath.Clean(v))
		}
		if _, present := valuesFilesMap[norm]; !present {
			pkg.Values.Files = append(pkg.Values.Files, norm)
			valuesFilesMap[norm] = true
		}
	}
	pkg.Components = components

	schemasMap := map[string]bool{}
	var deduplicatedSchemas []string
	for _, s := range importedSchemas {
		norm := filepath.ToSlash(filepath.Clean(s))
		if !schemasMap[norm] {
			deduplicatedSchemas = append(deduplicatedSchemas, norm)
			schemasMap[norm] = true
		}
	}

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
	return pkg, deduplicatedSchemas, nil
}

func componentImportPath(component api.Component) string {
	if len(component.Import.Local) == 0 {
		return ""
	}
	return component.Import.Local[0].Path
}

func componentImportURL(component api.Component) string {
	if len(component.Import.Remote) == 0 {
		return ""
	}
	return component.Import.Remote[0].URL
}

func validateComponentCompose(c api.Component) error {
	errs := []error{}
	importPath := componentImportPath(c)
	importURL := componentImportURL(c)
	if strings.Contains(importPath, api.PackageTemplatePrefix) || strings.Contains(importURL, api.PackageTemplatePrefix) {
		errs = append(errs, errors.New("package templates are not supported for import path or URL"))
	}
	if importPath == "" && importURL == "" {
		errs = append(errs, errors.New("neither a path nor a URL was provided"))
	}
	if importPath != "" && importURL != "" {
		errs = append(errs, errors.New("both a path and a URL were provided"))
	}
	if importURL == "" && importPath != "" {
		if filepath.IsAbs(importPath) {
			errs = append(errs, errors.New("path cannot be an absolute path"))
		}
	}
	if importURL != "" && importPath == "" {
		ok := helpers.IsOCIURL(importURL)
		if !ok {
			errs = append(errs, errors.New("URL is not a valid OCI URL"))
		}
	}
	return errors.Join(errs...)
}

func compatibleComponent(c api.Component, arch, flavor string) bool {
	satisfiesArch := c.Target.Architecture == "" || c.Target.Architecture == arch
	satisfiesFlavor := c.Target.Flavor == "" || c.Target.Flavor == flavor
	return satisfiesArch && satisfiesFlavor
}

// TODO (phillebaba): Refactor package structure so that pullOCI can be used instead.
func fetchOCISkeleton(ctx context.Context, component api.Component, packagePath string, cachePath string, remoteOptions types.RemoteOptions) (string, error) {
	if componentImportURL(component) == "" {
		return componentImportPath(component), nil
	}

	name := component.Name
	if component.Import.Name != "" {
		name = component.Import.Name
	}

	cache := filepath.Join(cachePath, "oci")
	if err := helpers.CreateDirectory(cache, helpers.ReadWriteExecuteUser); err != nil {
		return "", err
	}

	// Get the descriptor for the component.
	remote, err := zoci.NewRemoteWithOptions(ctx, componentImportURL(component), zoci.PlatformForSkeleton(), zoci.RemoteClientOptions{
		RemoteOptions: remoteOptions,
	})
	if err != nil {
		return "", err
	}
	_, err = remote.ResolveRoot(ctx)
	if err != nil {
		// This error likely won't occur as the root has been resolved before this function is invoked.
		// This serves as a secondary mechanism to highlight the potential for the package existing without a published skeleton.
		if strings.Contains(err.Error(), "no matching manifest was found in the manifest list") {
			return "", fmt.Errorf("package at %s exists but has not been published as a skeleton: %w", componentImportURL(component), err)
		}
		return "", fmt.Errorf("published skeleton package for %s does not exist: %w", componentImportURL(component), err)
	}
	manifest, err := remote.FetchRoot(ctx)
	if err != nil {
		return "", err
	}
	componentDesc := manifest.Locate(filepath.Join(layout.ComponentsDir, fmt.Sprintf("%s.tar", name)))
	var tarball, dir string
	// If the descriptor for the component tarball was not found then all resources in the component are remote
	// In this case, we represent the component with an empty directory
	if oci.IsEmptyDescriptor(componentDesc) {
		h := sha256.New()
		h.Write([]byte(componentImportURL(component) + name))
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

func overrideMetadata(comp api.Component, override api.Component) (api.Component, error) {
	// Metadata
	comp.Name = override.Name
	comp.Default = override.Default
	comp.Optional = override.Optional

	// Override description if it was provided.
	if override.Description != "" {
		comp.Description = override.Description
	}

	// If the imported component has a flavor, mark the component with that flavor
	if override.Target.Flavor != "" {
		comp.Target.Flavor = override.Target.Flavor
	}

	if override.Target.OS != "" {
		if comp.Target.OS != "" {
			return api.Component{}, fmt.Errorf("component %q: \"only.localOS\" %q cannot be redefined as %q during compose", comp.Name, comp.Target.OS, override.Target.OS)
		}
		comp.Target.OS = override.Target.OS
	}
	return comp, nil
}

func overrideDeprecated(comp api.Component, override api.Component) api.Component {
	comp.Group = override.Group

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

func overrideActions(comp api.Component, override api.Component) api.Component {
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

func overrideResources(comp api.Component, override api.Component) api.Component {
	comp.DataInjections = append(comp.DataInjections, override.DataInjections...)
	comp.Files = append(comp.Files, override.Files...)
	comp.Images = append(comp.Images, override.Images...)
	comp.Repositories = append(comp.Repositories, override.Repositories...)

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
				if overrideChart.SourceURL() != "" {
					comp.Charts[idx].HelmRepository = overrideChart.HelmRepository
					comp.Charts[idx].Git = overrideChart.Git
					comp.Charts[idx].Local = overrideChart.Local
					comp.Charts[idx].OCI = overrideChart.OCI
				}
				comp.Charts[idx].ValuesFiles = append(comp.Charts[idx].ValuesFiles, overrideChart.ValuesFiles...)
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
				if len(overrideManifest.Kustomize.Files) > 0 || overrideManifest.Kustomize.AllowAnyDirectory || overrideManifest.Kustomize.EnablePlugins {
					comp.Manifests[idx].Kustomize.Files = append(comp.Manifests[idx].Kustomize.Files, overrideManifest.Kustomize.Files...)
					comp.Manifests[idx].Kustomize.AllowAnyDirectory = comp.Manifests[idx].Kustomize.AllowAnyDirectory || overrideManifest.Kustomize.AllowAnyDirectory
					comp.Manifests[idx].Kustomize.EnablePlugins = comp.Manifests[idx].Kustomize.EnablePlugins || overrideManifest.Kustomize.EnablePlugins
				}

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

func fixPaths(child api.Component, relativeToHead, packagePath string) api.Component {
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
			child.Charts[chartIdx].ValuesFiles[valuesIdx].Path = makePathRelativeTo(valuesFile.Path, relativeToHead)
		}
		if child.Charts[chartIdx].Local != nil {
			child.Charts[chartIdx].Local.Path = makePathRelativeTo(chart.Local.Path, relativeToHead)
		}
	}

	for manifestIdx, manifest := range child.Manifests {
		for fileIdx, file := range manifest.Files {
			composed := makePathRelativeTo(file, relativeToHead)
			child.Manifests[manifestIdx].Files[fileIdx] = composed
		}
		for kustomizeIdx, kustomization := range manifest.Kustomize.Files {
			composed := makePathRelativeTo(kustomization, relativeToHead)
			// kustomizations can use non-standard urls, so we need to check if the composed path exists on the local filesystem
			invalid := helpers.InvalidPath(filepath.Join(packagePath, composed))
			if !invalid {
				child.Manifests[manifestIdx].Kustomize.Files[kustomizeIdx] = composed
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
func fixActionPaths(actions []api.Action, defaultDir, relativeToHead string) []api.Action {
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
