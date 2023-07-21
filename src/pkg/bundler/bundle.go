// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bundler contains functions for interacting with, managing and deploying Zarf bundles.
package bundler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/types"
	goyaml "github.com/goccy/go-yaml"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
)

// Bundle publishes the given bundle w/ optional signature to the remote repository.
func Bundle(r *oci.OrasRemote, bundle *types.ZarfBundle, signature []byte) error {
	if bundle.Metadata.Architecture == "" {
		return fmt.Errorf("architecture is required for bundling")
	}
	ref := r.Repo().Reference
	message.Debug("Bundling", bundle.Metadata.Name, "to", ref)

	manifest := ocispec.Manifest{}

	for _, pkg := range bundle.Packages {
		// TODO: handle components + wildcards
		url := fmt.Sprintf("%s:%s", pkg.Repository, pkg.Ref)
		remote, err := oci.NewOrasRemote(url)
		if err != nil {
			return err
		}
		pkgRef := remote.Repo().Reference
		// fetch the root manifest so we can push it into the bundle
		root, err := remote.FetchRoot()
		if err != nil {
			return err
		}
		manifestBytes, err := json.Marshal(root)
		if err != nil {
			return err
		}
		// push the manifest into the bundle
		manifestDesc, err := r.PushLayer(manifestBytes, oci.ZarfLayerMediaTypeBlob)
		if err != nil {
			return err
		}
		// hack the media type to be a manifest
		manifestDesc.MediaType = ocispec.MediaTypeImageManifest
		message.Debugf("Pushed %s sub-manifest into %s: %s", url, ref, message.JSONValue(manifestDesc))
		manifest.Layers = append(manifest.Layers, manifestDesc)
		// stream copy the blobs from remote to o, otherwise do a blob mount
		if remote.Repo().Reference.Registry != ref.Registry {
			message.Debugf("Streaming layers from %s --> %s", pkgRef, ref)
			if err := oci.CopyPackage(remote, r, config.CommonOptions.OCIConcurrency); err != nil {
				return err
			}
		} else {
			message.Debugf("Performing a cross repository blob mount on %s from %s --> %s", ref, ref.Repository, ref.Repository)
			spinner := message.NewProgressSpinner("Mounting layers from %s", pkgRef.Repository)
			includingConfig := append(root.Layers, root.Config)
			for _, layer := range includingConfig {
				spinner.Updatef("Mounting %s", layer.Digest.Encoded())
				if err := r.Repo().Mount(context.TODO(), layer, pkgRef.Repository, func() (io.ReadCloser, error) {
					return remote.Repo().Fetch(context.TODO(), layer)
				}); err != nil {
					return err
				}
			}
			spinner.Successf("Mounted %d layers", len(includingConfig))
		}
	}

	// push the bundle's metadata
	zarfBundleYamlBytes, err := goyaml.Marshal(bundle)
	if err != nil {
		return err
	}
	zarfBundleYamlDesc, err := r.PushLayer(zarfBundleYamlBytes, oci.ZarfLayerMediaTypeBlob)
	if err != nil {
		return err
	}
	zarfBundleYamlDesc.Annotations = map[string]string{
		ocispec.AnnotationTitle: BundleYAML,
	}

	message.Debug("Pushed", BundleYAML+":", message.JSONValue(zarfBundleYamlDesc))
	manifest.Layers = append(manifest.Layers, zarfBundleYamlDesc)

	// push the bundle's signature
	if len(signature) > 0 {
		zarfBundleYamlSigDesc, err := r.PushLayer(signature, oci.ZarfLayerMediaTypeBlob)
		if err != nil {
			return err
		}
		zarfBundleYamlSigDesc.Annotations = map[string]string{
			ocispec.AnnotationTitle: BundleYAMLSignature,
		}
		manifest.Layers = append(manifest.Layers, zarfBundleYamlSigDesc)
		message.Debug("Pushed", BundleYAMLSignature+":", message.JSONValue(zarfBundleYamlSigDesc))
	}

	// push the manifest config
	configDesc, err := r.PushManifestConfigFromMetadata(&bundle.Metadata, &bundle.Build)
	if err != nil {
		return err
	}

	message.Debug("Pushed config:", message.JSONValue(configDesc))

	manifest.Config = configDesc

	manifest.SchemaVersion = 2

	manifest.Annotations = r.ManifestAnnotationsFromMetadata(&bundle.Metadata)
	b, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	expected := content.NewDescriptorFromBytes(ocispec.MediaTypeImageManifest, b)

	message.Debug("Pushing manifest:", message.JSONValue(expected))

	if err := r.Repo().Manifests().PushReference(context.TODO(), expected, bytes.NewReader(b), ref.Reference); err != nil {
		return fmt.Errorf("failed to push manifest: %w", err)
	}

	message.Successf("Published %s [%s]", ref, expected.MediaType)

	message.HorizontalRule()
	flags := ""
	if config.CommonOptions.Insecure {
		flags = "--insecure"
	}
	message.Title("To inspect/deploy/pull:", "")
	message.Command("bundle inspect oci://%s %s", ref, flags)
	message.Command("bundle deploy oci://%s %s", ref, flags)
	message.Command("bundle pull oci://%s %s", ref, flags)

	return nil
}
