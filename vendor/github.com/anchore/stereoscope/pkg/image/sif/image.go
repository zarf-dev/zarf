package sif

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/sylabs/sif/v2/pkg/sif"

	"github.com/anchore/stereoscope/pkg/image"
)

const SingularityMediaType = "application/vnd.sylabs.sif.layer.v1.sif"

// fileSectionReader implements an io.ReadCloser that reads from r and closes c.
type fileSectionReader struct {
	*io.SectionReader
	f *os.File
}

// newFileSectionReader returns a fileSectionReader that reads from the file at path starting at
// offset off and stops with EOF after n bytes.
func newFileSectionReader(path string, off int64, n int64) (*fileSectionReader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	rc := fileSectionReader{
		SectionReader: io.NewSectionReader(f, off, n),
		f:             f,
	}
	return &rc, nil
}

// Close closes rc, rendering it unusable for I/O.
func (rc *fileSectionReader) Close() error {
	return rc.f.Close()
}

// sifLayer implements the GGCR partial.UncompressedLayer interface for a given Descriptor.
type sifLayer struct {
	path string         // Path to SIF image.
	d    sif.Descriptor // SIF object descriptor of layer.
	h    v1.Hash        // Hash of layer.
}

// DiffID returns the Hash of the uncompressed layer.
func (l *sifLayer) DiffID() (v1.Hash, error) {
	return l.h, nil
}

// Uncompressed returns an io.ReadCloser for the uncompressed layer contents.
func (l *sifLayer) Uncompressed() (io.ReadCloser, error) {
	return newFileSectionReader(l.path, l.d.Offset(), l.d.Size())
}

// Returns the media type for the layer.
func (l *sifLayer) MediaType() (types.MediaType, error) {
	fs, _, _, err := l.d.PartitionMetadata()
	if err != nil {
		return "", err
	}

	switch fs {
	case sif.FsSquash:
		return image.SingularitySquashFSLayer, nil
	default:
		return "", fmt.Errorf("media type '%v' not supported", fs.String())
	}
}

// sifImage implements the GGCR partial.UncompressedImageCore interface for a SIF image.
type sifImage struct {
	path    string                     // Path to SIF image.
	arch    string                     // Architecture of primary system partition.
	diffIDs map[v1.Hash]sif.Descriptor // Map of layer diffIDs to descriptors.
	cfg     v1.ConfigFile              // Immitation config.
}

// newSIFImage returns a populated sifImage based on the SIF image found at path.
func newSIFImage(path string) (*sifImage, error) {
	f, err := sif.LoadContainerFromPath(path, sif.OptLoadWithFlag(os.O_RDONLY))
	if err != nil {
		return nil, fmt.Errorf("failed to load image: %w", err)
	}
	defer func() { _ = f.UnloadContainer() }()

	// The primary system partition is the root "layer".
	rootFS, err := f.GetDescriptor(sif.WithPartitionType(sif.PartPrimSys))
	if err != nil {
		return nil, fmt.Errorf("failed to get partition descriptor: %w", err)
	}

	_, _, arch, err := rootFS.PartitionMetadata()
	if err != nil {
		return nil, fmt.Errorf("failed to get partition metadata: %w", err)
	}

	// Calculate diffID of the root "layer".
	h, n, err := v1.SHA256(rootFS.GetReader())
	if err != nil {
		return nil, fmt.Errorf("failed to calculate hash: %w", err)
	} else if n != rootFS.Size() {
		return nil, errors.New("short read while calculating hash")
	}

	im := sifImage{
		path: path,
		arch: arch,
		diffIDs: map[v1.Hash]sif.Descriptor{
			h: rootFS,
		},
		cfg: v1.ConfigFile{
			Created: v1.Time{
				Time: f.CreatedAt(),
			},
			Architecture: arch,
			OS:           "linux",
			RootFS: v1.RootFS{
				Type:    "layers",
				DiffIDs: []v1.Hash{h},
			},
		},
	}
	return &im, nil
}

// RawConfigFile returns the serialized bytes of this image's config file.
func (im *sifImage) RawConfigFile() ([]byte, error) {
	return json.Marshal(im.cfg)
}

// MediaType of this image's manifest.
func (im *sifImage) MediaType() (types.MediaType, error) {
	return SingularityMediaType, nil
}

// LayerByDiffID is a variation on the v1.Image method, which returns an UncompressedLayer instead.
func (im *sifImage) LayerByDiffID(h v1.Hash) (partial.UncompressedLayer, error) {
	if d, ok := im.diffIDs[h]; ok {
		l := sifLayer{
			path: im.path,
			h:    h,
			d:    d,
		}
		return &l, nil
	}
	return nil, fmt.Errorf("layer %v not found", h)
}
