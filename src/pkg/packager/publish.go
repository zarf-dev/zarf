// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/mholt/archiver/v3"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry"
)

// ZarfLayerMediaType<Extension> is the media type for Zarf layers.
const (
	ZarfLayerMediaTypeTarZstd = "application/vnd.zarf.layer.v1.tar+zstd"
	ZarfLayerMediaTypeTarGzip = "application/vnd.zarf.layer.v1.tar+gzip"
	ZarfLayerMediaTypeYaml    = "application/vnd.zarf.layer.v1.yaml"
	ZarfLayerMediaTypeJSON    = "application/vnd.zarf.layer.v1.json"
	ZarfLayerMediaTypeTxt     = "application/vnd.zarf.layer.v1.txt"
	ZarfLayerMediaTypeUnknown = "application/vnd.zarf.layer.v1.unknown"
)

// parseZarfLayerMediaType returns the Zarf layer media type for the given filename.
func parseZarfLayerMediaType(filename string) string {
	// since we are controlling the filenames, we can just use the extension
	switch filepath.Ext(filename) {
	case ".zst":
		return ZarfLayerMediaTypeTarZstd
	case ".gz":
		return ZarfLayerMediaTypeTarGzip
	case ".yaml":
		return ZarfLayerMediaTypeYaml
	case ".json":
		return ZarfLayerMediaTypeJSON
	case ".txt":
		return ZarfLayerMediaTypeTxt
	default:
		return ZarfLayerMediaTypeUnknown
	}
}

// Publish publishes the package to a registry
//
// This is a wrapper around the oras library
// and much of the code was adapted from the oras CLI - https://github.com/oras-project/oras/blob/main/cmd/oras/push.go
//
// Authentication is handled via the Docker config file created w/ `zarf tools registry login`
func (p *Packager) Publish() error {
	p.cfg.DeployOpts.PackagePath = p.cfg.PublishOpts.PackagePath
	if err := p.loadZarfPkg(); err != nil {
		return fmt.Errorf("unable to load the package: %w", err)
	}

	paths := []string{
		p.tmp.ZarfYaml,
		filepath.Join(p.tmp.Base, "sboms.tar.zst"),
	}
	componentDirs, err := filepath.Glob(filepath.Join(p.tmp.Base, "components", "*"))
	if err != nil {
		return err
	}
	componentTarballs := []string{}
	// repackage the component directories into tarballs
	for _, componentDir := range componentDirs {
		dst := filepath.Join(p.tmp.Base, "components", filepath.Base(componentDir)+".tar.zst")
		err = archiver.Archive([]string{componentDir}, dst)
		if err != nil {
			return err
		}
		componentTarballs = append(componentTarballs, dst)
	}
	paths = append(paths, componentTarballs...)
	imagesLayers, err := filepath.Glob(filepath.Join(p.tmp.Base, "images", "*"))
	if err != nil {
		return err
	}
	paths = append(paths, imagesLayers...)
	ref, err := p.ref("")
	if err != nil {
		return err
	}
	message.HeaderInfof("📦 PACKAGE PUBLISH %s:%s", p.cfg.Pkg.Metadata.Name, ref.Reference)
	err = p.publish(ref, paths)
	if err != nil {
		return fmt.Errorf("unable to publish package %s: %w", ref, err)
	}
	return nil
}

func (p *Packager) generateManifestConfigFile() (ocispec.Descriptor, []byte, error) {
	// Unless specified, an empty manifest config will be used: `{}`
	// which causes an error on Google Artifact Registry
	// to negate this, we create a simple manifest config with some build metadata
	// the contents of this file are not used by Zarf
	type OCIConfigPartial struct {
		Architecture string            `json:"architecture"`
		OCIVersion   string            `json:"ociVersion"`
		Annotations  map[string]string `json:"annotations,omitempty"`
	}

	annotations := map[string]string{
		"org.opencontainers.image.title":       p.cfg.Pkg.Metadata.Name,
		"org.opencontainers.image.description": p.cfg.Pkg.Metadata.Description,
	}

	manifestConfig := OCIConfigPartial{
		Architecture: p.cfg.Pkg.Build.Architecture,
		OCIVersion:   "1.0.1",
		Annotations:  annotations,
	}
	manifestConfigBytes, err := json.Marshal(manifestConfig)
	if err != nil {
		return ocispec.Descriptor{}, nil, err
	}
	manifestConfigDesc := content.NewDescriptorFromBytes("application/vnd.unknown.config.v1+json", manifestConfigBytes)
	return manifestConfigDesc, manifestConfigBytes, nil
}

func (p *Packager) publish(ref registry.Reference, paths []string) error {
	message.Infof("Publishing package to %s", ref)
	spinner := message.NewProgressSpinner("")
	defer spinner.Stop()

	dst, err:= utils.NewOrasRemote(ref)
	if err != nil {
		return err
	}
	ctx := dst.Context

	store, err := file.New(p.tmp.Base)
	if err != nil {
		return err
	}
	defer store.Close()

	var descs []ocispec.Descriptor

	for idx, path := range paths {
		name, err := filepath.Rel(p.tmp.Base, path)
		if err != nil {
			return err
		}

		mediaType := parseZarfLayerMediaType(name)

		desc, err := store.Add(ctx, name, mediaType, path)
		spinner.Updatef("Preparing layer %d/%d: %s", idx+1, len(paths), name)
		if err != nil {
			return err
		}
		descs = append(descs, desc)
	}
	packOpts := p.cfg.PublishOpts.PackOptions
	pack := func(artifactType string) (ocispec.Descriptor, error) {
		root, err := oras.Pack(ctx, store, artifactType, descs, packOpts)
		if err != nil {
			return ocispec.Descriptor{}, err
		}
		if err = store.Tag(ctx, root, root.Digest.String()); err != nil {
			return ocispec.Descriptor{}, err
		}
		return root, nil
	}

	copyOpts := oras.DefaultCopyOptions
	copyOpts.Concurrency = p.cfg.PublishOpts.CopyOptions.Concurrency
	spinner.Stop()
	var total int64
	var totalLayersSize int64
	for _, desc := range descs {
		total += desc.Size
		totalLayersSize += desc.Size
	}
	copyOpts.OnCopySkipped = func(ctx context.Context, desc ocispec.Descriptor) error {
		title := desc.Annotations[ocispec.AnnotationTitle]
		var format string
		if title != "" {
			format = fmt.Sprintf("%s %s", desc.Digest.Encoded()[:12], utils.First30last30(title))
		} else {
			format = fmt.Sprintf("%s [%s]", desc.Digest.Encoded()[:12], desc.MediaType)
		}
		message.Successf(format)
		return nil
	}
	copyOpts.PostCopy = copyOpts.OnCopySkipped

	// first attempt to do a ArtifactManifest push
	root, err := pack(ocispec.MediaTypeArtifactManifest)
	if err != nil {
		return err
	}

	copyRootAttempted := false
	preCopy := copyOpts.PreCopy
	copyOpts.PreCopy = func(ctx context.Context, desc ocispec.Descriptor) error {
		if content.Equal(root, desc) {
			// copyRootAttempted helps track whether the returned error is
			// generated from copying root.
			copyRootAttempted = true
		}
		if preCopy != nil {
			return preCopy(ctx, desc)
		}
		return nil
	}

	total += root.Size
	dst.ProgressBar = message.NewProgressBar(total, "Publish layers")
	defer dst.ProgressBar.Stop()
	// attempt to push the artifact manifest
	_, err = oras.Copy(ctx, store, root.Digest.String(), dst, dst.Reference.Reference, copyOpts)

	if err == nil {
		dst.ProgressBar.Successf("Published %s [%s]", ref, root.MediaType)
		// message.Infof("Published: %s [%s]", ref, root.MediaType)
		message.Infof("Digest: %s", root.Digest)
		return nil
	}
	dst.ProgressBar.Stop()
	// log the error, the expected error is a 400 manifest invalid
	message.Debug("ArtifactManifest push failed with the following error, falling back to an ImageManifest push:", err)

	// if copyRootAttempted is false here, then there was an error generated before
	// the root was copied. This is unexpected, so return the error.
	if !copyRootAttempted {
		return fmt.Errorf("push failed before the manifest was pushed, returning the error: %w", err)
	}

	// if the error returned from the push is not an expected error, then return the error
	if !isManifestUnsupported(err) {
		return err
	}

	// assumes referrers API is not supported since OCI artifact
	// media type is not supported
	dst.SetReferrersCapability(false)

	// fallback to an ImageManifest push
	manifestConfigDesc, manifestConfigContent, err := p.generateManifestConfigFile()
	if err != nil {
		return err
	}
	// push the manifest config
	// since this config is so tiny, and the content is not used again
	// it is not logged to the multispinner, but will error if it fails
	err = dst.Push(ctx, manifestConfigDesc, bytes.NewReader(manifestConfigContent))
	if err != nil {
		return err
	}
	packOpts.ConfigDescriptor = &manifestConfigDesc
	packOpts.PackImageManifest = true
	root, err = pack(ocispec.MediaTypeImageManifest)
	if err != nil {
		return err
	}

	copyOpts.FindSuccessors = func(ctx context.Context, fetcher content.Fetcher, node ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		if content.Equal(node, root) {
			// skip non-config
			content, err := content.FetchAll(ctx, fetcher, root)
			if err != nil {
				return nil, err
			}
			var manifest ocispec.Manifest
			if err := json.Unmarshal(content, &manifest); err != nil {
				return nil, err
			}
			return []ocispec.Descriptor{manifest.Config}, nil
		}
		// config has no successors
		return nil, nil
	}

	total = totalLayersSize + root.Size + manifestConfigDesc.Size
	dst.ProgressBar = message.NewProgressBar(total, "Publish layers")
	defer dst.ProgressBar.Stop()
	// attempt to push the image manifest
	_, err = oras.Copy(ctx, store, root.Digest.String(), dst, dst.Reference.Reference, copyOpts)
	if err != nil {
		return err
	}
	dst.ProgressBar.Successf("Published %s [%s]", ref, root.MediaType)
	message.Infof("Digest: %s", root.Digest)
	return nil
}

// ref returns a registry.Reference using metadata from the package's build config and the PublishOpts
//
// if skeleton is not empty, the architecture will be replaced with the skeleton string (e.g. "skeleton")
func (p *Packager) ref(skeleton string) (registry.Reference, error) {
	ver := p.cfg.Pkg.Metadata.Version
	if len(ver) == 0 {
		ver = "0.0.1"
		// return registry.Reference{}, errors.New("version is required for publishing")
	}
	arch := p.cfg.Pkg.Build.Architecture
	// changes package ref from "name:version-arch" to "name:version-skeleton"
	if len(skeleton) > 0 {
		arch = skeleton
	}
	ref := registry.Reference{
		Registry:   p.cfg.PublishOpts.Reference.Registry,
		Repository: fmt.Sprintf("%s/%s", p.cfg.PublishOpts.Reference.Repository, p.cfg.Pkg.Metadata.Name),
		Reference:  fmt.Sprintf("%s-%s", ver, arch),
	}
	if len(p.cfg.PublishOpts.Reference.Repository) == 0 {
		ref.Repository = p.cfg.Pkg.Metadata.Name
	}
	err := ref.Validate()
	if err != nil {
		return registry.Reference{}, err
	}
	return ref, nil
}
