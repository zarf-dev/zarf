// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/pkg/oci"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/file"
	ociDirectory "oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/registry"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/logger"
)

const (
	// ZarfConfigMediaType is the media type for the manifest config
	ZarfConfigMediaType = "application/vnd.zarf.config.v1+json"
	// ZarfLayerMediaTypeBlob is the media type for all Zarf layers due to the range of possible content
	ZarfLayerMediaTypeBlob = "application/vnd.zarf.layer.v1.blob"
	// SkeletonArch is the architecture used for skeleton packages
	SkeletonArch = "skeleton"
	// DefaultConcurrency is the default concurrency used for operations
	DefaultConcurrency = 3
	// ImageCacheDirectory is the directory within the Zarf cache containing an OCI store
	ImageCacheDirectory = "images"
)

// LayersSelector is a type for selecting subsets of layers in a Zarf package
type LayersSelector string

const (
	// AllLayers is the default selector for all layers
	AllLayers LayersSelector = ""
	//SbomLayers is the selector for SBOM layers including metadata
	SbomLayers LayersSelector = "sbom"
	// MetadataLayers is the selector for metadata layers (zarf.yaml, signature, checksums)
	MetadataLayers LayersSelector = "metadata"
	// ImageLayers is the selector for image layers including metadata
	ImageLayers LayersSelector = "images"
	// ComponentLayers is the selector for component layers including metadata
	ComponentLayers LayersSelector = "components"
)

// OCITimestampFormat is the format for OCI timestamp annotations
const OCITimestampFormat = time.RFC3339

// Remote is a wrapper around the Oras remote repository with zarf specific functions
type Remote struct {
	*oci.OrasRemote
}

// NewRemote returns an oras remote repository client and context for the given url
// with zarf opination embedded
func NewRemote(ctx context.Context, url string, platform ocispec.Platform, mods ...oci.Modifier) (*Remote, error) {
	l := logger.From(ctx)
	if config.CommonOptions.CachePath != "" {
		absCachePath, err := config.GetAbsCachePath()
		if err != nil {
			return nil, err
		}
		ociCache, err := ociDirectory.NewWithContext(ctx, filepath.Join(absCachePath, ImageCacheDirectory))
		if err != nil {
			return nil, err
		}
		mods = append(mods, oci.WithCache(ociCache))
	}

	modifiers := append([]oci.Modifier{
		oci.WithPlainHTTP(config.CommonOptions.PlainHTTP),
		oci.WithInsecureSkipVerify(config.CommonOptions.InsecureSkipTLSVerify),
		oci.WithLogger(l),
		oci.WithUserAgent("zarf/" + config.CLIVersion),
	}, mods...)
	remote, err := oci.NewOrasRemote(url, platform, modifiers...)
	if err != nil {
		return nil, err
	}
	return &Remote{remote}, nil
}

// Push pushes the given package layout to the remote registry.
func (r *Remote) Push(ctx context.Context, pkgLayout *PackageLayout, concurrency int) (err error) {
	logger.From(ctx).Info("pushing package to registry",
		"destination", r.Repo().Reference.String(),
		"architecture", pkgLayout.Pkg.Build.Architecture)

	src, err := file.New("")
	if err != nil {
		return err
	}
	defer func(src *file.Store) {
		err2 := src.Close()
		err = errors.Join(err, err2)
	}(src)

	descs := []ocispec.Descriptor{}
	files, err := pkgLayout.Files()
	if err != nil {
		return err
	}
	for path, name := range files {
		desc, err := src.Add(ctx, name, ZarfLayerMediaTypeBlob, path)
		if err != nil {
			return err
		}
		descs = append(descs, desc)
	}

	// Sort by Digest string
	sort.Slice(descs, func(i, j int) bool {
		return descs[i].Digest < descs[j].Digest
	})

	annotations := annotationsFromMetadata(pkgLayout.Pkg.Metadata)

	// Perform the conversion of the string timestamp to the appropriate format in order to maintain backwards compatibility

	t, err := time.Parse(CreateTimestampFormat, pkgLayout.Pkg.Build.Timestamp)
	if err != nil {
		// if we change the format of the timestamp, we need to update the conversion here
		// and also account for an error state for mismatch with older formats
		return fmt.Errorf("unable to parse timestamp: %w", err)
	}
	annotations[ocispec.AnnotationCreated] = t.Format(OCITimestampFormat)

	manifestConfigDesc, err := r.CreateAndPushManifestConfig(ctx, annotations, ZarfConfigMediaType)
	if err != nil {
		return err
	}
	// here is where the manifest is created and written to the filesystem given the file.store Push() functionality
	root, err := r.PackAndTagManifest(ctx, src, descs, manifestConfigDesc, annotations)
	if err != nil {
		return err
	}

	defer func() {
		// remove the dangling manifest file created by the PackAndTagManifest
		// should this behavior change, we should expect this to begin producing an error
		err2 := os.Remove(pkgLayout.Pkg.Metadata.Name)
		err = errors.Join(err, err2)
	}()

	copyOpts := r.GetDefaultCopyOpts()
	copyOpts.Concurrency = concurrency
	publishedDesc, err := oras.Copy(ctx, src, root.Digest.String(), r.Repo(), "", copyOpts)
	if err != nil {
		return err
	}

	err = r.UpdateIndex(ctx, r.Repo().Reference.Reference, publishedDesc)
	if err != nil {
		return err
	}

	return nil
}

// ReferenceFromMetadata creates an OCI reference using the package metadata
func ReferenceFromMetadata(registryLocation string, pkg v1alpha1.ZarfPackage) (string, error) {
	if len(pkg.Metadata.Version) == 0 {
		return "", errors.New("version is required for publishing")
	}
	if !strings.HasSuffix(registryLocation, "/") {
		registryLocation = registryLocation + "/"
	}
	registryLocation = strings.TrimPrefix(registryLocation, helpers.OCIURLPrefix)

	raw := fmt.Sprintf("%s%s:%s", registryLocation, pkg.Metadata.Name, pkg.Metadata.Version)
	if pkg.Build.Flavor != "" {
		raw = fmt.Sprintf("%s-%s", raw, pkg.Build.Flavor)
	}

	ref, err := registry.ParseReference(raw)
	if err != nil {
		return "", fmt.Errorf("failed to parse %s: %w", raw, err)
	}
	return ref.String(), nil
}

func annotationsFromMetadata(metadata v1alpha1.ZarfMetadata) map[string]string {
	annotations := map[string]string{
		ocispec.AnnotationTitle:       metadata.Name,
		ocispec.AnnotationDescription: metadata.Description,
	}
	if url := metadata.URL; url != "" {
		annotations[ocispec.AnnotationURL] = url
	}
	if authors := metadata.Authors; authors != "" {
		annotations[ocispec.AnnotationAuthors] = authors
	}
	if documentation := metadata.Documentation; documentation != "" {
		annotations[ocispec.AnnotationDocumentation] = documentation
	}
	if source := metadata.Source; source != "" {
		annotations[ocispec.AnnotationSource] = source
	}
	if vendor := metadata.Vendor; vendor != "" {
		annotations[ocispec.AnnotationVendor] = vendor
	}
	// annotations explicitly defined in `metadata.annotations` take precedence over legacy fields
	maps.Copy(annotations, metadata.Annotations)
	return annotations
}

// PlatformForSkeleton sets the target architecture for the remote to skeleton
func PlatformForSkeleton() ocispec.Platform {
	return ocispec.Platform{
		OS:           oci.MultiOS,
		Architecture: SkeletonArch,
	}
}

// CopyPackage copies a zarf package from one OCI registry to another
func CopyPackage(ctx context.Context, src *Remote, dst *Remote, concurrency int) (err error) {
	l := logger.From(ctx)
	if concurrency <= 0 {
		concurrency = DefaultConcurrency
	}

	srcManifest, err := src.FetchRoot(ctx)
	if err != nil {
		return err
	}
	l.Info("copying package",
		"src", src.Repo().Reference.String(),
		"dst", dst.Repo().Reference.String())
	if err := oci.Copy(ctx, src.OrasRemote, dst.OrasRemote, nil, concurrency, nil); err != nil {
		return err
	}

	srcRoot, err := src.ResolveRoot(ctx)
	if err != nil {
		return err
	}

	b, err := srcManifest.MarshalJSON()
	if err != nil {
		return err
	}
	expected := content.NewDescriptorFromBytes(ocispec.MediaTypeImageManifest, b)

	if err := dst.Repo().Manifests().PushReference(ctx, expected, bytes.NewReader(b), srcRoot.Digest.String()); err != nil {
		return err
	}

	tag := src.Repo().Reference.Reference
	if err := dst.UpdateIndex(ctx, tag, expected); err != nil {
		return err
	}

	src.Log().Info(fmt.Sprintf("Published %s to %s", src.Repo().Reference, dst.Repo().Reference))
	return nil
}
