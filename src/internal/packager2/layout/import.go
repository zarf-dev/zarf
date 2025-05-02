// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zarf-dev/zarf/src/pkg/logger"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/pkg/oci"
	"github.com/mholt/archiver/v3"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	ocistore "oras.land/oras-go/v2/content/oci"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/layout"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
)

func getComponentToImportName(component v1alpha1.ZarfComponent) string {
	if component.Import.Name != "" {
		return component.Import.Name
	}
	return component.Name
}

func resolveImports(ctx context.Context, pkg v1alpha1.ZarfPackage, packagePath, arch, flavor string, importStack []string) (v1alpha1.ZarfPackage, error) {
	l := logger.From(ctx)
	start := time.Now()

	// Zarf imports merge in the top level package objects variables and constants
	// however, imports are defined at the component level.
	// Two packages can both import one another as long as the importing components are on a different chains.
	// To detect cyclic imports, the stack is checked to see if the package has already been imported on that chain.
	// Recursive calls only include components from the imported pkg that have the name of the component to import
	importStack = append(importStack, packagePath)

	l.Debug("start layout.ResolveImports",
		"pkg", pkg.Metadata.Name,
		"path", packagePath,
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
		if component.Import.Path != "" {
			importPath := filepath.Join(packagePath, component.Import.Path)
			for _, sp := range importStack {
				if sp == importPath {
					return v1alpha1.ZarfPackage{}, fmt.Errorf("package %s imported in cycle by %s in component %s", filepath.ToSlash(importPath), filepath.ToSlash(packagePath), component.Name)
				}
			}
			b, err := os.ReadFile(filepath.Join(importPath, layout.ZarfYAML))
			if err != nil {
				return v1alpha1.ZarfPackage{}, err
			}
			importedPkg, err = ParseZarfPackage(ctx, b)
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
			importedPkg, err = resolveImports(ctx, importedPkg, importPath, arch, flavor, importStack)
			if err != nil {
				return v1alpha1.ZarfPackage{}, err
			}
		} else if component.Import.URL != "" {
			remote, err := zoci.NewRemote(ctx, component.Import.URL, zoci.PlatformForSkeleton())
			if err != nil {
				return v1alpha1.ZarfPackage{}, err
			}
			_, err = remote.ResolveRoot(ctx)
			if err != nil {
				return v1alpha1.ZarfPackage{}, err
			}
			importedPkg, err = remote.FetchZarfYAML(ctx)
			if err != nil {
				return v1alpha1.ZarfPackage{}, err
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

		importPath, err := fetchOCISkeleton(ctx, component, packagePath)
		if err != nil {
			return v1alpha1.ZarfPackage{}, err
		}
		importedComponent = fixPaths(importedComponent, importPath, packagePath)
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

// TODO (phillebaba): Refactor package structure so that pullOCI can be used instead.
func fetchOCISkeleton(ctx context.Context, component v1alpha1.ZarfComponent, packagePath string) (string, error) {
	if component.Import.URL == "" {
		return component.Import.Path, nil
	}

	name := component.Name
	if component.Import.Name != "" {
		name = component.Import.Name
	}

	absCachePath, err := config.GetAbsCachePath()
	if err != nil {
		return "", err
	}
	cache := filepath.Join(absCachePath, "oci")
	if err := helpers.CreateDirectory(cache, helpers.ReadWriteExecuteUser); err != nil {
		return "", err
	}

	// Get the descriptor for the component.
	remote, err := zoci.NewRemote(ctx, component.Import.URL, zoci.PlatformForSkeleton())
	if err != nil {
		return "", err
	}
	_, err = remote.ResolveRoot(ctx)
	if err != nil {
		return "", fmt.Errorf("published skeleton package for %s does not exist: %w", component.Import.URL, err)
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

	// TODO(mkcp): See https://github.com/zarf-dev/zarf/issues/3051
	tu := archiver.Tar{
		OverwriteExisting: true,
		// removes /<component-name>/ from the paths
		StripComponents: 1,
	}
	err = tu.Unarchive(tarball, dir)
	if err != nil {
		return "", err
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
	// Override cosign key path if it was provided.
	if override.DeprecatedCosignKeyPath != "" {
		comp.DeprecatedCosignKeyPath = override.DeprecatedCosignKeyPath
	}

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
				comp.Charts[idx].ValuesFiles = append(comp.Charts[idx].ValuesFiles, overrideChart.ValuesFiles...)
				comp.Charts[idx].Variables = append(comp.Charts[idx].Variables, overrideChart.Variables...)
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

	return comp
}

func makePathRelativeTo(path, relativeTo string) string {
	if helpers.IsURL(path) {
		return path
	}
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(relativeTo, path)
}

func fixPaths(child v1alpha1.ZarfComponent, relativeToHead, packagePath string) v1alpha1.ZarfComponent {
	for fileIdx, file := range child.Files {
		composed := makePathRelativeTo(file.Source, relativeToHead)
		child.Files[fileIdx].Source = composed
	}

	for chartIdx, chart := range child.Charts {
		for valuesIdx, valuesFile := range chart.ValuesFiles {
			composed := makePathRelativeTo(valuesFile, relativeToHead)
			child.Charts[chartIdx].ValuesFiles[valuesIdx] = composed
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

	// deprecated
	if child.DeprecatedCosignKeyPath != "" {
		composed := makePathRelativeTo(child.DeprecatedCosignKeyPath, relativeToHead)
		child.DeprecatedCosignKeyPath = composed
	}

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
