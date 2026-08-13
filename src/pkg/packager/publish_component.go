// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/pkg/oci"
	goyaml "github.com/goccy/go-yaml"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/load"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
	"github.com/zarf-dev/zarf/src/types"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry"
)

// ComponentConfigMediaType identifies a v1beta1 Zarf component config OCI artifact.
const ComponentConfigMediaType = "application/vnd.zarf.component.config.v1+yaml"

const componentLayerMediaType = "application/vnd.zarf.component.layer.v1.blob"

// PublishComponentOptions declares parameters for publishing a v1beta1 component config.
type PublishComponentOptions struct {
	// Flavor selects the component config variant to publish.
	Flavor string
	// OCIConcurrency configures the number of blobs pushed in parallel.
	OCIConcurrency int
	types.RemoteOptions
}

// PublishComponent validates a v1beta1 ZarfComponentConfig and publishes it as an OCI artifact.
// The component config is stored as the artifact's config blob; later remote-import support can
// retrieve it without treating it as a Zarf package.
func PublishComponent(ctx context.Context, componentPath string, destination registry.Reference, opts PublishComponentOptions) (registry.Reference, error) {
	if err := destination.ValidateRegistry(); err != nil {
		return registry.Reference{}, fmt.Errorf("invalid registry: %w", err)
	}

	component, err := load.ComponentConfig(componentPath)
	if err != nil {
		return registry.Reference{}, err
	}
	if component.Component.Selector.Flavor != "" && component.Component.Selector.Flavor != opts.Flavor {
		return registry.Reference{}, fmt.Errorf("component flavor %q does not match requested flavor %q", component.Component.Selector.Flavor, opts.Flavor)
	}
	if component.Metadata.Version == "" {
		return registry.Reference{}, errors.New("version is required for publishing")
	}

	component.PublishData.ZarfVersion = config.CLIVersion
	componentYAML, err := goyaml.Marshal(component)
	if err != nil {
		return registry.Reference{}, fmt.Errorf("unable to marshal component config: %w", err)
	}

	componentRef, err := componentReference(destination, component)
	if err != nil {
		return registry.Reference{}, err
	}
	store := memory.New()
	configDescriptor := content.NewDescriptorFromBytes(ComponentConfigMediaType, componentYAML)
	if err := store.Push(ctx, configDescriptor, bytes.NewReader(componentYAML)); err != nil {
		return registry.Reference{}, fmt.Errorf("unable to stage component config: %w", err)
	}
	layers, err := stageComponentResources(ctx, store, componentPath, component)
	if err != nil {
		return registry.Reference{}, err
	}
	manifest, err := oras.PackManifest(ctx, store, oras.PackManifestVersion1_1, "", oras.PackManifestOptions{
		ConfigDescriptor: &configDescriptor,
		Layers:           layers,
		ManifestAnnotations: map[string]string{
			ocispec.AnnotationTitle:          component.Metadata.Name,
			ocispec.AnnotationDescription:    component.Metadata.Description,
			ocispec.AnnotationVersion:        component.Metadata.Version,
			"zarf.dev/component-config-path": filepath.ToSlash(filepath.Base(componentPath)),
		},
	})
	if err != nil {
		return registry.Reference{}, fmt.Errorf("unable to create component artifact: %w", err)
	}
	if err := store.Tag(ctx, manifest, manifest.Digest.String()); err != nil {
		return registry.Reference{}, fmt.Errorf("unable to stage component artifact: %w", err)
	}

	remote, err := zoci.NewRemote(ctx, componentRef.String(), ocispec.Platform{}, oci.WithPlainHTTP(opts.PlainHTTP), oci.WithInsecureSkipVerify(opts.InsecureSkipTLSVerify))
	if err != nil {
		return registry.Reference{}, fmt.Errorf("unable to connect to component registry: %w", err)
	}

	copyOpts := remote.GetDefaultCopyOpts()
	if opts.OCIConcurrency > 0 {
		copyOpts.Concurrency = opts.OCIConcurrency
	}
	if _, err := oras.Copy(ctx, store, manifest.Digest.String(), remote.Repo(), componentRef.Reference, copyOpts); err != nil {
		return registry.Reference{}, fmt.Errorf("unable to publish component: %w", err)
	}
	logger.From(ctx).Info("published component", "destination", helpers.OCIURLPrefix+componentRef.String())
	return componentRef, nil
}

// stageComponentResources includes local component resources while leaving remote resources to
// be fetched when the component is imported during package creation.
func stageComponentResources(ctx context.Context, store *memory.Store, componentPath string, component v1beta1.ComponentConfig) ([]ocispec.Descriptor, error) {
	root := filepath.Dir(componentPath)
	resources, err := componentResources(componentPath, component, root, map[string]bool{})
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(resources))
	for rel := range resources {
		paths = append(paths, rel)
	}
	sort.Strings(paths)

	layers := make([]ocispec.Descriptor, 0, len(paths))
	for _, rel := range paths {
		contents, err := os.ReadFile(resources[rel])
		if err != nil {
			return nil, fmt.Errorf("unable to read component resource %q: %w", rel, err)
		}
		descriptor := content.NewDescriptorFromBytes(componentLayerMediaType, contents)
		descriptor.Annotations = map[string]string{ocispec.AnnotationTitle: filepath.ToSlash(rel)}
		exists, err := store.Exists(ctx, descriptor)
		if err != nil {
			return nil, fmt.Errorf("unable to check component resource %q: %w", rel, err)
		}
		if !exists {
			err = store.Push(ctx, descriptor, bytes.NewReader(contents))
		}
		if err != nil {
			return nil, fmt.Errorf("unable to stage component resource %q: %w", rel, err)
		}
		layers = append(layers, descriptor)
	}
	return layers, nil
}

func componentResources(componentPath string, component v1beta1.ComponentConfig, root string, seen map[string]bool) (map[string]string, error) {
	resources := map[string]string{}
	if err := collectComponentResources(componentPath, component, root, seen, resources); err != nil {
		return nil, err
	}
	return resources, nil
}

func collectComponentResources(componentPath string, component v1beta1.ComponentConfig, root string, seen map[string]bool, resources map[string]string) error {
	componentPath = filepath.Clean(componentPath)
	if seen[componentPath] {
		return nil
	}
	seen[componentPath] = true
	baseDir := filepath.Dir(componentPath)

	add := func(resourcePath string) error {
		if resourcePath == "" || helpers.IsURL(resourcePath) {
			return nil
		}
		return addComponentResource(root, baseDir, resourcePath, resources)
	}
	for _, file := range component.Values.Files {
		if err := add(file); err != nil {
			return err
		}
	}
	if err := add(component.Values.Schema); err != nil {
		return err
	}
	for _, chart := range component.Component.Charts {
		if chart.Local != nil {
			if err := add(chart.Local.Path); err != nil {
				return err
			}
		}
		for _, values := range chart.ValuesFiles {
			if err := add(values.Path); err != nil {
				return err
			}
		}
	}
	for _, manifest := range component.Component.Manifests {
		for _, file := range manifest.Files {
			if err := add(file); err != nil {
				return err
			}
		}
		if manifest.Kustomize != nil {
			for _, file := range manifest.Kustomize.Files {
				if err := add(file); err != nil {
					return err
				}
			}
		}
	}
	for _, file := range component.Component.Files {
		if err := add(file.Source); err != nil {
			return err
		}
	}
	for _, archive := range component.Component.ImageArchives {
		if err := add(archive.Path); err != nil {
			return err
		}
	}
	for _, imported := range component.Component.Import.Local {
		importPath := filepath.Join(baseDir, imported.Path)
		if err := addComponentResource(root, baseDir, imported.Path, resources); err != nil {
			return err
		}
		importedComponent, err := load.ComponentConfig(importPath)
		if err != nil {
			return err
		}
		if err := collectComponentResources(importPath, importedComponent, root, seen, resources); err != nil {
			return err
		}
	}
	return nil
}

func addComponentResource(root, baseDir, resourcePath string, resources map[string]string) error {
	absPath := resourcePath
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(baseDir, resourcePath)
	}
	absPath = filepath.Clean(absPath)
	rel, err := filepath.Rel(root, absPath)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("local component resource %q must be within %q", resourcePath, root)
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("unable to access local component resource %q: %w", resourcePath, err)
	}
	if !info.IsDir() {
		resources[rel] = absPath
		return nil
	}
	return filepath.WalkDir(absPath, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		fileRel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		resources[fileRel] = path
		return nil
	})
}

func componentReference(destination registry.Reference, component v1beta1.ComponentConfig) (registry.Reference, error) {
	tag := component.Metadata.Version
	if component.Component.Selector.Flavor != "" {
		tag += "-" + component.Component.Selector.Flavor
	}
	return registry.ParseReference(fmt.Sprintf("%s/%s:%s", destination.Registry, path.Join(destination.Repository, component.Metadata.Name), tag))
}
