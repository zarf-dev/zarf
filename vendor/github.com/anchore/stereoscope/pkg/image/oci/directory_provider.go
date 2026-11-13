package oci

import (
	"context"
	"errors"
	"fmt"
	"runtime"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"

	"github.com/anchore/stereoscope/internal/log"
	"github.com/anchore/stereoscope/pkg/file"
	"github.com/anchore/stereoscope/pkg/image"
)

const Directory image.Source = image.OciDirectorySource

// NewDirectoryProvider creates a new provider instance for the specific image already at the given path.
func NewDirectoryProvider(tmpDirGen *file.TempDirGenerator, path string) image.Provider {
	return NewDirectoryProviderWithPlatform(tmpDirGen, path, nil)
}

// NewDirectoryProviderWithPlatform creates a new provider instance for the specific image already at the given path,
// with the given platform used to select an image from a multiplatform layout. The platform is also enforced for
// single-platform layouts: Provide returns an *image.ErrPlatformMismatch if the image does not match it. A nil
// platform selects the only image in a single-platform layout, or the host platform in a multiplatform one.
// The platform should come from image.NewPlatform so that OS and architecture aliases are normalized.
func NewDirectoryProviderWithPlatform(tmpDirGen *file.TempDirGenerator, path string, platform *image.Platform) image.Provider {
	return &directoryImageProvider{
		tmpDirGen: tmpDirGen,
		path:      path,
		platform:  platform,
	}
}

// directoryImageProvider is an image.Provider for an OCI image (V1) for an existing tar on disk (from a buildah push <img> oci:<img> command).
type directoryImageProvider struct {
	tmpDirGen *file.TempDirGenerator
	path      string
	platform  *image.Platform
}

func (p *directoryImageProvider) Name() string {
	return Directory
}

// Provide an image object that represents the OCI image as a directory.
func (p *directoryImageProvider) Provide(ctx context.Context) (*image.Image, error) {
	if _, err := layout.FromPath(p.path); err != nil {
		return nil, fmt.Errorf("unable to read image from OCI directory path %q: %w", p.path, err)
	}

	index, err := layout.ImageIndexFromPath(p.path)
	if err != nil {
		return nil, fmt.Errorf("unable to parse OCI directory index: %w", err)
	}

	if _, err := index.IndexManifest(); err != nil {
		return nil, fmt.Errorf("unable to parse OCI directory indexManifest: %w", err)
	}

	allImages, err := findAllImages(index)
	if err != nil {
		return nil, fmt.Errorf("unable to find all images in OCI directory: %w", err)
	}

	log.Debugf("found %d total images in OCI directory", len(allImages))

	if len(allImages) == 0 {
		return nil, fmt.Errorf("no images found in OCI directory at path %q", p.path)
	}

	var selectedImage v1.Image
	if len(allImages) == 1 {
		// a single-image layout has no ambiguity to resolve, so don't narrow by platform here: the index
		// descriptor platform is optional and is frequently absent (e.g. `skopeo copy ... oci:<dir>`).
		// the image config is checked below, which is the authoritative source.
		for _, img := range allImages {
			selectedImage = img.image
		}
	} else {
		platform := defaultPlatformIfNil(p.platform)
		ggcrPlatform := toContainerRegistryPlatform(platform)
		if ggcrPlatform == nil {
			return nil, fmt.Errorf("unable to determine a platform to select (host architecture %q is not supported)", runtime.GOARCH)
		}
		matchedImages := imagesForPlatform(allImages, *ggcrPlatform)
		if len(matchedImages) != 1 {
			return nil, fmt.Errorf("unexpected number of images matching platform %q in OCI directory (expected 1, found %d)", ggcrPlatform, len(matchedImages))
		}
		selectedImage = matchedImages[0]
	}

	// the index descriptor is advisory; confirm the selection against the image config and report what the
	// image actually is (Image.Read populates OS/architecture/variant from the same config).
	selectedConfig, err := selectedImage.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("unable to get config for selected image: %w", err)
	}
	if err := validatePlatform(p.platform, selectedConfig.OS, selectedConfig.Architecture, selectedConfig.Variant); err != nil {
		return nil, err
	}

	selectedImageDigest, err := selectedImage.Digest()
	if err != nil {
		return nil, fmt.Errorf("unable to get digest for selected image: %w", err)
	}

	log.Debugf("selecting image with digest %s from OCI layout", selectedImageDigest.String())

	metadata := []image.AdditionalMetadata{
		image.WithManifestDigest(selectedImageDigest.String()),
	}

	// make a best-effort attempt at getting the raw indexManifest
	rawManifest, err := selectedImage.RawManifest()
	if err == nil {
		metadata = append(metadata, image.WithManifest(rawManifest))
	}

	contentTempDir, err := p.tmpDirGen.NewDirectory("oci-dir-image")
	if err != nil {
		return nil, err
	}

	out := image.New(selectedImage, p.tmpDirGen, contentTempDir, metadata...)
	err = out.Read(ctx)
	if err != nil {
		cleanErr := out.Cleanup()
		return nil, errors.Join(err, cleanErr)
	}
	return out, err
}

type imageReference struct {
	image     v1.Image
	platforms []v1.Platform
}

func imagesForPlatform(images map[v1.Hash]imageReference, desiredPlatform v1.Platform) []v1.Image {
	var matches []v1.Image
	for _, img := range images {
		for _, platform := range img.platforms {
			if matchesPlatform(platform, desiredPlatform) {
				// an image may be referenced by several index entries (e.g. multiple tags), so only
				// count it once
				matches = append(matches, img.image)
				break
			}
		}
	}
	return matches
}

func findAllImages(index v1.ImageIndex) (map[v1.Hash]imageReference, error) {
	images := make(map[v1.Hash]imageReference)
	err := walkImages(index, func(i v1.Image, p *v1.Platform) error {
		digest, err := i.Digest()
		if err != nil {
			return err
		}
		var platforms []v1.Platform
		// Collect all platforms for this image.
		// There may be multiple index entries that reference the same image with different platform information.
		if existingImage, found := images[digest]; found {
			platforms = existingImage.platforms
		}
		if p != nil {
			platforms = append(platforms, *p)
		}
		images[digest] = imageReference{
			image:     i,
			platforms: platforms,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return images, nil
}

// isAttestationManifest indicates the descriptor references build attestation data rather than a runnable
// image. Buildx marks these with an "unknown/unknown" platform and a reference-type annotation.
func isAttestationManifest(desc v1.Descriptor) bool {
	if desc.Annotations["vnd.docker.reference.type"] == "attestation-manifest" {
		return true
	}
	return desc.Platform != nil && desc.Platform.OS == "unknown" && desc.Platform.Architecture == "unknown"
}

func platformToString(p *v1.Platform) string {
	if p == nil {
		return "<nil>"
	}
	return p.String()
}

func walkImages(index v1.ImageIndex, fn func(v1.Image, *v1.Platform) error) error {
	indexManifest, err := index.IndexManifest()
	if err != nil {
		return err
	}
	for i, manifest := range indexManifest.Manifests {
		log.WithFields(
			"index", i,
			"mediaType", manifest.MediaType,
			"digest", manifest.Digest.String(),
			"platform", platformToString(manifest.Platform),
		).Debug("walking images in OCI directory")
		switch {
		case manifest.MediaType.IsIndex():
			imgIndex, err := index.ImageIndex(manifest.Digest)
			if err != nil {
				return fmt.Errorf("unable to parse reference %s from OCI directory as an image index: %w", manifest.Digest, err)
			}
			err = walkImages(imgIndex, fn)
			if err != nil {
				return err
			}
		case manifest.MediaType.IsImage():
			if isAttestationManifest(manifest) {
				// buildx writes provenance/SBOM attestations as image manifests; they are never a valid
				// selection target and would otherwise be counted as images
				continue
			}
			img, err := index.Image(manifest.Digest)
			if err != nil {
				return fmt.Errorf("unable to parse reference %s from OCI directory as an image: %w", manifest.Digest, err)
			}
			err = fn(img, manifest.Platform)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
