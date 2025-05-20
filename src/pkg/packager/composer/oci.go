// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package composer contains functions for composing components within Zarf packages.
package composer

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/pkg/oci"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/archive"
	"github.com/zarf-dev/zarf/src/pkg/layout"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
	ocistore "oras.land/oras-go/v2/content/oci"
)

func (ic *ImportChain) getRemote(ctx context.Context, url string) (*zoci.Remote, error) {
	if ic.remote != nil {
		return ic.remote, nil
	}
	var err error
	ic.remote, err = zoci.NewRemote(ctx, url, zoci.PlatformForSkeleton())
	if err != nil {
		return nil, err
	}
	_, err = ic.remote.ResolveRoot(ctx)
	if err != nil {
		return nil, fmt.Errorf("published skeleton package for %q does not exist: %w", url, err)
	}
	return ic.remote, nil
}

// ContainsOCIImport returns true if the import chain contains a remote import
func (ic *ImportChain) ContainsOCIImport() bool {
	// only the 2nd to last node may have a remote import
	return ic.tail.prev != nil && ic.tail.prev.Import.URL != ""
}

func (ic *ImportChain) fetchOCISkeleton(ctx context.Context) error {
	l := logger.From(ctx)
	if !ic.ContainsOCIImport() {
		return nil
	}
	node := ic.tail.prev
	remote, err := ic.getRemote(ctx, node.Import.URL)
	if err != nil {
		return err
	}

	manifest, err := remote.FetchRoot(ctx)
	if err != nil {
		return err
	}

	name := node.ImportName()

	componentDesc := manifest.Locate(filepath.Join(layout.ComponentsDir, fmt.Sprintf("%s.tar", name)))

	absCachePath, err := config.GetAbsCachePath()
	if err != nil {
		return err
	}
	cache := filepath.Join(absCachePath, "oci")
	if err := helpers.CreateDirectory(cache, helpers.ReadWriteExecuteUser); err != nil {
		return err
	}

	var tb, dir string

	// if there is not a tarball to fetch, create a directory named based upon
	// the import url and the component name
	if oci.IsEmptyDescriptor(componentDesc) {
		h := sha256.New()
		h.Write([]byte(node.Import.URL + name))
		id := fmt.Sprintf("%x", h.Sum(nil))

		dir = filepath.Join(cache, "dirs", id)

		l.Debug("creating empty directory for remote component", "component", filepath.Join("<zarf-cache>", "oci", "dirs", id))
	} else {
		tb = filepath.Join(cache, "blobs", "sha256", componentDesc.Digest.Encoded())
		dir = filepath.Join(cache, "dirs", componentDesc.Digest.Encoded())

		store, err := ocistore.New(cache)
		if err != nil {
			return err
		}

		// ensure the tarball is in the cache
		exists, err := store.Exists(ctx, componentDesc)
		if err != nil {
			return err
		} else if !exists {
			err = remote.CopyToTarget(ctx, []ocispec.Descriptor{componentDesc}, store, remote.GetDefaultCopyOpts())
			if err != nil {
				return err
			}
		}
	}

	if err := helpers.CreateDirectory(dir, helpers.ReadWriteExecuteUser); err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(cwd, dir)
	if err != nil {
		return err
	}
	// the tail node is the only node whose relativeToHead is based solely upon cwd<->cache
	// contrary to the other nodes, which are based upon the previous node
	ic.tail.relativeToHead = rel

	if oci.IsEmptyDescriptor(componentDesc) {
		// nothing was fetched, nothing to extract
		return nil
	}

	decompressOpts := archive.DecompressOpts{
		OverwriteExisting: true,
		StripComponents:   1,
	}
	return archive.Decompress(ctx, tb, dir, decompressOpts)
}
