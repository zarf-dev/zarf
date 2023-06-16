// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package oci contains functions for interacting with Zarf packages stored in OCI registries.
package oci

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/file"
)

// ConfigPartial is a partial OCI config that is used to create the manifest config.
//
// Unless specified, an empty manifest config will be used: `{}`
// which causes an error on Google Artifact Registry
//
// to negate this, we create a simple manifest config with some build metadata
//
// the contents of this file are not used by Zarf
type ConfigPartial struct {
	Architecture string            `json:"architecture"`
	OCIVersion   string            `json:"ociVersion"`
	Annotations  map[string]string `json:"annotations,omitempty"`
}

// PushFile pushes the file at the given path to the remote repository.
func (o *OrasRemote) PushFile(path string) (*ocispec.Descriptor, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return o.PushBytes(b, ZarfLayerMediaTypeBlob)
}

// PushBytes pushes the given bytes to the remote repository.
func (o *OrasRemote) PushBytes(b []byte, mediaType string) (*ocispec.Descriptor, error) {
	desc := content.NewDescriptorFromBytes(mediaType, b)
	return &desc, o.Push(o.Context, desc, bytes.NewReader(b))
}

func (o *OrasRemote) pushManifestConfigFromMetadata(metadata *types.ZarfMetadata, build *types.ZarfBuildData) (*ocispec.Descriptor, error) {
	annotations := map[string]string{
		ocispec.AnnotationTitle:       metadata.Name,
		ocispec.AnnotationDescription: metadata.Description,
	}
	manifestConfig := ConfigPartial{
		Architecture: build.Architecture,
		OCIVersion:   "1.0.1",
		Annotations:  annotations,
	}
	manifestConfigBytes, err := json.Marshal(manifestConfig)
	if err != nil {
		return nil, err
	}
	return o.PushBytes(manifestConfigBytes, ocispec.MediaTypeImageConfig)
}

func (o *OrasRemote) manifestAnnotationsFromMetadata(metadata *types.ZarfMetadata) map[string]string {
	annotations := map[string]string{
		ocispec.AnnotationDescription: metadata.Description,
	}

	if url := metadata.URL; url != "" {
		annotations[ocispec.AnnotationURL] = url
	}
	if authors := metadata.Authors; authors != "" {
		annotations[ocispec.AnnotationAuthors] = authors
	}
	if documentation := metadata.Documentation; documentation != "" {
		annotations[ocispec.AnnotationDocumentation] = documentation
	}
	if source := metadata.Source; source != "" {
		annotations[ocispec.AnnotationSource] = source
	}
	if vendor := metadata.Vendor; vendor != "" {
		annotations[ocispec.AnnotationVendor] = vendor
	}

	return annotations
}

func (o *OrasRemote) generatePackManifest(src *file.Store, descs []ocispec.Descriptor, configDesc *ocispec.Descriptor, metadata *types.ZarfMetadata) (ocispec.Descriptor, error) {
	packOpts := oras.PackOptions{}
	packOpts.ConfigDescriptor = configDesc
	packOpts.PackImageManifest = true
	packOpts.ManifestAnnotations = o.manifestAnnotationsFromMetadata(metadata)

	root, err := oras.Pack(o.Context, src, ocispec.MediaTypeImageManifest, descs, packOpts)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	if err = src.Tag(o.Context, root, root.Digest.String()); err != nil {
		return ocispec.Descriptor{}, err
	}

	return root, nil
}

// PublishPackage publishes the package to the remote repository.
func (o *OrasRemote) PublishPackage(pkg *types.ZarfPackage, sourceDir string, concurrency int) error {
	ctx := o.Context
	// source file store
	src, err := file.New(sourceDir)
	if err != nil {
		return err
	}
	defer src.Close()

	message.Infof("Publishing package to %s", o.Reference.String())
	spinner := message.NewProgressSpinner("")
	defer spinner.Stop()

	// Get all of the layers in the package
	paths := []string{}
	err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		// Catch any errors that happened during the walk
		if err != nil {
			return err
		}

		// Add any resource that is not a directory to the paths of objects we will include into the package
		if !info.IsDir() {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("unable to get the layers in the package to publish: %w", err)
	}

	var descs []ocispec.Descriptor
	for idx, path := range paths {
		name, err := filepath.Rel(sourceDir, path)
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

	copyOpts := o.CopyOpts
	copyOpts.Concurrency = concurrency
	var total int64
	for _, desc := range descs {
		total += desc.Size
	}
	// assumes referrers API is not supported since OCI artifact
	// media type is not supported
	o.SetReferrersCapability(false)

	// push the manifest config
	// since this config is so tiny, and the content is not used again
	// it is not logged to the progress, but will error if it fails
	manifestConfigDesc, err := o.pushManifestConfigFromMetadata(&pkg.Metadata, &pkg.Build)
	if err != nil {
		return err
	}
	root, err := o.generatePackManifest(src, descs, manifestConfigDesc, &pkg.Metadata)
	if err != nil {
		return err
	}
	total += root.Size + manifestConfigDesc.Size

	o.Transport.ProgressBar = message.NewProgressBar(total, fmt.Sprintf("Publishing %s:%s", o.Reference.Repository, o.Reference.Reference))
	defer o.Transport.ProgressBar.Stop()
	// attempt to push the image manifest
	_, err = oras.Copy(ctx, src, root.Digest.String(), o, o.Reference.Reference, copyOpts)
	if err != nil {
		return err
	}

	o.Transport.ProgressBar.Successf("Published %s [%s]", o.Reference, root.MediaType)
	message.HorizontalRule()
	if strings.HasSuffix(o.Reference.String(), SkeletonSuffix) {
		message.Title("How to import components from this skeleton:", "")
		ex := []types.ZarfComponent{}
		for _, c := range pkg.Components {
			ex = append(ex, types.ZarfComponent{
				Name: fmt.Sprintf("import-%s", c.Name),
				Import: types.ZarfComponentImport{
					ComponentName: c.Name,
					URL:           fmt.Sprintf("oci://%s", o.Reference),
				},
			})
		}
		utils.ColorPrintYAML(ex, nil)
	} else {
		flags := ""
		if config.CommonOptions.Insecure {
			flags = "--insecure"
		}
		message.Title("To inspect/deploy/pull:", "")
		message.ZarfCommand("package inspect oci://%s %s", o.Reference, flags)
		message.ZarfCommand("package deploy oci://%s %s", o.Reference, flags)
		message.ZarfCommand("package pull oci://%s %s", o.Reference, flags)
	}

	return nil
}
