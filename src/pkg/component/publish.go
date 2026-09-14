// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package component handles publishing remote components
package component

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"time"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/images"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/packager/load"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
	"github.com/zarf-dev/zarf/src/types"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	ocistore "oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/errdef"
	"oras.land/oras-go/v2/registry"
)

const componentLayerMediaType = "application/vnd.zarf.component.layer.v1.blob"

// PublishOptions declares parameters for publishing a v1beta1 component config.
type PublishOptions struct {
	// OCIConcurrency configures the number of blobs pushed in parallel.
	OCIConcurrency int
	// Retries is the number of attempts to make when publishing fails.
	Retries int
	types.RemoteOptions
}

// Publish validates a v1beta1 ZarfComponentConfig and publishes it as an OCI artifact.
// The component config is stored as the artifact's config blob; later remote-import support can
// retrieve it without treating it as a Zarf package.
func Publish(ctx context.Context, componentPath string, destination registry.Reference, opts PublishOptions) (registry.Reference, error) {
	if err := destination.ValidateRegistry(); err != nil {
		return registry.Reference{}, fmt.Errorf("invalid registry: %w", err)
	}

	component, err := load.ComponentConfig(componentPath)
	if err != nil {
		return registry.Reference{}, err
	}
	if component.Metadata.Version == "" {
		return registry.Reference{}, errors.New("version is required for publishing")
	}
	resolved, err := load.ResolveComponentConfigImports(ctx, component, componentPath, opts.RemoteOptions)
	if err != nil {
		return registry.Reference{}, fmt.Errorf("unable to resolve component imports: %w", err)
	}
	component = resolved.Component
	resourceSet, err := resolved.MaterializeResources(ctx, componentPath)
	if err != nil {
		return registry.Reference{}, fmt.Errorf("unable to materialize imported component resources: %w", err)
	}
	defer func() {
		if err := resourceSet.Close(); err != nil {
			logger.From(ctx).Warn("unable to clean up imported component resources", "error", err)
		}
	}()

	componentRef, err := componentReference(destination, component)
	if err != nil {
		return registry.Reference{}, err
	}
	component.PublishData.ZarfVersion = config.CLIVersion
	component, resources, err := normalizeComponentResources(componentPath, component, resolved.ImportedSchemas, resourceSet)
	if err != nil {
		return registry.Reference{}, err
	}
	componentJSON, err := json.Marshal(component)
	if err != nil {
		return registry.Reference{}, fmt.Errorf("unable to marshal component config: %w", err)
	}

	stagingDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return registry.Reference{}, fmt.Errorf("unable to create component staging directory: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(stagingDir); err != nil {
			logger.From(ctx).Warn("unable to clean up component staging directory", "error", err)
		}
	}()

	store, err := ocistore.NewWithContext(ctx, stagingDir)
	if err != nil {
		return registry.Reference{}, fmt.Errorf("unable to create component staging store: %w", err)
	}
	configDescriptor := content.NewDescriptorFromBytes(layout.ZarfComponentConfigMediaType, componentJSON)
	if err := store.Push(ctx, configDescriptor, bytes.NewReader(componentJSON)); err != nil {
		return registry.Reference{}, fmt.Errorf("unable to stage component config: %w", err)
	}
	layers, err := stageComponentResources(ctx, store, resources)
	if err != nil {
		return registry.Reference{}, err
	}
	manifest, err := oras.PackManifest(ctx, store, oras.PackManifestVersion1_1, "", oras.PackManifestOptions{
		ConfigDescriptor: &configDescriptor,
		Layers:           layers,
		ManifestAnnotations: map[string]string{
			ocispec.AnnotationTitle:       component.Metadata.Name,
			ocispec.AnnotationDescription: component.Metadata.Description,
			ocispec.AnnotationVersion:     component.Metadata.Version,
		},
	})
	if err != nil {
		return registry.Reference{}, fmt.Errorf("unable to create component artifact: %w", err)
	}
	if err := store.Tag(ctx, manifest, manifest.Digest.String()); err != nil {
		return registry.Reference{}, fmt.Errorf("unable to stage component artifact: %w", err)
	}

	remote, err := zoci.NewRemoteWithOptions(ctx, componentRef.String(), ocispec.Platform{Architecture: component.Variant.Architecture}, zoci.RemoteClientOptions{
		RemoteOptions: opts.RemoteOptions,
	})
	if err != nil {
		return registry.Reference{}, fmt.Errorf("unable to connect to component registry: %w", err)
	}

	totalSize := configDescriptor.Size + manifest.Size
	for _, layer := range layers {
		totalSize += layer.Size
	}
	_, err = pushComponentArtifact(ctx, store, manifest.Digest.String(), remote, componentRef, component.Variant.Architecture, totalSize, opts)
	if err != nil {
		return registry.Reference{}, err
	}
	logger.From(ctx).Info("published component", "destination", helpers.OCIURLPrefix+componentRef.String())
	return componentRef, nil
}

func pushComponentArtifact(ctx context.Context, store oras.ReadOnlyTarget, sourceRef string, remote *zoci.Remote, componentRef registry.Reference, architecture string, totalSize int64, opts PublishOptions) (_ ocispec.Descriptor, err error) {
	l := logger.From(ctx)
	start := time.Now()

	if opts.OCIConcurrency == 0 {
		opts.OCIConcurrency = zoci.DefaultConcurrency
	}
	copyOpts := remote.GetDefaultCopyOpts()
	copyOpts.Concurrency = opts.OCIConcurrency

	var published ocispec.Descriptor
	err = zoci.Retry(ctx, opts.Retries,
		func() error {
			existing, err := remote.Repo().Resolve(ctx, componentRef.Reference)
			if err != nil && !errors.Is(err, errdef.ErrNotFound) {
				return err
			}
			if architecture == "" && err == nil && existing.MediaType == ocispec.MediaTypeImageIndex {
				return fmt.Errorf("cannot publish architecture-generic component: %s already contains architecture-specific variants", componentRef.String())
			}
			if architecture != "" && err == nil && existing.MediaType != ocispec.MediaTypeImageIndex {
				return fmt.Errorf("cannot publish architecture-specific component: %s already contains an architecture-generic artifact", componentRef.String())
			}
			l.Info("pushing component to registry", "destination", componentRef.String(), "size", utils.ByteFormat(float64(totalSize), 2))
			trackedRemote := images.NewTrackedTarget(
				remote.Repo(),
				totalSize,
				images.DefaultReport(l, "component publish in progress", componentRef.String()),
			)
			trackedRemote.StartReporting(ctx)
			defer trackedRemote.StopReporting()

			var copyErr error
			destinationRef := componentRef.Reference
			if architecture != "" {
				destinationRef = ""
			}
			published, copyErr = oras.Copy(ctx, store, sourceRef, trackedRemote, destinationRef, copyOpts)
			if copyErr != nil {
				return copyErr
			}
			if architecture != "" {
				return remote.UpdateIndex(ctx, componentRef.Reference, published)
			}
			return nil
		},
	)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("component publish failed: %w", err)
	}

	l.Info("completed component publish", "destination", componentRef.String(), "duration", time.Since(start).Round(100*time.Millisecond))
	return published, nil
}

// stageComponentResources includes local component resources while leaving remote resources to
// be fetched when the component is imported during package creation.
func stageComponentResources(ctx context.Context, store content.Storage, resources normalizedComponentResources) ([]ocispec.Descriptor, error) {
	cleanupImageLayout, err := addComponentImageLayout(ctx, resources.imageArchives, resources.architecture, resources.resources)
	if err != nil {
		return nil, err
	}
	defer cleanupImageLayout()
	paths := make([]string, 0, len(resources.resources))
	for rel := range resources.resources {
		paths = append(paths, rel)
	}
	sort.Strings(paths)

	layers := make([]ocispec.Descriptor, 0, len(paths))
	for _, rel := range paths {
		resource := resources.resources[rel]
		descriptor, reader, err := componentResourceDescriptor(resource)
		if err != nil {
			return nil, fmt.Errorf("unable to read component resource %q: %w", rel, err)
		}
		descriptor.Annotations = map[string]string{
			ocispec.AnnotationTitle:                     rel,
			layout.ComponentResourceMountPathAnnotation: rel,
		}
		exists, err := store.Exists(ctx, descriptor)
		if err != nil {
			return nil, fmt.Errorf("unable to check component resource %q: %w", rel, err)
		}
		if !exists {
			err = store.Push(ctx, descriptor, reader)
		}
		closeErr := reader.Close()
		if err != nil {
			return nil, fmt.Errorf("unable to stage component resource %q: %w", rel, err)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("unable to close component resource %q: %w", rel, closeErr)
		}
		layers = append(layers, descriptor)
	}
	return layers, nil
}

// componentResourceDescriptor creates a descriptor and a fresh reader for a component resource.
// File-backed resources are hashed and staged as streams so large component artifacts are never
// retained in process memory.
func componentResourceDescriptor(resource componentResource) (ocispec.Descriptor, io.ReadCloser, error) {
	if resource.contents != nil {
		return content.NewDescriptorFromBytes(componentLayerMediaType, resource.contents), io.NopCloser(bytes.NewReader(resource.contents)), nil
	}

	file, err := os.Open(resource.sourcePath)
	if err != nil {
		return ocispec.Descriptor{}, nil, err
	}
	digestValue, digestErr := digest.FromReader(file)
	closeErr := file.Close()
	if err := errors.Join(digestErr, closeErr); err != nil {
		return ocispec.Descriptor{}, nil, err
	}
	info, err := os.Stat(resource.sourcePath)
	if err != nil {
		return ocispec.Descriptor{}, nil, err
	}
	reader, err := os.Open(resource.sourcePath)
	if err != nil {
		return ocispec.Descriptor{}, nil, err
	}
	return ocispec.Descriptor{
		MediaType: componentLayerMediaType,
		Digest:    digestValue,
		Size:      info.Size(),
	}, reader, nil
}

// addComponentImageLayout expands image archives into the OCI layout used by regular packages.
// The layout is included as artifact layers rather than preserving the source archive itself. An
// empty architecture preserves every image platform; a selector architecture retains only that platform.
func addComponentImageLayout(ctx context.Context, archives []v1beta1.ImageArchive, architecture string, resources map[string]componentResource) (_ func(), err error) {
	if len(archives) == 0 {
		return func() {}, nil
	}

	tempDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return nil, fmt.Errorf("unable to create image layout: %w", err)
	}
	cleanup := func() {
		if err := os.RemoveAll(tempDir); err != nil {
			logger.From(ctx).Warn("unable to remove component image layout", "path", tempDir, "error", err)
		}
	}
	defer func() {
		if err != nil {
			cleanup()
		}
	}()

	imageDir := filepath.Join(tempDir, layout.ImagesDir)
	for _, archive := range archives {
		_, err := images.Unpack(ctx, v1alpha1.ImageArchive{Path: archive.Path, Images: archive.Images}, imageDir, architecture)
		if err != nil {
			return nil, fmt.Errorf("unable to unpack image archive %q: %w", archive.Path, err)
		}
	}
	if err := utils.SortImagesIndex(imageDir); err != nil {
		return nil, fmt.Errorf("unable to sort component image layout: %w", err)
	}

	err = filepath.WalkDir(imageDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(tempDir, path)
		if err != nil {
			return err
		}
		resources[filepath.ToSlash(rel)] = componentResource{sourcePath: path}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return cleanup, nil
}

func componentReference(destination registry.Reference, component v1beta1.ComponentConfig) (registry.Reference, error) {
	tag := component.Metadata.Version
	if component.Variant.Flavor != "" {
		tag += "-" + component.Variant.Flavor
	}
	return registry.ParseReference(fmt.Sprintf("%s/%s:%s", destination.Registry, path.Join(destination.Repository, component.Metadata.Name), tag))
}
