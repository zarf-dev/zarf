// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"strings"
	"time"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/pkg/oci"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
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
