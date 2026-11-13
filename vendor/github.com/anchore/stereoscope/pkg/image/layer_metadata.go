package image

import (
	v1 "github.com/google/go-containerregistry/pkg/v1"
	v1Types "github.com/google/go-containerregistry/pkg/v1/types"
)

// LayerMetadata represents container layer metadata.
type LayerMetadata struct {
	Index uint
	// Digest is the layer's diff ID: the sha256 of the uncompressed layer contents. Taken from the
	// image config's rootfs.diff_ids when it lists exactly one per layer (as ggcr itself does for
	// registry and docker images), otherwise computed from the layer. It is what the image claims
	// about its contents, not a verification of them.
	Digest    string
	MediaType v1Types.MediaType
	// Size in bytes of the layer content size
	Size int64
}

// newLayerMetadata aggregates pertinent layer metadata information. knownDiffID, when non-empty,
// is the layer's diff ID as recorded in the image config and is used instead of asking the layer:
// for an OCI layout on disk ggcr has no cheap answer and decompresses the entire layer just to
// hash it, only for the layer to be decompressed again to unpack it.
func newLayerMetadata(layer v1.Layer, idx int, knownDiffID string) (LayerMetadata, error) {
	mediaType, err := layer.MediaType()
	if err != nil {
		return LayerMetadata{}, err
	}
	digest := knownDiffID
	if digest == "" {
		diffID, err := layer.DiffID()
		if err != nil {
			return LayerMetadata{}, err
		}
		digest = diffID.String()
	}

	return LayerMetadata{
		Index:     uint(idx),
		Digest:    digest,
		MediaType: mediaType,
	}, nil
}
