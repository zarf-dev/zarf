// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package zoci contains functions for interacting with Zarf packages stored in OCI registries.
package zoci

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/internal/checksum"
	"github.com/zarf-dev/zarf/src/pkg/helpers"
	"github.com/zarf-dev/zarf/src/pkg/images"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/oci"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/file"
)

var (
	// PackageAlwaysPull is a list of paths that will always be pulled from the remote repository.
	PackageAlwaysPull = []string{layout.ZarfYAML, layout.Checksums, layout.Signature, layout.Bundle, layout.ValuesYAML, layout.ValuesSchema} //nolint:staticcheck // layout.Signature intentionally included for backward-compat with pre-v0.72.0 packages
)

// PullPackage pulls the package from the remote repository and saves it to the given path.
func (r *Remote) PullPackage(ctx context.Context, destinationDir string, concurrency int, layersToPull ...ocispec.Descriptor) (_ []ocispec.Descriptor, err error) {
	start := time.Now()
	// layersToPull is an explicit requirement for pulling package layers
	if len(layersToPull) == 0 {
		return nil, fmt.Errorf("no layers to pull")
	}

	if concurrency == 0 {
		concurrency = DefaultConcurrency
	}

	layerSize := oci.SumDescsSize(layersToPull)
	logger.From(ctx).Info("pulling package", "name", r.Repo().Reference, "size", utils.ByteFormat(float64(layerSize), 2))

	dst, err := file.New(destinationDir)
	if err != nil {
		return nil, err
	}
	// Zarf lays out package layers itself and never relies on oras-go's annotation-driven auto-unpack
	dst.SkipUnpack = true
	defer func(dst *file.Store) {
		err2 := dst.Close()
		err = errors.Join(err, err2)
	}(dst)

	copyOpts := r.GetDefaultCopyOpts()
	copyOpts.Concurrency = concurrency

	trackedDst := images.NewTrackedTarget(dst, layerSize, images.DefaultReport(r.Log(), "package pull in progress", r.Repo().Reference.String()))
	trackedDst.StartReporting(ctx)
	defer trackedDst.StopReporting()

	err = r.CopyToTarget(ctx, layersToPull, trackedDst, copyOpts)
	if err != nil {
		return nil, err
	}
	if err := restoreDuplicateLayers(destinationDir, layersToPull); err != nil {
		return nil, err
	}

	r.Log().Info("finished pulling package layers", "duration", time.Since(start).Round(time.Millisecond*100))
	return layersToPull, nil
}

// restoreDuplicateLayers restores each package file represented by duplicate OCI descriptors.
// ORAS transfers each content digest once, so only one of several differently named files
// with identical content is guaranteed to be materialized by the file store.
func restoreDuplicateLayers(destinationDir string, layers []ocispec.Descriptor) error {
	pathsByDigest := make(map[string][]string, len(layers))
	for _, layer := range layers {
		title := layer.Annotations[ocispec.AnnotationTitle]
		if title == "" {
			continue
		}
		path, err := packageLayerPath(destinationDir, title)
		if err != nil {
			return err
		}
		pathsByDigest[layer.Digest.String()] = append(pathsByDigest[layer.Digest.String()], path)
	}

	for digest, paths := range pathsByDigest {
		sourcePath := ""
		sourceIndex := -1
		for index, path := range paths {
			info, err := os.Lstat(path)
			if err == nil {
				if !info.Mode().IsRegular() {
					return fmt.Errorf("pulled layer %s at %q is not a regular file", digest, path)
				}
				sourcePath = path
				sourceIndex = index
				break
			}
			if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("checking pulled layer %s at %q: %w", digest, path, err)
			}
		}
		if sourcePath == "" {
			return fmt.Errorf("pulled layer %s was not materialized", digest)
		}

		for index, destinationPath := range paths {
			if destinationPath == sourcePath {
				continue
			}
			// Paths before the source were already observed missing while locating it.
			if index > sourceIndex {
				if _, err := os.Lstat(destinationPath); err == nil {
					continue
				} else if !errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("checking duplicate layer destination %q: %w", destinationPath, err)
				}
			}
			if err := helpers.CreateParentDirectory(destinationPath); err != nil {
				return fmt.Errorf("creating duplicate layer destination for %s: %w", digest, err)
			}
			if err := os.Link(sourcePath, destinationPath); err != nil {
				if err := helpers.CreatePathAndCopy(sourcePath, destinationPath); err != nil {
					return fmt.Errorf("restoring duplicate layer %s at %q: %w", digest, destinationPath, err)
				}
			}
		}
	}
	return nil
}

func packageLayerPath(destinationDir, title string) (string, error) {
	path := filepath.Join(destinationDir, filepath.FromSlash(title))
	relative, err := filepath.Rel(destinationDir, path)
	if err != nil {
		return "", fmt.Errorf("calculating package layer path for %q: %w", title, err)
	}
	if title == "" || title == "." || !filepath.IsLocal(relative) {
		return "", fmt.Errorf("invalid package layer title %q", title)
	}
	return path, nil
}

// AssembleLayers returns the OCI layer descriptors for the requested package resources.
// Metadata layers are always included.
func AssembleLayers(ctx context.Context, root *oci.Manifest, fetcher content.Fetcher, selection LayerSelection) ([]ocispec.Descriptor, error) {
	layers := make([]ocispec.Descriptor, 0)
	for _, path := range PackageAlwaysPull {
		desc := root.Locate(path)
		if !oci.IsEmptyDescriptor(desc) {
			layers = append(layers, desc)
		}
	}

	pkg, err := FetchZarfYAML(ctx, root, fetcher)
	if err != nil {
		return nil, err
	}

	if selection.IncludeComponents || selection.IncludeImages {
		requestedComponents := selection.RequestedComponents
		if requestedComponents == nil {
			requestedComponents = pkg.Components
		}
		componentLayers, images, err := LayersFromComponents(root, pkg, requestedComponents)
		if err != nil {
			return nil, err
		}
		if selection.IncludeComponents {
			layers = append(layers, componentLayers...)
		}
		if selection.IncludeImages && len(images) > 0 {
			imageLayers, err := LayersFromImages(ctx, root, fetcher, images)
			if err != nil {
				return nil, err
			}
			layers = append(layers, imageLayers...)
		}
	}

	if selection.SBOMs != nil {
		if len(selection.SBOMs.Keys) > 0 && !layout.UsesGranularResourceLayout(pkg) {
			return nil, fmt.Errorf("SBOM selection by key requires package API version v1beta1+")
		}
		if layout.UsesGranularResourceLayout(pkg) {
			sbomLayers, err := checksumVerifiedSBOMLayers(ctx, root, fetcher, pkg, selection.SBOMs.Keys)
			if err != nil {
				return nil, err
			}
			layers = append(layers, sbomLayers...)
		} else if desc := root.Locate(layout.SBOMTar); !oci.IsEmptyDescriptor(desc) {
			layers = append(layers, desc)
		}
	}

	if selection.Attachments != nil {
		if layout.UsesGranularResourceLayout(pkg) {
			attachmentLayers, err := attachmentResourceLayers(root, pkg.Attachments, selection.Attachments.Keys)
			if err != nil {
				return nil, err
			}
			layers = append(layers, attachmentLayers...)
		} else if len(pkg.Attachments) > 0 {
			if desc := root.Locate(layout.DocumentationTar); !oci.IsEmptyDescriptor(desc) {
				layers = append(layers, desc)
			}
		}
	}
	return layers, nil
}

// attachmentResourceLayers validates every declared attachment resource before selecting the requested descriptors.
func attachmentResourceLayers(root *oci.Manifest, attachments map[string]string, keys []string) ([]ocispec.Descriptor, error) {
	allFileNames, err := layout.GetAttachmentFileNames(attachments, nil)
	if err != nil {
		return nil, err
	}
	selectedFileNames, err := layout.GetAttachmentFileNames(attachments, keys)
	if err != nil {
		return nil, err
	}

	declaredTitles := make(map[string]struct{}, len(allFileNames))
	for _, fileName := range allFileNames {
		declaredTitles[path.Join(layout.AttachmentsDir, fileName)] = struct{}{}
	}
	selectedTitles := make(map[string]struct{}, len(selectedFileNames))
	for _, fileName := range selectedFileNames {
		selectedTitles[path.Join(layout.AttachmentsDir, fileName)] = struct{}{}
	}

	descriptorsByTitle := make(map[string]ocispec.Descriptor, len(declaredTitles))
	for _, descriptor := range root.Layers {
		title := descriptor.Annotations[ocispec.AnnotationTitle]
		if _, declared := declaredTitles[title]; declared {
			if _, found := descriptorsByTitle[title]; found {
				return nil, fmt.Errorf("multiple OCI descriptors represent attachment resource %q", title)
			}
			descriptorsByTitle[title] = descriptor
			if descriptor.Annotations[layout.ResourceKindAnnotation] != layout.ResourceKindAttachment {
				return nil, fmt.Errorf("attachment resource %q has an invalid OCI kind", title)
			}
			if descriptor.MediaType != layout.AttachmentLayerMediaTypeBlob {
				return nil, fmt.Errorf("attachment resource %q has an invalid OCI media type %q", title, descriptor.MediaType)
			}
		} else if descriptor.Annotations[layout.ResourceKindAnnotation] == layout.ResourceKindAttachment {
			return nil, fmt.Errorf("attachment resource %q is not declared by the package", title)
		}
	}
	for title := range declaredTitles {
		if _, found := descriptorsByTitle[title]; !found {
			return nil, fmt.Errorf("attachment resource %q is missing from OCI manifest", title)
		}
	}

	layers := make([]ocispec.Descriptor, 0, len(selectedTitles))
	for _, descriptor := range root.Layers {
		if _, selected := selectedTitles[descriptor.Annotations[ocispec.AnnotationTitle]]; selected {
			layers = append(layers, descriptor)
		}
	}
	return layers, nil
}

// checksumVerifiedSBOMLayers validates every checksum-listed SBOM resource before selecting requested resources.
func checksumVerifiedSBOMLayers(ctx context.Context, root *oci.Manifest, fetcher content.Fetcher, pkg api.Package, keys []string) ([]ocispec.Descriptor, error) {
	checksumDesc := root.Locate(layout.Checksums)
	if oci.IsEmptyDescriptor(checksumDesc) {
		return nil, fmt.Errorf("checksums descriptor not found in OCI manifest")
	}
	checksums, err := content.FetchAll(ctx, fetcher, checksumDesc)
	if err != nil {
		return nil, fmt.Errorf("fetching checksum inventory: %w", err)
	}
	aggregateChecksum, err := checksum.GetSHA256Hash(bytes.NewReader(checksums))
	if err != nil {
		return nil, fmt.Errorf("hashing checksum inventory: %w", err)
	}
	if aggregateChecksum != pkg.Build.AggregateChecksum {
		return nil, fmt.Errorf("checksum inventory digest mismatch: expected %s, found %s", pkg.Build.AggregateChecksum, aggregateChecksum)
	}
	entries, err := checksum.Parse(checksums)
	if err != nil {
		return nil, err
	}

	descriptorsByPath := make(map[string]ocispec.Descriptor)
	for _, descriptor := range root.Layers {
		resourcePath := descriptor.Annotations[ocispec.AnnotationTitle]
		if !strings.HasPrefix(resourcePath, layout.SBOMResourcesDir+"/") {
			continue
		}
		if _, found := descriptorsByPath[resourcePath]; found {
			return nil, fmt.Errorf("multiple OCI descriptors represent SBOM resource %q", resourcePath)
		}
		descriptorsByPath[resourcePath] = descriptor
	}

	requested := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		requested[key] = struct{}{}
	}
	var declaredImageReferences map[string]struct{}
	if len(requested) > 0 {
		declaredImageReferences = layout.SBOMImageReferenceSet(pkg)
	}
	foundRequested := make(map[string]struct{}, len(requested))
	layers := []ocispec.Descriptor{}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Path, layout.SBOMResourcesDir+"/") {
			continue
		}
		descriptor, found := descriptorsByPath[entry.Path]
		if !found {
			return nil, fmt.Errorf("SBOM resource %q is missing from OCI manifest", entry.Path)
		}
		if descriptor.Annotations[ocispec.AnnotationTitle] != entry.Path {
			return nil, fmt.Errorf("SBOM resource %q has an invalid OCI title", entry.Path)
		}
		if descriptor.Annotations[layout.ResourceKindAnnotation] != layout.ResourceKindSBOM {
			return nil, fmt.Errorf("SBOM resource %q has an invalid OCI kind", entry.Path)
		}
		if descriptor.MediaType != layout.SBOMMediaTypeSyftJSON {
			return nil, fmt.Errorf("SBOM resource %q has an invalid OCI media type %q", entry.Path, descriptor.MediaType)
		}
		if descriptor.Digest.String() != string(api.ChecksumSHA256)+":"+entry.Digest {
			return nil, fmt.Errorf("SBOM resource %q has an OCI digest that does not match checksums.txt", entry.Path)
		}
		if len(requested) == 0 {
			layers = append(layers, descriptor)
			continue
		}
		kind, identifier, supported := layout.ParseSBOMResourcePath(entry.Path)
		if !supported {
			continue
		}
		selected := false
		for key := range requested {
			if !layout.SBOMKeyMatchesResource(kind, identifier, key, declaredImageReferences) {
				continue
			}
			foundRequested[key] = struct{}{}
			selected = true
		}
		if selected {
			layers = append(layers, descriptor)
		}
	}
	for _, key := range keys {
		if _, found := foundRequested[key]; !found {
			return nil, fmt.Errorf("SBOM key %q not found in package", key)
		}
	}
	return layers, nil
}

// LayersFromComponents returns the layers for the requested components and
// the set of images those components reference, selecting from root.
func LayersFromComponents(root *oci.Manifest, pkg api.Package, requestedComponents []api.Component) ([]ocispec.Descriptor, map[string]bool, error) {
	layers := make([]ocispec.Descriptor, 0)

	images := map[string]bool{}
	tarballFormat := "%s.tar"
	for _, rc := range requestedComponents {
		componentIndex := slices.IndexFunc(pkg.Components, func(component api.Component) bool {
			return component.Name == rc.Name
		})
		if componentIndex == -1 {
			return nil, nil, fmt.Errorf("component %s does not exist in this package", rc.Name)
		}
		component := pkg.Components[componentIndex]
		for _, image := range component.GetImages() {
			images[image] = true
		}
		desc := root.Locate(filepath.Join(layout.ComponentsDir, fmt.Sprintf(tarballFormat, component.Name)))
		layers = append(layers, desc)
	}
	return layers, images, nil
}

// LayersFromImages returns the image blob layers referenced by imageList, selecting
// from root. fetcher reads image manifests/indexes to walk multi-arch children.
func LayersFromImages(ctx context.Context, root *oci.Manifest, fetcher content.Fetcher, imageList map[string]bool) ([]ocispec.Descriptor, error) {
	index, err := oci.FetchJSONFile[*ocispec.Index](ctx, fetcher, root, layout.IndexPath)
	if err != nil {
		return nil, err
	}

	layers := make([]ocispec.Descriptor, 0)

	layers = append(layers, root.Locate(layout.IndexPath), root.Locate(layout.OCILayoutPath))

	for image := range imageList {
		// use docker's transform lib to parse the image ref
		// this properly mirrors the logic within create
		refInfo, err := transform.ParseImageRef(image)
		if err != nil {
			return nil, fmt.Errorf("failed to parse image ref %q: %w", image, err)
		}

		entryIndex := slices.IndexFunc(index.Manifests, func(layer ocispec.Descriptor) bool {
			return layer.Annotations[ocispec.AnnotationBaseImageName] == refInfo.Reference ||
				// A backwards compatibility shim for older Zarf versions that would leave docker.io off of image annotations
				(layer.Annotations[ocispec.AnnotationBaseImageName] == refInfo.Path+refInfo.TagOrDigest && refInfo.Host == "docker.io")
		})
		if entryIndex == -1 {
			return nil, fmt.Errorf("image %q not found in package index", refInfo.Reference)
		}
		entry := index.Manifests[entryIndex]

		layers = append(layers, root.Locate(filepath.Join(layout.ImagesBlobsDir, entry.Digest.Encoded())))

		switch {
		case images.IsIndex(entry.MediaType):
			childLayers, err := layersFromIndexChildren(ctx, root, fetcher, entry)
			if err != nil {
				return nil, err
			}
			layers = append(layers, childLayers...)
		case images.IsManifest(entry.MediaType):
			manifestLayers, err := layersFromManifestChildren(ctx, root, fetcher, entry)
			if err != nil {
				return nil, err
			}
			layers = append(layers, manifestLayers...)
		default:
			return nil, fmt.Errorf("unexpected media type %q for image %s", entry.MediaType, entry.Digest)
		}
	}
	// Remove duplicate descriptors in case of shared base layers
	return oci.RemoveDuplicateDescriptors(layers), nil
}

func layersFromManifestChildren(ctx context.Context, root *oci.Manifest, fetcher content.Fetcher, manifestDesc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
	manifest, err := oci.FetchJSONFile[*ocispec.Manifest](ctx, fetcher, root, filepath.Join(layout.ImagesBlobsDir, manifestDesc.Digest.Encoded()))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch manifest %s: %w", manifestDesc.Digest, err)
	}
	layers := make([]ocispec.Descriptor, 0, len(manifest.Layers)+1)
	if manifest.Config.Digest != "" {
		layers = append(layers, root.Locate(filepath.Join(layout.ImagesBlobsDir, manifest.Config.Digest.Encoded())))
	}
	for _, layer := range manifest.Layers {
		layers = append(layers, root.Locate(filepath.Join(layout.ImagesBlobsDir, layer.Digest.Encoded())))
	}
	return layers, nil
}

func layersFromIndexChildren(ctx context.Context, root *oci.Manifest, fetcher content.Fetcher, indexDesc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
	idx, err := oci.FetchJSONFile[*ocispec.Index](ctx, fetcher, root, filepath.Join(layout.ImagesBlobsDir, indexDesc.Digest.Encoded()))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch child index %s: %w", indexDesc.Digest, err)
	}
	layers := make([]ocispec.Descriptor, 0, len(idx.Manifests))
	for _, child := range idx.Manifests {
		layers = append(layers, root.Locate(filepath.Join(layout.ImagesBlobsDir, child.Digest.Encoded())))
		switch {
		case images.IsIndex(child.MediaType):
			nestedLayers, err := layersFromIndexChildren(ctx, root, fetcher, child)
			if err != nil {
				return nil, err
			}
			layers = append(layers, nestedLayers...)
		case images.IsManifest(child.MediaType):
			manifestLayers, err := layersFromManifestChildren(ctx, root, fetcher, child)
			if err != nil {
				return nil, err
			}
			layers = append(layers, manifestLayers...)
		default:
			return nil, fmt.Errorf("unexpected media type %q for index child %s", child.MediaType, child.Digest)
		}
	}
	return layers, nil
}

// AssembleLayers returns the OCI layer descriptors for the requested package resources.
func (r *Remote) AssembleLayers(ctx context.Context, selection LayerSelection) ([]ocispec.Descriptor, error) {
	root, err := r.FetchRoot(ctx)
	if err != nil {
		return nil, err
	}
	return AssembleLayers(ctx, root, r, selection)
}

// LayersFromComponents returns the layers for the requested components and
// the set of images those components reference, selecting from root.
func (r *Remote) LayersFromComponents(ctx context.Context, pkg api.Package, requestedComponents []api.Component) ([]ocispec.Descriptor, map[string]bool, error) {
	root, err := r.FetchRoot(ctx)
	if err != nil {
		return []ocispec.Descriptor{}, map[string]bool{}, err
	}
	return LayersFromComponents(root, pkg, requestedComponents)
}

// LayersFromImages returns the layers for the given images to pull from OCI.
func (r *Remote) LayersFromImages(ctx context.Context, imageList map[string]bool) ([]ocispec.Descriptor, error) {
	root, err := r.FetchRoot(ctx)
	if err != nil {
		return []ocispec.Descriptor{}, err
	}
	return LayersFromImages(ctx, root, r, imageList)
}
