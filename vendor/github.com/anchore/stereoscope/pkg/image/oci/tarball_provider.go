package oci

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/anchore/stereoscope/internal/log"
	"github.com/anchore/stereoscope/pkg/file"
	"github.com/anchore/stereoscope/pkg/image"
)

const Archive image.Source = image.OciTarballSource

// NewArchiveProvider creates a new provider instance for the specific image tarball already at the given path.
func NewArchiveProvider(tmpDirGen *file.TempDirGenerator, path string) image.Provider {
	return NewArchiveProviderWithPlatform(tmpDirGen, path, nil)
}

// NewArchiveProviderWithPlatform creates a new provider instance for the specific image tarball already at the given path,
// with the given platform used to select an image from a multiplatform layout. The platform is also enforced for
// single-platform layouts: Provide returns an *image.ErrPlatformMismatch if the image does not match it. A nil
// platform selects the only image in a single-platform layout, or the host platform in a multiplatform one.
// The platform should come from image.NewPlatform so that OS and architecture aliases are normalized.
func NewArchiveProviderWithPlatform(tmpDirGen *file.TempDirGenerator, path string, platform *image.Platform) image.Provider {
	return &tarballImageProvider{
		tmpDirGen: tmpDirGen,
		path:      path,
		platform:  platform,
	}
}

// tarballImageProvider is an image.Provider for an OCI image (V1) for an existing tar on disk (from a buildah push <img> oci-archive:<name>.tar command).
type tarballImageProvider struct {
	tmpDirGen *file.TempDirGenerator
	path      string
	platform  *image.Platform
}

func (p *tarballImageProvider) Name() string {
	return Archive
}

// Provide an image object that represents the OCI image from a tarball.
func (p *tarballImageProvider) Provide(ctx context.Context) (*image.Image, error) {
	// note: we are untaring the image and using the existing directory provider, we could probably enhance the google
	// container registry lib to do this without needing to untar to a temp dir (https://github.com/google/go-containerregistry/issues/726)
	f, err := os.Open(p.path)
	if err != nil {
		return nil, fmt.Errorf("unable to open OCI tarball: %w", err)
	}
	defer f.Close()

	tempDir, err := p.tmpDirGen.NewDirectory("oci-tarball-image")
	if err != nil {
		return nil, err
	}

	log.WithFields("file", p.path, "tempDir", tempDir).Trace("extracting OCI tar file to tempdir")
	startTime := time.Now()

	if err = file.UntarToDirectory(f, tempDir); err != nil {
		return nil, err
	}

	log.WithFields("file", p.path, "tempDir", tempDir, "time", time.Since(startTime)).Debug("extracted OCI tar file to tempdir")

	return NewDirectoryProviderWithPlatform(p.tmpDirGen, tempDir, p.platform).Provide(ctx)
}
