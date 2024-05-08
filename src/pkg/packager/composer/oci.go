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

	"github.com/defenseunicorns/pkg/helpers"
	"github.com/defenseunicorns/pkg/oci"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/zoci"
	"github.com/mholt/archiver/v3"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	ocistore "oras.land/oras-go/v2/content/oci"
)

func (ic *ImportChain) getRemote(url string) (*zoci.Remote, error) {
	if ic.remote != nil {
		return ic.remote, nil
	}
	var err error
	ic.remote, err = zoci.NewRemote(url, zoci.PlatformForSkeleton())
	if err != nil {
		return nil, err
	}
	_, err = ic.remote.ResolveRoot(context.TODO())
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

func (ic *ImportChain) fetchOCISkeleton() error {
	if !ic.ContainsOCIImport() {
		return nil
	}
	node := ic.tail.prev
	remote, err := ic.getRemote(node.Import.URL)
	if err != nil {
		return err
	}

	ctx := context.TODO()
	manifest, err := remote.FetchRoot(ctx)
	if err != nil {
		return err
	}

	name := node.ImportName()

	componentDesc := manifest.Locate(filepath.Join(layout.ComponentsDir, fmt.Sprintf("%s.tar", name)))

	cache := filepath.Join(config.GetAbsCachePath(), "oci")
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

		message.Debug("creating empty directory for remote component:", filepath.Join("<zarf-cache>", "oci", "dirs", id))
	} else {
		tb = filepath.Join(cache, "blobs", "sha256", componentDesc.Digest.Encoded())
		dir = filepath.Join(cache, "dirs", componentDesc.Digest.Encoded())

		store, err := ocistore.New(cache)
		if err != nil {
			return err
		}

		ctx := context.TODO()
		// ensure the tarball is in the cache
		exists, err := store.Exists(ctx, componentDesc)
		if err != nil {
			return err
		} else if !exists {
			doneSaving := make(chan error)
			successText := fmt.Sprintf("Pulling %q", helpers.OCIURLPrefix+remote.Repo().Reference.String())
			go utils.RenderProgressBarForLocalDirWrite(cache, componentDesc.Size, doneSaving, "Pulling", successText)
			err = remote.CopyToTarget(ctx, []ocispec.Descriptor{componentDesc}, store, remote.GetDefaultCopyOpts())
			doneSaving <- err
			<-doneSaving
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

	tu := archiver.Tar{
		OverwriteExisting: true,
		// removes /<component-name>/ from the paths
		StripComponents: 1,
	}
	return tu.Unarchive(tb, dir)
}
