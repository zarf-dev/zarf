// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package component

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/packager/load"
	"github.com/zarf-dev/zarf/src/pkg/value"
)

// componentResource describes a single file staged in a published component artifact.
type componentResource struct {
	sourcePath string
	contents   []byte
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
			chart.Local.Path, err = addLocalResource(&normalizer, chart.Local.Path)
			if err != nil {
				return component, normalizedComponentResources{}, err
			}
		}
		for j := range chart.ValuesFiles {
			chart.ValuesFiles[j].Path, err = addResource(&normalizer, chart.ValuesFiles[j].Path)
			if err != nil {
				return component, normalizedComponentResources{}, err
			}
		}
	}

	for i := range component.Component.Manifests {
		manifest := &component.Component.Manifests[i]
		for j := range manifest.Files {
			manifest.Files[j], err = addResource(&normalizer, manifest.Files[j])
			if err != nil {
				return component, normalizedComponentResources{}, err
			}
		}
		if manifest.Kustomize != nil {
			for j := range manifest.Kustomize.Files {
				manifest.Kustomize.Files[j], err = addResource(&normalizer, manifest.Kustomize.Files[j])
				if err != nil {
					return component, normalizedComponentResources{}, err
				}
			}
		}
	}

	for i := range component.Component.Files {
		component.Component.Files[i].Source, err = addResource(&normalizer, component.Component.Files[i].Source)
		if err != nil {
			return component, normalizedComponentResources{}, err
		}
	}

	if hasActions(component.Component.Actions.OnCreate) {
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
		architecture:  component.Metadata.Variant.Architecture,
	}, nil
}

func hasActions(actionSet v1beta1.ComponentActionSet) bool {
	return actionSet.Defaults != nil || len(actionSet.Before) > 0 || len(actionSet.OnSuccess) > 0 || len(actionSet.OnFailure) > 0
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
	info, err := os.Stat(sourcePath)
	if err != nil {
		return "", fmt.Errorf("unable to access local component resource %q: %w", resourcePath, err)
	}

	mountPath := path.Join("resources", strconv.Itoa(n.nextID), filepath.Base(sourcePath))
	n.nextID++
	n.sourceMounts[sourcePath] = mountPath
	if !info.IsDir() {
		n.resources[mountPath] = componentResource{sourcePath: sourcePath}
		return mountPath, nil
	}

	err = filepath.WalkDir(sourcePath, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(sourcePath, filePath)
		if err != nil {
			return err
		}
		n.resources[path.Join(mountPath, filepath.ToSlash(rel))] = componentResource{sourcePath: filePath}
		return nil
	})
	if err != nil {
		return "", err
	}
	return mountPath, nil
}

func (n *componentResourceNormalizer) addGeneratedResource(filename string, contents []byte) string {
	mountPath := path.Join("resources", strconv.Itoa(n.nextID), filename)
	n.nextID++
	n.resources[mountPath] = componentResource{contents: contents}
	return mountPath
}
