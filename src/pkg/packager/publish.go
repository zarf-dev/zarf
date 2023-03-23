// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/mholt/archiver/v3"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry"
)

// ZarfLayerMediaTypeBlob is the media type for all Zarf layers due to the range of possible content
const (
	ZarfLayerMediaTypeBlob = "application/vnd.zarf.layer.v1.blob"
)

// Publish publishes the package to a registry
//
// This is a wrapper around the oras library
// and much of the code was adapted from the oras CLI - https://github.com/oras-project/oras/blob/main/cmd/oras/push.go
//
// Authentication is handled via the Docker config file created w/ `zarf tools registry login`
func (p *Packager) Publish() error {
	p.cfg.DeployOpts.PackagePath = p.cfg.PublishOpts.PackagePath
	if utils.IsDir(p.cfg.PublishOpts.PackagePath) {
		base, err := filepath.Abs(p.cfg.PublishOpts.PackagePath)
		if err != nil {
			return err
		}
		if err := os.Chdir(base); err != nil {
			return err
		}
		paths := []string{
			"zarf.yaml",
		}
		err = utils.ReadYaml("zarf.yaml", &p.cfg.Pkg)
		if err != nil {
			return err
		}
		ref, err := p.ref("skeleton")
		if err != nil {
			return err
		}
		for _, component := range p.cfg.Pkg.Components {
			if len(component.Import.Path) > 0 {
				message.Warnf("Component '%s' is a locally imported component and will not be included in the skeleton package", component.Name)
			}
			if len(component.Files) > 0 {
				for _, file := range component.Files {
					paths = append(paths, file.Source)
				}
			}
			if len(component.Charts) > 0 {
				for _, chart := range component.Charts {
					if len(chart.LocalPath) > 0 {
						localChartPaths, err := utils.RecursiveFileList(chart.LocalPath, regexp.MustCompile(".*"))
						if err != nil {
							return fmt.Errorf("unable to get local chart paths from %s: %w", component.Name, err)
						}
						paths = append(paths, localChartPaths...)
					}
					paths = append(paths, chart.ValuesFiles...)
				}
			}
			if len(component.Manifests) > 0 {
				for _, manifest := range component.Manifests {
					paths = append(paths, manifest.Files...)
					paths = append(paths, manifest.Kustomizations...)
				}
			}
			if component.Extensions.BigBang != nil && component.Extensions.BigBang.ValuesFiles != nil {
				paths = append(paths, component.Extensions.BigBang.ValuesFiles...)
			}
		}
		for idx := range paths {
			paths[idx] = filepath.Join(base, paths[idx])
		}
		paths = utils.Filter(paths, func(path string) bool {
			return !utils.IsURL(path) && utils.DirHasFile(base, path) && path != base
		})
		paths = utils.Unique(paths)
		// TODO: (@RAZZLE) make the checksums.txt here and include it in `paths` + perform signing
		message.HeaderInfof("📦 PACKAGE PUBLISH %s:%s", p.cfg.Pkg.Metadata.Name, ref.Reference)
		err = p.publish(base, paths, ref)
		if err != nil {
			return fmt.Errorf("unable to publish package %s: %w", ref, err)
		}

		return nil
	}
	if err := p.loadZarfPkg(); err != nil {
		return fmt.Errorf("unable to load the package: %w", err)
	}

	paths := []string{
		p.tmp.ZarfYaml,
		filepath.Join(p.tmp.Images, "index.json"),
		filepath.Join(p.tmp.Images, "oci-layout"),
	}
	// if checksums.txt file exists, include it
	if !utils.InvalidPath(filepath.Join(p.tmp.Base, "checksums.txt")) {
		paths = append(paths, filepath.Join(p.tmp.Base, "checksums.txt"))
	}
	// if p.tmp.SbomTar exists, include it
	if !utils.InvalidPath(p.tmp.SbomTar) {
		paths = append(paths, p.tmp.SbomTar)
	}

	if p.cfg.Pkg.Kind == "ZarfInitConfig" {
		seedImagePaths := []string{
			filepath.Join(p.tmp.SeedImage, "index.json"),
			filepath.Join(p.tmp.SeedImage, "oci-layout"),
		}
		seedImageLayers, err := filepath.Glob(filepath.Join(p.tmp.SeedImage, "blobs", "sha256", "*"))
		if err != nil {
			return err
		}
		seedImagePaths = append(seedImagePaths, seedImageLayers...)
		paths = append(paths, seedImagePaths...)
	}
	componentDirs, err := filepath.Glob(filepath.Join(p.tmp.Components, "*"))
	if err != nil {
		return err
	}
	componentTarballs := []string{}

	// repackage the component directories into tarballs
	for _, componentDir := range componentDirs {
		dst := filepath.Join(p.tmp.Components, filepath.Base(componentDir)+".tar")
		err = archiver.Archive([]string{componentDir}, dst)
		if err != nil {
			return err
		}
		componentTarballs = append(componentTarballs, dst)
		_ = os.RemoveAll(componentDir)
	}
	paths = append(paths, componentTarballs...)
	imagesLayers, err := filepath.Glob(filepath.Join(p.tmp.Images, "blobs", "sha256", "*"))
	if err != nil {
		return err
	}
	paths = append(paths, imagesLayers...)
	ref, err := p.ref("")
	if err != nil {
		return err
	}
	message.HeaderInfof("📦 PACKAGE PUBLISH %s:%s", p.cfg.Pkg.Metadata.Name, ref.Reference)
	err = p.publish(p.tmp.Base, paths, ref)
	if err != nil {
		return fmt.Errorf("unable to publish package %s: %w", ref, err)
	}
	return nil
}

func (p *Packager) publish(base string, paths []string, ref registry.Reference) error {
	message.Infof("Publishing package to %s", ref)
	spinner := message.NewProgressSpinner("")
	defer spinner.Stop()

	// destination remote
	dst, err := utils.NewOrasRemote(ref)
	if err != nil {
		return err
	}
	ctx := dst.Context

	// source file store
	src, err := file.New(p.tmp.Base)
	if err != nil {
		return err
	}
	defer src.Close()

	var descs []ocispec.Descriptor

	for idx, path := range paths {
		name, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		spinner.Updatef("Preparing layer %d/%d: %s", idx+1, len(paths), name)

		mediaType := ZarfLayerMediaTypeBlob

		desc, err := src.Add(ctx, name, mediaType, path)
		if err != nil {
			return err
		}
		descs = append(descs, desc)
	}
	spinner.Successf("Prepared %d layers", len(descs))

	copyOpts := oras.DefaultCopyOptions
	copyOpts.Concurrency = p.cfg.PublishOpts.CopyOptions.Concurrency
	copyOpts.OnCopySkipped = utils.PrintLayerExists
	copyOpts.PostCopy = utils.PrintLayerExists

	var root ocispec.Descriptor

	// try to push an ArtifactManifest first
	// not every registry supports ArtifactManifests, so fallback to an ImageManifest if the push fails
	// see https://oras.land/implementors/#registries-supporting-oci-artifacts
	root, err = p.publishArtifact(dst, src, descs, copyOpts)
	if err != nil {
		// reset the progress bar between attempts
		dst.Transport.ProgressBar.Stop()

		// log the error, the expected error is a 400 manifest invalid
		message.Debug("ArtifactManifest push failed with the following error, falling back to an ImageManifest push:", err)

		// if the error returned from the push is not an expected error, then return the error
		if !isManifestUnsupported(err) {
			return err
		}
		// fallback to an ImageManifest push
		root, err = p.publishImage(dst, src, descs, copyOpts)
		if err != nil {
			return err
		}
	}
	dst.Transport.ProgressBar.Successf("Published %s [%s]", ref, root.MediaType)
	fmt.Println()
	flags := ""
	if config.CommonOptions.Insecure {
		flags = "--insecure"
	}
	message.Info("To inspect/deploy/pull:")
	message.Infof("zarf package inspect oci://%s %s", ref, flags)
	message.Infof("zarf package deploy oci://%s %s", ref, flags)
	message.Infof("zarf package pull oci://%s %s", ref, flags)
	return nil
}

func (p *Packager) publishArtifact(dst *utils.OrasRemote, src *file.Store, descs []ocispec.Descriptor, copyOpts oras.CopyOptions) (root ocispec.Descriptor, err error) {
	var total int64
	for _, desc := range descs {
		total += desc.Size
	}
	packOpts := p.cfg.PublishOpts.PackOptions

	// first attempt to do a ArtifactManifest push
	root, err = p.pack(dst.Context, ocispec.MediaTypeArtifactManifest, descs, src, packOpts)
	if err != nil {
		return root, err
	}
	total += root.Size

	dst.Transport.ProgressBar = message.NewProgressBar(total, fmt.Sprintf("Publishing %s:%s", dst.Reference.Repository, dst.Reference.Reference))
	defer dst.Transport.ProgressBar.Stop()

	// attempt to push the artifact manifest
	_, err = oras.Copy(dst.Context, src, root.Digest.String(), dst, dst.Reference.Reference, copyOpts)
	return root, err
}

func (p *Packager) publishImage(dst *utils.OrasRemote, src *file.Store, descs []ocispec.Descriptor, copyOpts oras.CopyOptions) (root ocispec.Descriptor, err error) {
	var total int64
	for _, desc := range descs {
		total += desc.Size
	}
	// assumes referrers API is not supported since OCI artifact
	// media type is not supported
	dst.SetReferrersCapability(false)

	// fallback to an ImageManifest push
	manifestConfigDesc, manifestConfigContent, err := p.generateManifestConfigFile()
	if err != nil {
		return root, err
	}
	// push the manifest config
	// since this config is so tiny, and the content is not used again
	// it is not logged to the progress, but will error if it fails
	err = dst.Push(dst.Context, manifestConfigDesc, bytes.NewReader(manifestConfigContent))
	if err != nil {
		return root, err
	}
	packOpts := p.cfg.PublishOpts.PackOptions
	packOpts.ConfigDescriptor = &manifestConfigDesc
	packOpts.PackImageManifest = true
	root, err = p.pack(dst.Context, ocispec.MediaTypeImageManifest, descs, src, packOpts)
	if err != nil {
		return root, err
	}
	total += root.Size + manifestConfigDesc.Size

	dst.Transport.ProgressBar = message.NewProgressBar(total, fmt.Sprintf("Publishing %s:%s", dst.Reference.Repository, dst.Reference.Reference))
	defer dst.Transport.ProgressBar.Stop()
	// attempt to push the image manifest
	_, err = oras.Copy(dst.Context, src, root.Digest.String(), dst, dst.Reference.Reference, copyOpts)
	if err != nil {
		return root, err
	}

	return root, nil
}

func (p *Packager) generateAnnotations(artifactType string) map[string]string {
	annotations := map[string]string{
		ocispec.AnnotationDescription: p.cfg.Pkg.Metadata.Description,
	}

	if artifactType == ocispec.MediaTypeArtifactManifest {
		annotations[ocispec.AnnotationTitle] = p.cfg.Pkg.Metadata.Name
	}

	if url := p.cfg.Pkg.Metadata.URL; url != "" {
		annotations[ocispec.AnnotationURL] = url
	}
	if authors := p.cfg.Pkg.Metadata.Authors; authors != "" {
		annotations[ocispec.AnnotationAuthors] = authors
	}
	if documentation := p.cfg.Pkg.Metadata.Documentation; documentation != "" {
		annotations[ocispec.AnnotationDocumentation] = documentation
	}
	if source := p.cfg.Pkg.Metadata.Source; source != "" {
		annotations[ocispec.AnnotationSource] = source
	}
	if vendor := p.cfg.Pkg.Metadata.Vendor; vendor != "" {
		annotations[ocispec.AnnotationVendor] = vendor
	}

	return annotations
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
		ocispec.AnnotationTitle:       p.cfg.Pkg.Metadata.Name,
		ocispec.AnnotationDescription: p.cfg.Pkg.Metadata.Description,
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

// pack creates an artifact/image manifest from the provided descriptors and pushes it to the store
func (p *Packager) pack(ctx context.Context, artifactType string, descs []ocispec.Descriptor, src *file.Store, packOpts oras.PackOptions) (ocispec.Descriptor, error) {
	packOpts.ManifestAnnotations = p.generateAnnotations(artifactType)
	root, err := oras.Pack(ctx, src, artifactType, descs, packOpts)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	if err = src.Tag(ctx, root, root.Digest.String()); err != nil {
		return ocispec.Descriptor{}, err
	}

	return root, nil
}

// ref returns a registry.Reference using metadata from the package's build config and the PublishOpts
//
// if skeleton is not empty, the architecture will be replaced with the skeleton string (e.g. "skeleton")
func (p *Packager) ref(skeleton string) (registry.Reference, error) {
	ver := p.cfg.Pkg.Metadata.Version
	if len(ver) == 0 {
		return registry.Reference{}, errors.New("version is required for publishing")
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
