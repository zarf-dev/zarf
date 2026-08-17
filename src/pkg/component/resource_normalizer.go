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
	"github.com/zarf-dev/zarf/src/pkg/value"
)

type componentResourceKind string

const (
	componentResourceKindValuesFile   componentResourceKind = "values-file"
	componentResourceKindValuesSchema componentResourceKind = "values-schema"
	componentResourceKindChart        componentResourceKind = "chart"
	componentResourceKindChartValues  componentResourceKind = "chart-values"
	componentResourceKindManifest     componentResourceKind = "manifest"
	componentResourceKindKustomize    componentResourceKind = "kustomize"
	componentResourceKindFile         componentResourceKind = "file"
	componentResourceKindImageLayout  componentResourceKind = "image-layout"
)

// componentResource describes a single file staged in a published component artifact.
type componentResource struct {
	sourcePath string
	contents   []byte
	kind       componentResourceKind
}

type normalizedComponentResources struct {
	resources     map[string]componentResource
	imageArchives []v1beta1.ImageArchive
	architecture  string
}

type componentResourceNormalizer struct {
	baseDir      string
	resources    map[string]componentResource
	sourceMounts map[string]string
	nextID       int
}

// normalizeComponentResources makes every local source path artifact-relative. This keeps the
// published config independent of the directory in which it was built and gives remote imports a
// stable mount contract.
func normalizeComponentResources(componentPath string, component v1beta1.ComponentConfig, importedSchemas []string) (v1beta1.ComponentConfig, normalizedComponentResources, error) {
	normalizer := componentResourceNormalizer{
		baseDir:      filepath.Dir(componentPath),
		resources:    map[string]componentResource{},
		sourceMounts: map[string]string{},
	}

	for i := range component.Values.Files {
		resourcePath, err := addLocalResource(&normalizer, component.Values.Files[i], componentResourceKindValuesFile)
		if err != nil {
			return component, normalizedComponentResources{}, err
		}
		component.Values.Files[i] = resourcePath
	}
	var err error
	if len(importedSchemas) > 0 {
		merged, err := value.MergeSchemaFiles(component.Values.Schema, importedSchemas, normalizer.baseDir)
		if err != nil {
			return component, normalizedComponentResources{}, fmt.Errorf("merging imported values schemas: %w", err)
		}
		contents, err := json.MarshalIndent(merged, "", "  ")
		if err != nil {
			return component, normalizedComponentResources{}, fmt.Errorf("marshaling imported values schemas: %w", err)
		}
		component.Values.Schema = normalizer.addGeneratedResource(layout.ValuesSchema, contents, componentResourceKindValuesSchema)
	} else {
		component.Values.Schema, err = addLocalResource(&normalizer, component.Values.Schema, componentResourceKindValuesSchema)
		if err != nil {
			return component, normalizedComponentResources{}, err
		}
	}

	for i := range component.Component.Charts {
		chart := &component.Component.Charts[i]
		if chart.Local != nil {
			chart.Local.Path, err = addLocalResource(&normalizer, chart.Local.Path, componentResourceKindChart)
			if err != nil {
				return component, normalizedComponentResources{}, err
			}
		}
		for j := range chart.ValuesFiles {
			chart.ValuesFiles[j].Path, err = addResource(&normalizer, chart.ValuesFiles[j].Path, componentResourceKindChartValues)
			if err != nil {
				return component, normalizedComponentResources{}, err
			}
		}
	}

	for i := range component.Component.Manifests {
		manifest := &component.Component.Manifests[i]
		for j := range manifest.Files {
			manifest.Files[j], err = addResource(&normalizer, manifest.Files[j], componentResourceKindManifest)
			if err != nil {
				return component, normalizedComponentResources{}, err
			}
		}
		if manifest.Kustomize != nil {
			for j := range manifest.Kustomize.Files {
				manifest.Kustomize.Files[j], err = addResource(&normalizer, manifest.Kustomize.Files[j], componentResourceKindKustomize)
				if err != nil {
					return component, normalizedComponentResources{}, err
				}
			}
		}
	}

	for i := range component.Component.Files {
		component.Component.Files[i].Source, err = addResource(&normalizer, component.Component.Files[i].Source, componentResourceKindFile)
		if err != nil {
			return component, normalizedComponentResources{}, err
		}
	}

	if hasActions(component.Component.Actions.OnCreate) {
		return component, normalizedComponentResources{}, fmt.Errorf("onCreate actions are not supported for published remote components")
	}
	if len(component.Component.Import.Remote) > 0 {
		return component, normalizedComponentResources{}, fmt.Errorf("remote component imports are not yet supported for v1beta1 packages")
	}

	imageArchives := make([]v1beta1.ImageArchive, len(component.Component.ImageArchives))
	for i := range component.Component.ImageArchives {
		archive := component.Component.ImageArchives[i]
		archive.Path = normalizer.absolutePath(archive.Path)
		imageArchives[i] = archive
		component.Component.ImageArchives[i].Path = filepath.ToSlash(string(layout.ImagesDir))
	}

	return component, normalizedComponentResources{
		resources:     normalizer.resources,
		imageArchives: imageArchives,
		architecture:  component.Metadata.Architecture,
	}, nil
}

func hasActions(actionSet v1beta1.ComponentActionSet) bool {
	return actionSet.Defaults != nil || len(actionSet.Before) > 0 || len(actionSet.OnSuccess) > 0 || len(actionSet.OnFailure) > 0
}

// addResource stages a local resource and leaves a remote URL for package creation to fetch.
func addResource(normalizer *componentResourceNormalizer, resourcePath string, kind componentResourceKind) (string, error) {
	if resourcePath == "" || helpers.IsURL(resourcePath) {
		return resourcePath, nil
	}
	return normalizer.add(resourcePath, kind)
}

// addLocalResource stages a resource that regular package assembly only supports from the local filesystem.
func addLocalResource(normalizer *componentResourceNormalizer, resourcePath string, kind componentResourceKind) (string, error) {
	if helpers.IsURL(resourcePath) {
		return "", fmt.Errorf("resource kind %s cannot be pulled from remote sources", kind)
	}
	return addResource(normalizer, resourcePath, kind)
}

func (n *componentResourceNormalizer) add(resourcePath string, kind componentResourceKind) (string, error) {
	absPath := n.absolutePath(resourcePath)
	if mountPath, ok := n.sourceMounts[absPath]; ok {
		return mountPath, nil
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("unable to access local component resource %q: %w", resourcePath, err)
	}

	mountPath := path.Join("resources", strconv.Itoa(n.nextID), filepath.Base(absPath))
	n.nextID++
	n.sourceMounts[absPath] = mountPath
	if !info.IsDir() {
		n.resources[mountPath] = componentResource{sourcePath: absPath, kind: kind}
		return mountPath, nil
	}

	err = filepath.WalkDir(absPath, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(absPath, filePath)
		if err != nil {
			return err
		}
		n.resources[path.Join(mountPath, filepath.ToSlash(rel))] = componentResource{sourcePath: filePath, kind: kind}
		return nil
	})
	if err != nil {
		return "", err
	}
	return mountPath, nil
}

func (n *componentResourceNormalizer) addGeneratedResource(filename string, contents []byte, kind componentResourceKind) string {
	mountPath := path.Join("resources", strconv.Itoa(n.nextID), filename)
	n.nextID++
	n.resources[mountPath] = componentResource{contents: contents, kind: kind}
	return mountPath
}

func (n *componentResourceNormalizer) absolutePath(resourcePath string) string {
	if filepath.IsAbs(resourcePath) {
		return filepath.Clean(resourcePath)
	}
	return filepath.Clean(filepath.Join(n.baseDir, resourcePath))
}
