package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"

	"github.com/anchore/stereoscope/internal/log"
	"github.com/anchore/stereoscope/pkg/file"
	"github.com/anchore/stereoscope/pkg/image"
)

const Archive image.Source = image.DockerTarballSource

// NewArchiveProvider creates a new provider able to resolve docker tarball archives
func NewArchiveProvider(tmpDirGen *file.TempDirGenerator, path string, additionalMetadata ...image.AdditionalMetadata) image.Provider {
	return &tarballImageProvider{
		tmpDirGen:          tmpDirGen,
		path:               path,
		additionalMetadata: additionalMetadata,
	}
}

var ErrMultipleManifests = fmt.Errorf("cannot process multiple docker manifests")

// tarballImageProvider is a image.Provider for a docker image (V2) for an existing tar on disk (the output from a "docker image save ..." command).
type tarballImageProvider struct {
	tmpDirGen          *file.TempDirGenerator
	path               string
	additionalMetadata []image.AdditionalMetadata
}

func (p *tarballImageProvider) Name() string {
	return Archive
}

// Provide an image object that represents the docker image tar at the configured location on disk.
func (p *tarballImageProvider) Provide(_ context.Context) (*image.Image, error) {
	startTime := time.Now()

	img, err := tarball.ImageFromPath(p.path, nil)
	if err != nil {
		// raise a more controlled error for when there are multiple images within the given tar (from https://github.com/anchore/grype/issues/215)
		if err.Error() == "tarball must contain only a single image to be used with tarball.Image" {
			return nil, ErrMultipleManifests
		}
		return nil, fmt.Errorf("unable to provide image from tarball: %w", err)
	}

	log.WithFields("image", p.path, "time", time.Since(startTime)).Debug("got uncompressed image tarball")

	// make a best-effort to generate an OCI manifest and gets tags, but ultimately this should be considered optional
	var rawOCIManifest []byte
	var rawConfig []byte
	var ociManifest *v1.Manifest
	var metadata []image.AdditionalMetadata

	theManifest, err := extractManifest(p.path)
	if err != nil {
		log.Warnf("could not extract manifest: %+v", err)
	}

	if theManifest != nil {
		// given that we have a manifest, continue processing to get the tags and OCI manifest
		metadata = append(metadata, image.WithTags(theManifest.allTags()...))

		ociManifest, rawConfig, err = generateOCIManifest(p.path, theManifest)
		if err != nil {
			log.Warnf("failed to generate OCI manifest from docker archive: %+v", err)
		}

		// we may have the config available, use it
		if rawConfig != nil {
			metadata = append(metadata, image.WithConfig(rawConfig))
		}
	}

	if ociManifest != nil {
		rawOCIManifest, err = json.Marshal(&ociManifest)
		if err != nil {
			log.Warnf("failed to serialize OCI manifest: %+v", err)
		} else {
			metadata = append(metadata, image.WithManifest(rawOCIManifest))
		}
	}

	// apply user-supplied metadata last to override any default behavior
	metadata = append(metadata, p.additionalMetadata...)

	contentTempDir, err := p.tmpDirGen.NewDirectory("docker-tarball-image")
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
