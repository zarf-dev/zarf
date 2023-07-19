// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bundler contains functions for interacting with, managing and deploying Zarf bundles.
package bundler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/mholt/archiver/v4"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// Pull pulls a bundle and saves it locally + caches it
//
// : retrieve the `zarf-bundle.yaml`, and `zarf-bundle.yaml.sig`
// : verify sigs / checksums
// : pull the bundle into cache and tarball it up
func (b *Bundler) Pull() error {
	if err := b.SetOCIRemote(b.cfg.PullOpts.Source); err != nil {
		return err
	}

	// fetch the bundle's root descriptor
	// to later get the bundle's descriptor
	rootDesc, err := b.remote.ResolveRoot()
	if err != nil {
		return err
	}
	root, err := b.remote.FetchManifest(rootDesc)
	if err != nil {
		return err
	}

	cacheDir := filepath.Join(config.GetAbsCachePath(), "packages")
	// create the cache directory if it doesn't exist
	if err := utils.CreateDirectory(cacheDir, 0755); err != nil {
		return err
	}

	// TODO: figure out the best path to check the signature before we pull the bundle

	// pull the bundle
	layersPulled, err := b.remote.PullBundle(cacheDir, config.CommonOptions.OCIConcurrency, nil)
	if err != nil {
		return err
	}

	// locate the zarf-bundle.yaml's descriptor
	bundleDesc := root.Locate(config.ZarfBundleYAML)
	if err != nil {
		return err
	}

	// make an index.json specifically for this bundle
	index := ocispec.Index{}
	index.SchemaVersion = 2
	index.Manifests = append(index.Manifests, rootDesc)

	// write the index.json to tmp
	bytes, err := json.Marshal(index)
	if err != nil {
		return err
	}
	indexJSONPath := filepath.Join(b.tmp, "index.json")
	if err := utils.WriteFile(indexJSONPath, bytes); err != nil {
		return err
	}

	// read the zarf-bundle.yaml into memory
	bundleYamlPath := filepath.Join(cacheDir, "blobs", "sha256", bundleDesc.Digest.Encoded())
	if err := b.ReadBundleYaml(bundleYamlPath, &b.bundle); err != nil {
		return err
	}

	// tarball the bundle
	filename := fmt.Sprintf("%s%s-%s-%s.tar.zst", config.ZarfBundlePrefix, b.bundle.Metadata.Name, b.bundle.Metadata.Architecture, b.bundle.Metadata.Version)
	dst := filepath.Join(b.cfg.PullOpts.OutputDirectory, filename)

	// TODO: instead of removing then writing
	// 	 we should figure out a way to stream the
	//   differential into the tarball

	_ = os.RemoveAll(dst)

	// get the paths of the layers and the root descriptor
	paths := []string{}
	for _, layer := range layersPulled {
		paths = append(paths, filepath.Join(cacheDir, "blobs", "sha256", layer.Digest.Encoded()))
	}
	paths = append(paths, filepath.Join(cacheDir, "blobs", "sha256", rootDesc.Digest.Encoded()))
	paths = helpers.Unique(paths)

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	// TODO: support an --uncompressed flag?

	format := archiver.CompressedArchive{
		Compression: archiver.Zstd{},
		Archival:    archiver.Tar{},
	}

	pathMap := make(map[string]string)

	// put the index.json and oci-layout at the root of the tarball
	pathMap[indexJSONPath] = "index.json"
	pathMap[filepath.Join(cacheDir, "oci-layout")] = "oci-layout"

	// re-map the paths to be relative to the cache directory
	for _, path := range paths {
		pathMap[path] = strings.TrimPrefix(path, cacheDir+string(os.PathSeparator))
	}

	files, err := archiver.FilesFromDisk(nil, pathMap)
	if err != nil {
		return err
	}

	// tarball the bundle
	if err := format.Archive(context.TODO(), out, files); err != nil {
		return err
	}

	message.Debug("Bundle tarball saved to", dst)

	return nil
}
