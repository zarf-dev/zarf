// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package component

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"

	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/pkg/helpers"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/packager/load"
	"github.com/zarf-dev/zarf/src/pkg/value"
)

// componentResource describes a file or directory staged in a published component artifact.
type componentResource struct {
	sourcePath string
	contents   []byte
	mode       os.FileMode
}

type normalizedComponentResources struct {
	resources     map[string]componentResource
	imageArchives []v1beta1.ImageArchive
	architecture  string
}

type componentResourceNormalizer struct {
	baseDir      string
	resourceSet  *load.ResourceSet
	resources    map[string]componentResource
	sourceMounts map[string]string
	nextID       int
}

// normalizeComponentResources makes every local source path artifact-relative. This keeps the
// published config independent of the directory in which it was built and gives remote imports a
// stable mount contract.
func normalizeComponentResources(componentPath string, component v1beta1.ComponentConfig, importedSchemas []string, resourceSet *load.ResourceSet) (v1beta1.ComponentConfig, normalizedComponentResources, error) {
	normalizer := componentResourceNormalizer{
		baseDir:      filepath.Dir(componentPath),
		resourceSet:  resourceSet,
		resources:    map[string]componentResource{},
		sourceMounts: map[string]string{},
	}

	for i := range component.Values.Files {
		resourcePath, err := addLocalResource(&normalizer, component.Values.Files[i])
		if err != nil {
			return component, normalizedComponentResources{}, err
		}
		component.Values.Files[i] = resourcePath
	}
	var err error
	if len(importedSchemas) > 0 {
		for i := range importedSchemas {
			importedSchemas[i], err = normalizer.resourceSet.Path(importedSchemas[i])
			if err != nil {
				return component, normalizedComponentResources{}, err
			}
		}
		merged, err := value.MergeSchemaFiles(component.Values.Schema, importedSchemas, normalizer.baseDir)
		if err != nil {
			return component, normalizedComponentResources{}, fmt.Errorf("merging imported values schemas: %w", err)
		}
		contents, err := json.MarshalIndent(merged, "", "  ")
		if err != nil {
			return component, normalizedComponentResources{}, fmt.Errorf("marshaling imported values schemas: %w", err)
		}
		component.Values.Schema = normalizer.addGeneratedResource(layout.ValuesSchema, contents)
	} else {
		component.Values.Schema, err = addLocalResource(&normalizer, component.Values.Schema)
		if err != nil {
			return component, normalizedComponentResources{}, err
		}
	}

	for i := range component.Component.Charts {
		chart := &component.Component.Charts[i]
		if chart.Local != nil {
			if helpers.IsURL(chart.Local.Path) {
				return component, normalizedComponentResources{}, fmt.Errorf("resource %q must be local", chart.Local.Path)
			}
			localPath, pathErr := resourceSet.Path(chart.Local.Path)
			if pathErr != nil {
				return component, normalizedComponentResources{}, pathErr
			}
			if helpers.IsDir(localPath) {
				chart.Local.Path, err = normalizer.add(chart.Local.Path)
			} else {
				chart.Local.Path, err = normalizer.addCopiedResource(chart.Local.Path, path.Join(string(layout.ChartsComponentDir), layout.ChartArchiveName(chart.Name, "")), 0)
			}
			if err != nil {
				return component, normalizedComponentResources{}, err
			}
		}
		for j := range chart.ValuesFiles {
			chart.ValuesFiles[j].Path, err = normalizer.addCopiedResource(chart.ValuesFiles[j].Path, path.Join(string(layout.ValuesComponentDir), layout.ChartValuesFileName(chart.Name, "", j)), 0)
			if err != nil {
				return component, normalizedComponentResources{}, err
			}
		}
	}

	for i := range component.Component.Manifests {
		manifest := &component.Component.Manifests[i]
		for j := range manifest.Files {
			manifest.Files[j], err = normalizer.addCopiedResource(manifest.Files[j], path.Join(string(layout.ManifestsComponentDir), layout.ManifestFileName(manifest.Name, j)), 0)
			if err != nil {
				return component, normalizedComponentResources{}, err
			}
		}
		for j := range manifest.Kustomize.Files {
			manifest.Kustomize.Files[j], err = addResource(&normalizer, manifest.Kustomize.Files[j])
			if err != nil {
				return component, normalizedComponentResources{}, err
			}
		}
	}

	for i := range component.Component.Files {
		file := &component.Component.Files[i]
		if file.ExtractPath != "" {
			file.Source, err = addResource(&normalizer, file.Source)
		} else {
			mode := os.FileMode(helpers.ReadWriteUser)
			if file.Executable {
				mode = helpers.ReadWriteExecuteUser
			}
			file.Source, err = normalizer.addCopiedResource(file.Source, path.Join(string(layout.FilesComponentDir), filepath.ToSlash(layout.ComponentFileRelPath(i, file.Destination))), mode)
		}
		if err != nil {
			return component, normalizedComponentResources{}, err
		}
	}

	if load.HasActionSet(component.Component.Actions.OnCreate) {
		return component, normalizedComponentResources{}, fmt.Errorf("onCreate actions are not supported for published remote components")
	}

	imageArchives := make([]v1beta1.ImageArchive, len(component.Component.ImageArchives))
	for i := range component.Component.ImageArchives {
		archive := component.Component.ImageArchives[i]
		archive.Path, err = normalizer.resourceSet.Path(archive.Path)
		if err != nil {
			return component, normalizedComponentResources{}, err
		}
		imageArchives[i] = archive
		component.Component.ImageArchives[i].Path = filepath.ToSlash(string(layout.ImagesDir))
	}

	return component, normalizedComponentResources{
		resources:     normalizer.resources,
		imageArchives: imageArchives,
		architecture:  component.Variant.Architecture,
	}, nil
}

// addResource stages a local resource and leaves a remote URL for package creation to fetch.
func addResource(normalizer *componentResourceNormalizer, resourcePath string) (string, error) {
	if resourcePath == "" || helpers.IsURL(resourcePath) {
		return resourcePath, nil
	}
	return normalizer.add(resourcePath)
}

// addLocalResource stages a resource that regular package assembly only supports from the local filesystem.
func addLocalResource(normalizer *componentResourceNormalizer, resourcePath string) (string, error) {
	if helpers.IsURL(resourcePath) {
		return "", fmt.Errorf("resource %q must be local", resourcePath)
	}
	return addResource(normalizer, resourcePath)
}

func (n *componentResourceNormalizer) add(resourcePath string) (string, error) {
	sourcePath, err := n.resourceSet.Path(resourcePath)
	if err != nil {
		return "", err
	}
	if mountPath, ok := n.sourceMounts[sourcePath]; ok {
		return mountPath, nil
	}
	mountPath := path.Join("resources", strconv.Itoa(n.nextID), filepath.Base(sourcePath))
	n.nextID++
	n.sourceMounts[sourcePath] = mountPath
	n.resources[mountPath] = componentResource{sourcePath: sourcePath}
	return mountPath, nil
}

// addCopiedResource uses the same path and permissions as package assembly for resources copied verbatim.
// Inputs needing create-time processing retain their source paths under resources/.
func (n *componentResourceNormalizer) addCopiedResource(resourcePath, mountPath string, mode os.FileMode) (string, error) {
	if resourcePath == "" || helpers.IsURL(resourcePath) {
		return resourcePath, nil
	}
	sourcePath, err := n.resourceSet.Path(resourcePath)
	if err != nil {
		return "", err
	}
	if mode != 0 && helpers.IsDir(sourcePath) {
		mode = helpers.ReadWriteExecuteUser
	}
	n.resources[mountPath] = componentResource{sourcePath: sourcePath, mode: mode}
	return mountPath, nil
}

func (n *componentResourceNormalizer) addGeneratedResource(filename string, contents []byte) string {
	mountPath := path.Join("resources", strconv.Itoa(n.nextID), filename)
	n.nextID++
	n.resources[mountPath] = componentResource{contents: contents}
	return mountPath
}
