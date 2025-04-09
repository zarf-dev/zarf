// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/pkg/oci"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
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
)

// Remote is a wrapper around the Oras remote repository with zarf specific functions
type Remote struct {
	orasRemote *oci.OrasRemote
}

// NewRemote returns an oras remote repository client and context for the given url with zarf opination embedded.
func NewRemote(ctx context.Context, url string, platform ocispec.Platform, mods ...oci.Modifier) (*Remote, error) {
	l := logger.From(ctx)
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
	return &Remote{orasRemote: remote}, nil
}

// Push pushes the given package layout to the remote registry.
func (r *Remote) Push(ctx context.Context, pkgLayout *PackageLayout, concurrency int) (err error) {
	logger.From(ctx).Info("pushing package to registry",
		"destination", r.orasRemote.Repo().Reference.String(),
		"architecture", pkgLayout.Pkg.Metadata.Architecture)

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

	annotations := annotationsFromMetadata(pkgLayout.Pkg.Metadata)
	manifestConfigDesc, err := r.orasRemote.CreateAndPushManifestConfig(ctx, annotations, ZarfConfigMediaType)
	if err != nil {
		return err
	}
	root, err := r.orasRemote.PackAndTagManifest(ctx, src, descs, manifestConfigDesc, annotations)
	if err != nil {
		return err
	}

	copyOpts := r.orasRemote.GetDefaultCopyOpts()
	copyOpts.Concurrency = concurrency
	publishedDesc, err := oras.Copy(ctx, src, root.Digest.String(), r.orasRemote.Repo(), "", copyOpts)
	if err != nil {
		return err
	}

	err = r.orasRemote.UpdateIndex(ctx, r.orasRemote.Repo().Reference.Reference, publishedDesc)
	if err != nil {
		return err
	}

	return nil
}

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
