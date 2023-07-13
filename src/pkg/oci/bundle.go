// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package oci contains functions for interacting with Zarf packages stored in OCI registries.
package oci

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
	goyaml "github.com/goccy/go-yaml"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
)

// Bundle publishes the given bundle w/ optional signature to the remote repository.
func (o *OrasRemote) Bundle(bundle *types.ZarfBundle, signature []byte) error {
	if bundle.Metadata.Architecture == "" {
		return fmt.Errorf("architecture is required for bundling")
	}
	ref := o.repo.Reference
	message.Debug("Bundling", bundle.Metadata.Name, "to", ref)

	manifest := ocispec.Manifest{}

	for _, pkg := range bundle.Packages {
		// TODO: handle components + wildcards
		url := fmt.Sprintf("%s:%s", pkg.Repository, pkg.Ref)
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
		manifestDesc, err := o.PushLayer(manifestBytes, ZarfLayerMediaTypeBlob)
		if err != nil {
			return err
		}
		// hack the media type to be a manifest
		manifestDesc.MediaType = ocispec.MediaTypeImageManifest
		message.Debugf("Pushed %s sub-manifest into %s: %s", url, ref, message.JSONValue(manifestDesc))
		manifest.Layers = append(manifest.Layers, manifestDesc)
		// stream copy the blobs from remote to o, otherwise do a blob mount
		if remote.repo.Reference.Registry != o.repo.Reference.Registry {
			message.Debugf("Streaming layers from %s --> %s", remote.repo.Reference, o.repo.Reference)
			if err := CopyPackage(remote, o, config.CommonOptions.OCIConcurrency); err != nil {
				return err
			}
		} else {
			message.Debugf("Performing a cross repository blob mount on %s from %s --> %s", remote.repo.Reference.Registry, remote.repo.Reference.Repository, ref.Repository)
			spinner := message.NewProgressSpinner("Mounting layers from %s", remote.repo.Reference.Repository)
			includingConfig := append(root.Layers, root.Config)
			for _, layer := range includingConfig {
				spinner.Updatef("Mounting %s", layer.Digest.Encoded())
				if err := o.repo.Mount(o.ctx, layer, remote.repo.Reference.Repository, func() (io.ReadCloser, error) {
					return remote.repo.Fetch(o.ctx, layer)
				}); err != nil {
					return err
				}
			}
			spinner.Successf("Mounted %d layers", len(includingConfig))
		}
	}

	// push the zarf-bundle.yaml
	zarfBundleYamlBytes, err := goyaml.Marshal(bundle)
	if err != nil {
		return err
	}
	zarfBundleYamlDesc, err := o.PushLayer(zarfBundleYamlBytes, ZarfLayerMediaTypeBlob)
	if err != nil {
		return err
	}
	zarfBundleYamlDesc.Annotations = map[string]string{
		ocispec.AnnotationTitle: config.ZarfBundleYAML,
	}

	message.Debug("Pushed", config.ZarfBundleYAML+":", message.JSONValue(zarfBundleYamlDesc))
	manifest.Layers = append(manifest.Layers, zarfBundleYamlDesc)

	if len(signature) > 0 {
		zarfBundleYamlSigDesc, err := o.PushLayer(signature, ZarfLayerMediaTypeBlob)
		if err != nil {
			return err
		}
		zarfBundleYamlSigDesc.Annotations = map[string]string{
			ocispec.AnnotationTitle: config.ZarfBundleYAMLSignature,
		}
		manifest.Layers = append(manifest.Layers, zarfBundleYamlSigDesc)
		message.Debug("Pushed", config.ZarfBundleYAMLSignature+":", message.JSONValue(zarfBundleYamlSigDesc))
	}

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

	if err := o.repo.Manifests().PushReference(o.ctx, expected, bytes.NewReader(b), ref.Reference); err != nil {
		return fmt.Errorf("failed to push manifest: %w", err)
	}

	message.Successf("Published %s [%s]", ref, expected.MediaType)

	message.HorizontalRule()
	flags := ""
	if config.CommonOptions.Insecure {
		flags = "--insecure"
	}
	message.Title("To inspect/deploy/pull:", "")
	message.ZarfCommand("bundle inspect oci://%s %s", ref, flags)
	message.ZarfCommand("bundle deploy oci://%s %s", ref, flags)
	message.ZarfCommand("bundle pull oci://%s %s", ref, flags)

	return nil
}
