package image

import (
	v1 "github.com/google/go-containerregistry/pkg/v1"
	v1Types "github.com/google/go-containerregistry/pkg/v1/types"
)

// LayerMetadata represents container layer metadata.
type LayerMetadata struct {
	Index uint
	// Digest is the sha256 digest of the layer contents (the docker "diff id")
	Digest    string
	MediaType v1Types.MediaType
	// Size in bytes of the layer content size
	Size int64
}

// newLayerMetadata aggregates pertinent layer metadata information.
func newLayerMetadata(layer v1.Layer, idx int) (LayerMetadata, error) {
	mediaType, err := layer.MediaType()
	if err != nil {
		return LayerMetadata{}, err
	}
	diffID, err := layer.DiffID()
	if err != nil {
		return LayerMetadata{}, err
	}

	return LayerMetadata{
		Index:     uint(idx),
		Digest:    diffID.String(),
		MediaType: mediaType,
	}, nil
}
