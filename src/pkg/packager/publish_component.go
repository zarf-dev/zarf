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
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/pkg/oci"
	goyaml "github.com/goccy/go-yaml"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/images"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/load"
	"github.com/zarf-dev/zarf/src/pkg/signing"
	"github.com/zarf-dev/zarf/src/pkg/utils"
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
	// SignBlobOptions configures OCI artifact signing for the published component.
	SignBlobOptions signing.SignBlobOptions
	// OCIConcurrency configures the number of blobs pushed in parallel.
	OCIConcurrency int
	// Retries is the number of attempts to make when publishing fails.
	Retries int
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

	componentRef, err := componentReference(destination, component)
	if err != nil {
		return registry.Reference{}, err
	}
	// The flavor is selected by this publish operation and represented by the artifact tag.
	// Do not require the same selection again when the published component is imported.
	component.Component.Selector.Flavor = ""
	component.PublishData.ZarfVersion = config.CLIVersion
	componentYAML, err := goyaml.Marshal(component)
	if err != nil {
		return registry.Reference{}, fmt.Errorf("unable to marshal component config: %w", err)
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

	totalSize := configDescriptor.Size + manifest.Size
	for _, layer := range layers {
		totalSize += layer.Size
	}
	published, err := pushComponentArtifact(ctx, store, manifest.Digest.String(), remote, componentRef, totalSize, opts)
	if err != nil {
		return registry.Reference{}, err
	}
	if opts.SignBlobOptions.ShouldSign() {
		artifactRef := fmt.Sprintf("%s/%s@%s", componentRef.Registry, componentRef.Repository, published.Digest)
		logger.From(ctx).Info("signing published component", "reference", artifactRef)
		if err := signing.CosignSignImageWithOptions(ctx, artifactRef, opts.SignBlobOptions, opts.PlainHTTP, opts.InsecureSkipTLSVerify); err != nil {
			return registry.Reference{}, fmt.Errorf("component was published but signing artifact %q failed: %w", artifactRef, err)
		}
	}
	logger.From(ctx).Info("published component", "destination", helpers.OCIURLPrefix+componentRef.String())
	return componentRef, nil
}

func pushComponentArtifact(ctx context.Context, store *memory.Store, sourceRef string, remote *zoci.Remote, componentRef registry.Reference, totalSize int64, opts PublishComponentOptions) (_ ocispec.Descriptor, err error) {
	l := logger.From(ctx)
	start := time.Now()

	if opts.OCIConcurrency == 0 {
		opts.OCIConcurrency = zoci.DefaultConcurrency
	}
	if opts.Retries <= 0 {
		if opts.Retries < 0 {
			return ocispec.Descriptor{}, errors.New("retries cannot be negative")
		}
		l.Debug("retries set to default", "retries", zoci.DefaultRetries)
		opts.Retries = zoci.DefaultRetries
	}

	copyOpts := remote.GetDefaultCopyOpts()
	copyOpts.Concurrency = opts.OCIConcurrency

	var published ocispec.Descriptor
	err = retry.Do(
		func() error {
			l.Info("pushing component to registry", "destination", componentRef.String(), "size", utils.ByteFormat(float64(totalSize), 2))
			trackedRemote := images.NewTrackedTarget(
				remote.Repo(),
				totalSize,
				images.DefaultReport(l, "component publish in progress", componentRef.String()),
			)
			trackedRemote.StartReporting(ctx)
			defer trackedRemote.StopReporting()

			var copyErr error
			published, copyErr = oras.Copy(ctx, store, sourceRef, trackedRemote, componentRef.Reference, copyOpts)
			return copyErr
		},
		retry.Attempts(uint(opts.Retries)),
		retry.Delay(500*time.Millisecond),
		retry.MaxDelay(8*time.Second),
		retry.DelayType(retry.BackOffDelay),
		retry.LastErrorOnly(true),
		retry.Context(ctx),
		retry.OnRetry(func(n uint, retryErr error) {
			if opts.Retries > 1 && n+1 < uint(opts.Retries) {
				l.Warn("retrying component push", "attempt", n+1, "maxAttempts", opts.Retries, "error", retryErr)
			}
		}),
	)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("component publish failed: %w", err)
	}

	l.Info("completed component publish", "destination", componentRef.String(), "duration", time.Since(start).Round(100*time.Millisecond))
	return published, nil
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

// FIXME: not sure I love passing around resource maps
func collectComponentResources(componentPath string, component v1beta1.ComponentConfig, root string, seen map[string]bool, resources map[string]string) error {
	componentPath = filepath.Clean(componentPath)
	if seen[componentPath] {
		return nil
	}
	seen[componentPath] = true
	baseDir := filepath.Dir(componentPath)

	for _, file := range component.Values.Files {
		if err := addLocalResource(root, baseDir, file, "values files", resources); err != nil {
			return err
		}
	}
	if err := addLocalResource(root, baseDir, component.Values.Schema, "values schemas", resources); err != nil {
		return err
	}
	for _, chart := range component.Component.Charts {
		if chart.Local != nil {
			if err := addLocalResource(root, baseDir, chart.Local.Path, "local chart paths", resources); err != nil {
				return err
			}
		}
		for _, values := range chart.ValuesFiles {
			if err := addResource(root, baseDir, values.Path, resources); err != nil {
				return err
			}
		}
	}
	for _, manifest := range component.Component.Manifests {
		for _, file := range manifest.Files {
			if err := addResource(root, baseDir, file, resources); err != nil {
				return err
			}
		}
		if manifest.Kustomize != nil {
			for _, file := range manifest.Kustomize.Files {
				if err := addResource(root, baseDir, file, resources); err != nil {
					return err
				}
			}
		}
	}
	for _, file := range component.Component.Files {
		if err := addResource(root, baseDir, file.Source, resources); err != nil {
			return err
		}
	}
	for _, archive := range component.Component.ImageArchives {
		if err := addLocalResource(root, baseDir, archive.Path, "image archive paths", resources); err != nil {
			return err
		}
	}
	if len(component.Component.Import.Remote) > 0 {
		return fmt.Errorf("remote component imports are not yet supported for v1beta1 packages")
	}
	for _, imported := range component.Component.Import.Local {
		if err := addLocalResource(root, baseDir, imported.Path, "local import paths", resources); err != nil {
			return err
		}
		importPath := filepath.Join(baseDir, imported.Path)
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

// addResource stages a local resource and leaves a remote URL for package creation to fetch.
func addResource(root, baseDir, resourcePath string, resources map[string]string) error {
	if resourcePath == "" || helpers.IsURL(resourcePath) {
		return nil
	}
	return addComponentResource(root, baseDir, resourcePath, resources)
}

// addLocalResource stages a resource that regular package assembly only supports from the local filesystem.
func addLocalResource(root, baseDir, resourcePath, field string, resources map[string]string) error {
	if helpers.IsURL(resourcePath) {
		return fmt.Errorf("remote %s are not supported", field)
	}
	return addResource(root, baseDir, resourcePath, resources)
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
