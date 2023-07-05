// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package oci contains functions for interacting with Zarf packages stored in OCI registries.
package oci

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
	goyaml "github.com/goccy/go-yaml"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
)

// Bundle pushes the given bundle to the remote repository.
func (o *OrasRemote) Bundle(bundle *types.ZarfBundle, sigPath string, sigPsswd string) error {
	message.Debug("Bundling", bundle.Metadata.Name, "to", o.Reference)
	layers := []ocispec.Descriptor{}
	index := ocispec.Index{}

	for _, pkg := range bundle.Packages {
		url := fmt.Sprintf("%s:%s-%s", pkg.Repository, pkg.Ref, bundle.Metadata.Architecture)
		remote, err := NewOrasRemote(url)
		if err != nil {
			return err
		}
		root, err := remote.FetchRoot()
		if err != nil {
			return err
		}
		manifestBytes, err := json.Marshal(root)
		if err != nil {
			return err
		}
		// push the manifest into the bundle
		manifestDesc, err := o.PushBytes(manifestBytes, ZarfLayerMediaTypeBlob)
		if err != nil {
			return err
		}
		// add the package name to the manifest's annotations to make it easier to find
		manifestDesc.Annotations = map[string]string{
			ocispec.AnnotationBaseImageName: url,
		}
		message.Debugf("Pushed %s sub-manifest into %s: %s", url, o.Reference, message.JSONValue(manifestDesc))
		index.Manifests = append(index.Manifests, manifestDesc)
		// stream copy the blobs from remote to o, otherwise do a blob mount
		if remote.Reference.Registry != o.Reference.Registry {
			message.Debugf("Streaming layers from %s --> %s", remote.Reference, o.Reference)
			if err := CopyPackage(remote, o, config.CommonOptions.OCIConcurrency); err != nil {
				return err
			}
		} else {
			message.Debugf("Performing a cross repository blob mount on %s from %s --> %s", remote.Reference.Registry, remote.Reference.Repository, o.Reference.Repository)
			spinner := message.NewProgressSpinner("Mounting layers from %s", remote.Reference.Repository)
			includingConfig := append(root.Layers, root.Config)
			for _, layer := range includingConfig {
				spinner.Updatef("Mounting %s", layer.Digest.Encoded())
				if err := o.Mount(o.Context, layer, remote.Reference.Repository, func() (io.ReadCloser, error) {
					// TODO: how does this handle auth?
					return remote.Fetch(o.Context, layer)
				}); err != nil {
					return err
				}
			}
			spinner.Successf("Mounted %d layers", len(includingConfig))
		}
		layers = append(layers, root.Layers...)
		layers = append(layers, manifestDesc)
		layers = append(layers, root.Config)
	}
	// push the index.json
	indexBytes, err := json.Marshal(index)
	if err != nil {
		return err
	}
	indexDesc, err := o.PushBytes(indexBytes, ZarfLayerMediaTypeBlob)
	if err != nil {
		return err
	}
	message.Debug("Pushed index.json:", message.JSONValue(indexDesc))
	// when doing an oras.Copy, the annotation title is used as the filepath to download to
	indexDesc.Annotations = map[string]string{
		ocispec.AnnotationTitle: "index.json",
	}

	manifest := ocispec.Manifest{}
	manifest.Layers = layers
	for idx, layer := range manifest.Layers {
		// when doing an oras.Copy, the annotation title is used as the filepath to download to
		// this makes it so it downloads all of the package layers to blobs/sha256/<layer-digest>
		manifest.Layers[idx].Annotations = map[string]string{
			ocispec.AnnotationTitle: filepath.Join("blobs", "sha256", layer.Digest.Encoded()),
		}
	}
	manifest.Layers = append(manifest.Layers, indexDesc)

	// push the zarf-bundle.yaml
	zarfBundleYamlBytes, err := goyaml.Marshal(bundle)
	if err != nil {
		return err
	}
	zarfBundleYamlDesc, err := o.PushBytes(zarfBundleYamlBytes, ZarfLayerMediaTypeBlob)
	if err != nil {
		return err
	}
	zarfBundleYamlDesc.Annotations = map[string]string{
		ocispec.AnnotationTitle: "zarf-bundle.yaml",
	}

	message.Debug("Pushed zarf-bundle.yaml:", message.JSONValue(zarfBundleYamlDesc))
	manifest.Layers = append(manifest.Layers, zarfBundleYamlDesc)

	// TODO: push + append the zarf-bundle.yaml.sig to the layers, w/ proper path
	message.Debug("TODO: signing bundle w/ %s - %s", sigPath, sigPsswd)

	// push the manifest config
	configDesc, err := o.pushManifestConfigFromMetadata(&bundle.Metadata, &bundle.Build)
	if err != nil {
		return err
	}

	message.Debug("Pushed config:", message.JSONValue(configDesc))

	manifest.Config = configDesc

	manifest.SchemaVersion = 2

	manifest.Annotations = o.manifestAnnotationsFromMetadata(&bundle.Metadata)
	b, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	expected := content.NewDescriptorFromBytes(ocispec.MediaTypeImageManifest, b)

	message.Debug("Pushing manifest:", message.JSONValue(expected))

	o.Transport.ProgressBar = message.NewProgressBar(expected.Size, "Pushing manifest")
	defer o.Transport.ProgressBar.Stop()

	if err := o.Manifests().PushReference(o.Context, expected, bytes.NewReader(b), o.Reference.Reference); err != nil {
		return fmt.Errorf("failed to push manifest: %w", err)
	}

	o.Transport.ProgressBar.Successf("Published %s [%s]", o.Reference, expected.MediaType)
	o.Transport.ProgressBar = nil

	message.HorizontalRule()
	flags := ""
	if config.CommonOptions.Insecure {
		flags = "--insecure"
	}
	message.Title("To inspect/deploy/pull:", "")
	message.ZarfCommand("bundle inspect oci://%s %s", o.Reference, flags)
	message.ZarfCommand("bundle deploy oci://%s %s", o.Reference, flags)
	message.ZarfCommand("bundle pull oci://%s %s", o.Reference, flags)

	return nil
}
