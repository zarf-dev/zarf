// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package oci contains functions for interacting with Zarf packages stored in OCI registries.
package oci

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
)

// Bundle pushes the given bundle to the remote repository.
func (o *OrasRemote) Bundle(bundle *types.ZarfBundle, sigPath string, sigPsswd string) error {
	layers := []ocispec.Descriptor{}
	for _, pkg := range bundle.Packages {
		url := fmt.Sprintf(pkg.Repository, pkg.Ref)
		remote, err := NewOrasRemote(url)
		if err != nil {
			return err
		}
		root, err := remote.FetchRoot()
		if err != nil {
			return err
		}
		if remote.Reference.Registry != o.Reference.Registry {
			message.Infof("Copying layers from %s to %s", remote.Reference, o.Reference)
			// stream copy the blobs from remote to o (if needed), otherwise do a blob mount
		} else {
			message.Infof("Blob mounting layers from %s to %s", remote.Reference, o.Reference)
			// mount the blobs from remote to o (if needed)
		}
		layers = append(layers, root.Layers...)
	}
	manifest := ocispec.Manifest{}
	manifest.Layers = layers
	// TODO: push + append the zarf-bundle.yaml to the layers, w/ proper path
	// TODO: push + append the zarf-bundle.yaml.sig to the layers, w/ proper path
	// TODO: strip the zarf.sig.yaml from each package + remove from checksums.txt + modify the zarf.yaml?
	message.Debug("TODO: signing bundle w/ %s - %s", sigPath, sigPsswd)
	manifest.Annotations = o.manifestAnnotationsFromMetadata(&bundle.Metadata)
	b, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	expected := content.NewDescriptorFromBytes(ocispec.MediaTypeImageManifest, b)

	_, err = o.pushManifestConfigFromMetadata(&bundle.Metadata, &bundle.Build)
	if err != nil {
		return err
	}

	return o.Manifests().PushReference(o.Context, expected, bytes.NewReader(b), o.Reference.Reference)
}
