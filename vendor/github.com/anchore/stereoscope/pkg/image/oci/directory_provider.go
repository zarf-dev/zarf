package oci

import (
	"context"
	"errors"
	"fmt"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"

	"github.com/anchore/stereoscope/pkg/file"
	"github.com/anchore/stereoscope/pkg/image"
)

const Directory image.Source = image.OciDirectorySource

// NewDirectoryProvider creates a new provider instance for the specific image already at the given path.
func NewDirectoryProvider(tmpDirGen *file.TempDirGenerator, path string) image.Provider {
	return &directoryImageProvider{
		tmpDirGen: tmpDirGen,
		path:      path,
	}
}

// directoryImageProvider is an image.Provider for an OCI image (V1) for an existing tar on disk (from a buildah push <img> oci:<img> command).
type directoryImageProvider struct {
	tmpDirGen *file.TempDirGenerator
	path      string
}

func (p *directoryImageProvider) Name() string {
	return Directory
}

// Provide an image object that represents the OCI image as a directory.
func (p *directoryImageProvider) Provide(_ context.Context) (*image.Image, error) {
	pathObj, err := layout.FromPath(p.path)
	if err != nil {
		return nil, fmt.Errorf("unable to read image from OCI directory path %q: %w", p.path, err)
	}

	index, err := layout.ImageIndexFromPath(p.path)
	if err != nil {
		return nil, fmt.Errorf("unable to parse OCI directory index: %w", err)
	}

	indexManifest, err := index.IndexManifest()
	if err != nil {
		return nil, fmt.Errorf("unable to parse OCI directory indexManifest: %w", err)
	}

	// for now, lets only support one image indexManifest (it is not clear how to handle multiple manifests)
	if len(indexManifest.Manifests) != 1 {
		if len(indexManifest.Manifests) == 0 {
			return nil, fmt.Errorf("unexpected number of OCI directory manifests (found %d)", len(indexManifest.Manifests))
		}
		// if all the manifests have the same digest, then we can treat this as a single image
		if !checkManifestDigestsEqual(indexManifest.Manifests) {
			return nil, fmt.Errorf("unexpected number of OCI directory manifests (found %d)", len(indexManifest.Manifests))
		}
	}

	manifest := indexManifest.Manifests[0]
	img, err := pathObj.Image(manifest.Digest)
	if err != nil {
		return nil, fmt.Errorf("unable to parse OCI directory as an image: %w", err)
	}

	metadata := []image.AdditionalMetadata{
		image.WithManifestDigest(manifest.Digest.String()),
	}

	// make a best-effort attempt at getting the raw indexManifest
	rawManifest, err := img.RawManifest()
	if err == nil {
		metadata = append(metadata, image.WithManifest(rawManifest))
	}

	contentTempDir, err := p.tmpDirGen.NewDirectory("oci-dir-image")
	if err != nil {
		return nil, err
	}

	out := image.New(img, p.tmpDirGen, contentTempDir, metadata...)
	err = out.Read()
	if err != nil {
		cleanErr := out.Cleanup()
		return nil, errors.Join(err, cleanErr)
	}
	return out, err
}

func checkManifestDigestsEqual(manifests []v1.Descriptor) bool {
	if len(manifests) < 1 {
		return false
	}
	for _, m := range manifests {
		if m.Digest != manifests[0].Digest {
			return false
		}
	}
	return true
}
