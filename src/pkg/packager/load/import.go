// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package load

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	goyaml "github.com/goccy/go-yaml"
	"github.com/mholt/archives"
	pkgvalidate "github.com/zarf-dev/zarf/src/internal/packager/requirements"
	"github.com/zarf-dev/zarf/src/internal/pkgcfg"
	"github.com/zarf-dev/zarf/src/internal/template"
	"github.com/zarf-dev/zarf/src/pkg/archive"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/types"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/pkg/oci"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	ocistore "oras.land/oras-go/v2/content/oci"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
)

func getComponentToImportName(component v1alpha1.ZarfComponent) string {
	if component.Import.Name != "" {
		return component.Import.Name
	}
	return component.Name
}

// getOrCreateComponentTempDir lazily creates a temp directory and returns a component-namespaced
// subdirectory within it. The tempDir pointer is populated on first call and reused on subsequent calls.
func getOrCreateComponentTempDir(tempDir *string, componentName string) (string, error) {
	if *tempDir == "" {
		dir, err := os.MkdirTemp("", "zarf-import-")
		if err != nil {
			return "", fmt.Errorf("failed to create temp directory: %w", err)
		}
		*tempDir = dir
	}
	componentDir := filepath.Join(*tempDir, componentName)
	if err := os.MkdirAll(componentDir, helpers.ReadWriteExecuteUser); err != nil {
		return "", fmt.Errorf("failed to create component temp directory: %w", err)
	}
	return componentDir, nil
}

// namespaceSchema reads a JSON schema, wraps its properties under the component name,
// and returns the namespaced schema. Returns nil if the schema path is empty.
func namespaceSchema(packagePath, schemaPath, componentName string) (map[string]any, error) {
	if schemaPath == "" {
		return nil, nil
	}

	// Resolve the full path to the schema file
	fullPath := schemaPath
	if !filepath.IsAbs(schemaPath) {
		fullPath = filepath.Join(packagePath, schemaPath)
	}

	// Read the schema file
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema file %s: %w", fullPath, err)
	}

	// Parse the JSON schema
	var schema map[string]any
	if err := json.Unmarshal(content, &schema); err != nil {
		return nil, fmt.Errorf("failed to parse schema file %s: %w", fullPath, err)
	}

	// Create a namespaced schema that wraps the original under the component name
	namespaced := map[string]any{
		"type": "object",
		"properties": map[string]any{
			componentName: schema,
		},
	}

	return namespaced, nil
}

// mergeSchemas merges two JSON schemas by combining their properties.
// The second schema's properties take precedence in case of conflicts.
func mergeSchemas(base, overlay map[string]any) map[string]any {
	if base == nil {
		return overlay
	}
	if overlay == nil {
		return base
	}

	result := map[string]any{
		"type": "object",
	}

	// Get properties from both schemas
	baseProps, ok := base["properties"].(map[string]any)
	if !ok {
		baseProps = nil
	}
	overlayProps, ok := overlay["properties"].(map[string]any)
	if !ok {
		overlayProps = nil
	}

	if baseProps == nil {
		baseProps = map[string]any{}
	}

	// Merge properties
	mergedProps := make(map[string]any)
	for k, v := range baseProps {
		mergedProps[k] = v
	}
	for k, v := range overlayProps {
		mergedProps[k] = v
	}

	result["properties"] = mergedProps
	return result
}

// writeMergedSchema writes a merged schema to the temp directory and returns the path.
func writeMergedSchema(tempDir *string, schema map[string]any) (string, error) {
	if schema == nil {
		return "", nil
	}

	// Ensure temp directory exists
	if *tempDir == "" {
		dir, err := os.MkdirTemp("", "zarf-import-")
		if err != nil {
			return "", fmt.Errorf("failed to create temp directory: %w", err)
		}
		*tempDir = dir
	}

	// Marshal the schema as JSON with indentation for readability
	content, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal merged schema: %w", err)
	}

	// Write to temp directory
	destPath := filepath.Join(*tempDir, "values.schema.json")
	if err := os.WriteFile(destPath, content, helpers.ReadWriteUser); err != nil {
		return "", fmt.Errorf("failed to write merged schema: %w", err)
	}

	return destPath, nil
}

// namespaceValuesFile reads a values file, wraps its contents under the component name as a root key,
// and writes it to the component's temp directory. Returns the path to the new file.
func namespaceValuesFile(tempDir *string, componentName, packagePath, valuesFilePath string) (string, error) {
	// Resolve the full path to the values file
	fullPath := valuesFilePath
	if !filepath.IsAbs(valuesFilePath) {
		fullPath = filepath.Join(packagePath, valuesFilePath)
	}

	// Read the original values file
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to read values file %s: %w", fullPath, err)
	}

	// Parse the YAML content
	var values map[string]any
	if err := goyaml.Unmarshal(content, &values); err != nil {
		return "", fmt.Errorf("failed to parse values file %s: %w", fullPath, err)
	}

	// Wrap under component name
	namespaced := map[string]any{
		componentName: values,
	}

	// Marshal back to YAML
	namespacedContent, err := goyaml.Marshal(namespaced)
	if err != nil {
		return "", fmt.Errorf("failed to marshal namespaced values: %w", err)
	}

	// Get or create the component temp directory
	componentDir, err := getOrCreateComponentTempDir(tempDir, componentName)
	if err != nil {
		return "", err
	}

	// Write to temp directory, preserving the original filename
	destPath := filepath.Join(componentDir, filepath.Base(valuesFilePath))
	if err := os.WriteFile(destPath, namespacedContent, helpers.ReadWriteUser); err != nil {
		return "", fmt.Errorf("failed to write namespaced values file: %w", err)
	}

	return destPath, nil
}

func resolveImports(ctx context.Context, pkg v1alpha1.ZarfPackage, packagePath, arch, flavor string, importStack []string, cachePath string, skipVersionCheck bool, remoteOptions types.RemoteOptions, tempDir *string) (v1alpha1.ZarfPackage, error) {
	l := logger.From(ctx)
	start := time.Now()

	pkgPath, err := layout.ResolvePackagePath(packagePath)
	if err != nil {
		return v1alpha1.ZarfPackage{}, err
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
	var mergedSchema map[string]any
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
			importedPkg, err = resolveImports(ctx, importedPkg, importPkgPath.ManifestFile, arch, flavor, importStack, cachePath, skipVersionCheck, remoteOptions, tempDir)
			if err != nil {
				return v1alpha1.ZarfPackage{}, err
			}
		} else if component.Import.URL != "" {
			cacheModifier, err := zoci.GetOCICacheModifier(ctx, cachePath)
			if err != nil {
				return v1alpha1.ZarfPackage{}, err
			}
			remote, err := zoci.NewRemote(ctx, component.Import.URL, zoci.PlatformForSkeleton(),
				cacheModifier, oci.WithPlainHTTP(remoteOptions.PlainHTTP), oci.WithInsecureSkipVerify(remoteOptions.InsecureSkipTLSVerify))
			if err != nil {
				return v1alpha1.ZarfPackage{}, err
			}
			_, err = remote.ResolveRoot(ctx)
			if err != nil {
				if strings.Contains(err.Error(), "no matching manifest was found in the manifest list") {
					return v1alpha1.ZarfPackage{}, fmt.Errorf("package at %s exists but has not been published as a skeleton: %w", component.Import.URL, err)
				}
				return v1alpha1.ZarfPackage{}, err
			}
			importedPkg, err = remote.FetchZarfYAML(ctx)
			if err != nil {
				return v1alpha1.ZarfPackage{}, err
			}
			if !skipVersionCheck {
				// Validate skeleton package is compatible with new package
				if err := pkgvalidate.ValidateVersionRequirements(importedPkg); err != nil {
					return v1alpha1.ZarfPackage{}, fmt.Errorf("package %s has unmet requirements: %w If you cannot upgrade Zarf you may skip this check with --skip-version-check. Unexpected behavior or errors may occur", component.Import.URL, err)
				}
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

		importPath, err := fetchOCISkeleton(ctx, component, pkgPath.BaseDir, cachePath, remoteOptions)
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
		// namespace the values here before overriding other data from the parent
		composed, err = namespaceTemplates(composed, pkgPath.BaseDir, tempDir)
		if err != nil {
			return v1alpha1.ZarfPackage{}, err
		}
		composed = overrideDeprecated(composed, component)
		composed = overrideActions(composed, component)
		composed = overrideResources(composed, component)

		components = append(components, composed)
		variables = append(variables, importedPkg.Variables...)
		constants = append(constants, importedPkg.Constants...)

		// values files will be merged with precedence on Assemble - we can namespace them here and order them accordingly
		for _, v := range importedPkg.Values.Files {
			// Namespace the values file under the component name and write to temp directory
			relativePath := makePathRelativeTo(v, importPath)
			namespacedPath, err := namespaceValuesFile(tempDir, component.Name, pkgPath.BaseDir, relativePath)
			if err != nil {
				return v1alpha1.ZarfPackage{}, fmt.Errorf("failed to namespace values file %s: %w", v, err)
			}
			valuesFiles = append(valuesFiles, namespacedPath)
		}

		// Namespace and merge the imported package's schema if it exists
		if importedPkg.Values.Schema != "" {
			schemaPath := makePathRelativeTo(importedPkg.Values.Schema, importPath)
			namespacedSchema, err := namespaceSchema(pkgPath.BaseDir, schemaPath, component.Name)
			if err != nil {
				return v1alpha1.ZarfPackage{}, fmt.Errorf("failed to namespace schema from %s: %w", importedPkg.Values.Schema, err)
			}
			mergedSchema = mergeSchemas(mergedSchema, namespacedSchema)
		}
	}

	valuesFiles = append(valuesFiles, pkg.Values.Files...)
	valuesFiles = slices.Compact(valuesFiles)
	pkg.Values.Files = valuesFiles
	pkg.Components = components

	// Merge imported schemas with parent schema and write to temp if needed
	if pkg.Values.Schema != "" {
		// Read the parent schema directly without namespacing
		schemaFullPath := pkg.Values.Schema
		if !filepath.IsAbs(schemaFullPath) {
			schemaFullPath = filepath.Join(pkgPath.BaseDir, pkg.Values.Schema)
		}
		content, err := os.ReadFile(schemaFullPath)
		if err != nil {
			return v1alpha1.ZarfPackage{}, fmt.Errorf("failed to read parent schema %s: %w", schemaFullPath, err)
		}
		var parentSchema map[string]any
		if err := json.Unmarshal(content, &parentSchema); err != nil {
			return v1alpha1.ZarfPackage{}, fmt.Errorf("failed to parse parent schema %s: %w", schemaFullPath, err)
		}
		mergedSchema = mergeSchemas(mergedSchema, parentSchema)
	}
	if mergedSchema != nil {
		schemaPath, err := writeMergedSchema(tempDir, mergedSchema)
		if err != nil {
			return v1alpha1.ZarfPackage{}, fmt.Errorf("failed to write merged schema: %w", err)
		}
		pkg.Values.Schema = schemaPath
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
func fetchOCISkeleton(ctx context.Context, component v1alpha1.ZarfComponent, packagePath string, cachePath string, remoteOptions types.RemoteOptions) (string, error) {
	if component.Import.URL == "" {
		return component.Import.Path, nil
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
	remote, err := zoci.NewRemote(ctx, component.Import.URL, zoci.PlatformForSkeleton(),
		oci.WithPlainHTTP(remoteOptions.PlainHTTP), oci.WithInsecureSkipVerify(remoteOptions.InsecureSkipTLSVerify))
	if err != nil {
		return "", err
	}
	_, err = remote.ResolveRoot(ctx)
	if err != nil {
		// This error likely won't occur as the root has been resolved before this function is invoked.
		// This serves as a secondary mechanism to highlight the potential for the package existing without a published skeleton.
		if strings.Contains(err.Error(), "no matching manifest was found in the manifest list") {
			return "", fmt.Errorf("package at %s exists but has not been published as a skeleton: %w", component.Import.URL, err)
		}
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

// namespaceTemplates updates the paths of templates to be namespaced by the component name
func namespaceTemplates(comp v1alpha1.ZarfComponent, packagePath string, tempDir *string) (v1alpha1.ZarfComponent, error) {
	// namespace chart values should replace sourcePath strings directly
	for chartIdx, chart := range comp.Charts {
		if len(chart.Values) > 0 {
			for valueIdx, value := range chart.Values {
				// sourcePath in chart values does not have a protected root - IE ".Values"
				comp.Charts[chartIdx].Values[valueIdx].SourcePath = fmt.Sprintf(".%s%s", comp.Name, value.SourcePath)
			}
		}
	}
	// namespace actions should evaluate replacing action contents
	namespaceActionTemplates(comp.Actions.OnDeploy.Before, comp.Name)
	namespaceActionTemplates(comp.Actions.OnDeploy.After, comp.Name)
	namespaceActionTemplates(comp.Actions.OnDeploy.OnFailure, comp.Name)
	namespaceActionTemplates(comp.Actions.OnDeploy.OnSuccess, comp.Name)
	// namespace on remove actions as well
	namespaceActionTemplates(comp.Actions.OnRemove.Before, comp.Name)
	namespaceActionTemplates(comp.Actions.OnRemove.After, comp.Name)
	namespaceActionTemplates(comp.Actions.OnRemove.OnFailure, comp.Name)
	namespaceActionTemplates(comp.Actions.OnRemove.OnSuccess, comp.Name)

	// namespace manifests should replace all instances of contents by reading/transforming/writing the file
	for manifestIdx, manifest := range comp.Manifests {
		if manifest.IsTemplate() {
			for fileIdx, file := range manifest.Files {
				// skipping remote files explicitly
				if helpers.IsURL(file) {
					continue
				}
				srcPath := filepath.Join(packagePath, file)
				tempPath, err := transformFileTemplates(tempDir, comp.Name, "manifests", file, srcPath, comp.Name)
				if err != nil {
					return comp, err
				}
				comp.Manifests[manifestIdx].Files[fileIdx] = tempPath
			}
		}
	}

	// namespace files should replace all instances of contents by reading/transforming/writing the file
	for fileIdx, file := range comp.Files {
		if file.IsTemplate() {
			// skipping remote files explicitly
			if helpers.IsURL(file.Source) {
				continue
			}
			srcPath := filepath.Join(packagePath, file.Source)
			tempPath, err := transformFileTemplates(tempDir, comp.Name, "files", file.Source, srcPath, comp.Name)
			if err != nil {
				return comp, err
			}
			comp.Files[fileIdx].Source = tempPath
		}
	}

	return comp, nil
}

// transformFileTemplates reads a file from srcPath, transforms template paths to be namespaced by key,
// and writes it to the temp directory. Returns the path to the transformed file.
func transformFileTemplates(tempDir *string, componentName, subdir, relativePath, srcPath, key string) (string, error) {
	info, err := os.Stat(srcPath)
	if err != nil {
		return "", fmt.Errorf("failed to stat file %s: %w", srcPath, err)
	}

	content, err := os.ReadFile(srcPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", srcPath, err)
	}

	transformed := template.InsertObjectKeyInContent(string(content), key)

	// Get or create the component temp directory with subdir
	componentDir, err := getOrCreateComponentTempDir(tempDir, componentName)
	if err != nil {
		return "", err
	}
	destDir := filepath.Join(componentDir, subdir)
	if err := os.MkdirAll(destDir, helpers.ReadWriteExecuteUser); err != nil {
		return "", fmt.Errorf("failed to create temp subdirectory: %w", err)
	}

	destPath := filepath.Join(destDir, filepath.Base(relativePath))
	if err := os.WriteFile(destPath, []byte(transformed), info.Mode()); err != nil {
		return "", fmt.Errorf("failed to write file %s: %w", destPath, err)
	}

	return destPath, nil
}

// namespaceActionTemplates transforms template paths in actions that have templating enabled.
func namespaceActionTemplates(actions []v1alpha1.ZarfComponentAction, key string) {
	for i, action := range actions {
		if action.ShouldTemplate() {
			actions[i].Cmd = template.InsertObjectKeyInContent(action.Cmd, key)
		}
	}
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

	for idx, imageArchive := range child.ImageArchives {
		composed := makePathRelativeTo(imageArchive.Path, relativeToHead)
		child.ImageArchives[idx].Path = composed
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
