package docker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"

	"github.com/anchore/stereoscope/internal/log"
	"github.com/anchore/stereoscope/pkg/file"
)

type dockerManifest struct {
	parsed tarball.Manifest
}

// newManifest creates a new manifest struct from the given Docker archive manifest bytes
func newManifest(raw []byte) (*dockerManifest, error) {
	var parsed tarball.Manifest
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("unable to parse manifest.json: %w", err)
	}

	if len(parsed) == 0 {
		return nil, fmt.Errorf("no valid manifest.json found")
	}

	return &dockerManifest{
		parsed: parsed,
	}, nil
}

// allTags returns the image tags referenced within the images manifest file (within the given docker image tar).
func (m dockerManifest) allTags() (tags []string) {
	for _, entry := range m.parsed {
		tags = append(tags, entry.RepoTags...)
	}
	return tags
}

// extractManifest is helper function for extracting and parsing a docker image manifest (V2) from a docker image tar.
func extractManifest(tarPath string) (*dockerManifest, error) {
	f, err := os.Open(tarPath)
	if err != nil {
		return nil, err
	}

	defer func() {
		err := f.Close()
		if err != nil {
			log.Errorf("unable to close tar file (%s): %w", f.Name(), err)
		}
	}()

	//nolint:closecheck // ReaderFromTar's Close just forwards to f, which is closed by the deferred close above
	manifestReader, err := file.ReaderFromTar(f, "manifest.json")
	if err != nil {
		return nil, err
	}

	contents, err := io.ReadAll(manifestReader)
	if err != nil {
		return nil, fmt.Errorf("unable to read manifest.json: %w", err)
	}
	return newManifest(contents)
}

// generateOCIManifest takes a docker manifest and a path to the tar and generates an OCI manifest derived from the given arguments and the docker config.
func generateOCIManifest(tarPath string, manifest *dockerManifest) (*v1.Manifest, []byte, error) {
	f, err := os.Open(tarPath)
	if err != nil {
		return nil, nil, err
	}

	defer func() {
		err := f.Close()
		if err != nil {
			log.Errorf("unable to close tar file (%s): %w", f.Name(), err)
		}
	}()

	if len(manifest.parsed) != 1 {
		return nil, nil, ErrMultipleManifests
	}

	//nolint:closecheck // ReaderFromTar's Close just forwards to f, which is closed by the deferred close above
	configReader, err := file.ReaderFromTar(f, manifest.parsed[0].Config)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to find docker config: %w", err)
	}

	configContents, err := io.ReadAll(configReader)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to read docker config: %w", err)
	}

	var layerSizes = make([]int64, len(manifest.parsed[0].Layers))
	for idx, layerTarPath := range manifest.parsed[0].Layers {
		_, err = f.Seek(0, io.SeekStart)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to reset tar reader: %w", err)
		}
		layerMetadata, err := file.MetadataFromTar(f, layerTarPath)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to find layer tar: %w", err)
		}
		layerSizes[idx] = layerMetadata.Size()
	}

	theManifest, err := assembleOCIManifest(configContents, layerSizes)

	return theManifest, configContents, err
}

// assembleOCIManifest takes the docker manifest and config file content to populate a v1.Manifest (OCI).
func assembleOCIManifest(configBytes []byte, layerSizes []int64) (*v1.Manifest, error) {
	cfgHash, cfgSize, err := v1.SHA256(bytes.NewReader(configBytes))
	if err != nil {
		return nil, err
	}

	cfg, err := v1.ParseConfigFile(bytes.NewReader(configBytes))
	if err != nil {
		return nil, fmt.Errorf("unable to parse docker config: %w", err)
	}

	ociManifest := v1.Manifest{
		SchemaVersion: 2,
		MediaType:     types.DockerManifestSchema2,
		Config: v1.Descriptor{
			MediaType: types.DockerConfigJSON,
			Size:      cfgSize,
			Digest:    cfgHash,
		},
	}

	for idx, diffID := range cfg.RootFS.DiffIDs {
		ociManifest.Layers = append(ociManifest.Layers, v1.Descriptor{
			MediaType: types.DockerLayer,
			Size:      layerSizes[idx],
			Digest:    diffID,
		})
	}

	return &ociManifest, nil
}
