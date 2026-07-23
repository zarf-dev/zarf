package image

import (
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	v1Types "github.com/google/go-containerregistry/pkg/v1/types"
)

// Metadata represents container image metadata.
type Metadata struct {
	// ID is the sha256 of this image config json (not manifest)
	ID string
	// Size in bytes of all the image layer content sizes (does not include config / manifest / index metadata sizes)
	Size      int64
	Config    v1.ConfigFile
	MediaType v1Types.MediaType
	// --- below fields are optional metadata
	Tags           []name.Tag
	RawManifest    []byte
	ManifestDigest string
	RawConfig      []byte
	RepoDigests    []string
	Architecture   string
	Variant        string
	OS             string
}

// readImageMetadata extracts the most pertinent information from the underlying image tar.
func readImageMetadata(img v1.Image) (Metadata, error) {
	id, err := img.ConfigName()
	if err != nil {
		return Metadata{}, err
	}

	config, err := img.ConfigFile()
	if err != nil {
		return Metadata{}, err
	}

	mediaType, err := img.MediaType()
	if err != nil {
		return Metadata{}, err
	}

	rawConfig, err := img.RawConfigFile()
	if err != nil {
		return Metadata{}, err
	}

	return Metadata{
		ID:        id.String(),
		Config:    *config,
		MediaType: mediaType,
		RawConfig: rawConfig,
	}, nil
}
