package sif

import (
	"context"
	"errors"

	"github.com/google/go-containerregistry/pkg/v1/partial"

	"github.com/anchore/stereoscope/pkg/file"
	"github.com/anchore/stereoscope/pkg/image"
)

const ProviderName = image.SingularitySource

// NewArchiveProvider creates a new provider instance for the Singularity Image Format (SIF) image
// at path.
func NewArchiveProvider(tmpDirGen *file.TempDirGenerator, path string) image.Provider {
	return &singularityImageProvider{
		tmpDirGen: tmpDirGen,
		path:      path,
	}
}

// singularityImageProvider is an image.Provider for a Singularity Image Format (SIF) image.
type singularityImageProvider struct {
	tmpDirGen *file.TempDirGenerator
	path      string
}

func (p *singularityImageProvider) Name() string {
	return ProviderName
}

// Provide returns an Image that represents a Singularity Image Format (SIF) image.
func (p *singularityImageProvider) Provide(_ context.Context) (*image.Image, error) {
	// We need to map the SIF to a GGCR v1.Image. Start with an implementation of the GGCR
	// partial.UncompressedImageCore interface.
	si, err := newSIFImage(p.path)
	if err != nil {
		return nil, err
	}

	// Promote our partial.UncompressedImageCore implementation to an v1.Image.
	ui, err := partial.UncompressedToImage(si)
	if err != nil {
		return nil, err
	}

	// The returned image must reference a content cache dir.
	contentCacheDir, err := p.tmpDirGen.NewDirectory()
	if err != nil {
		return nil, err
	}

	// Apply user-supplied metadata last to override any default behavior.
	metadata := []image.AdditionalMetadata{
		image.WithOS("linux"),
		image.WithArchitecture(si.arch, ""),
	}

	out := image.New(ui, p.tmpDirGen, contentCacheDir, metadata...)
	err = out.Read()
	if err != nil {
		cleanErr := out.Cleanup()
		return nil, errors.Join(err, cleanErr)
	}
	return out, err
}
