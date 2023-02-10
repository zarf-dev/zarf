// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	v1name "github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry/remote"
)

// Publish publishes the package to a registry
//
// This is a wrapper around the oras library
// and much of the code was adapted from the oras CLI - https://github.com/oras-project/oras/blob/main/cmd/oras/push.go
//
// Authentication is handled via the Docker config file created w/ `docker login`
func (p *Packager) Publish() error {
	p.cfg.DeployOpts.PackagePath = p.cfg.PublishOpts.PackagePath
	if err := p.loadZarfPkg(); err != nil {
		return fmt.Errorf("unable to load the package: %w", err)
	}
	spinner := message.NewProgressSpinner("")
	defer spinner.Stop()

	if p.cfg.PublishOpts.RegistryURL == "docker.io" {
		// docker.io is commonly used, but is not a valid registry URL
		// registry-1.docker.io is Docker's default public registry URL
		// https://github.com/docker/cli/blob/master/man/src/image/pull.md
		p.cfg.PublishOpts.RegistryURL = "registry-1.docker.io"
	}

	paths := []string{
		filepath.Join(p.tmp.Base, "checksums.txt"),
		filepath.Join(p.tmp.Base, "zarf.yaml"),
		filepath.Join(p.tmp.Base, "sboms.tar.zst"),
	}
	componentTarballs, err := filepath.Glob(filepath.Join(p.tmp.Base, "components", "*.tar.zst"))
	if err != nil {
		return err
	}
	paths = append(paths, componentTarballs...)
	if p.cfg.PublishOpts.IncludeImages {
		imagesLayers, err := filepath.Glob(filepath.Join(p.tmp.Base, "images", "*"))
		if err != nil {
			return err
		}
		paths = append(paths, imagesLayers...)
		ref, err := p.ref("")
		if err != nil {
			return err
		}
		message.HeaderInfof("📦 PACKAGE PUBLISH %s", ref.Name())
		err = p.publish(ref, paths, spinner)
		if err != nil {
			return fmt.Errorf("unable to publish package %s: %w", ref, err)
		}
	}

	// push the skeleton (package w/o the images)
	skeletonRef, err := p.ref("skeleton")
	if err != nil {
		return err
	}
	skeletonPaths := []string{}
	for idx, path := range paths {
		// remove images if they exist
		if !strings.HasPrefix(path, filepath.Join(p.tmp.Base, "images")) {
			skeletonPaths = append(skeletonPaths, paths[idx])
		}
	}
	message.HeaderInfof("📦 PACKAGE PUBLISH %s", skeletonRef.Name())
	err = p.publish(skeletonRef, skeletonPaths, spinner)
	if err != nil {
		return fmt.Errorf("unable to publish package %s: %w", skeletonRef, err)
	}

	return nil
}

func (p *Packager) generateManifestConfigFile(ctx context.Context, store *file.Store) (ocispec.Descriptor, error) {
	// Unless specified, an empty manifest config will be used: `{}`
	// which causes an error on Google Artifact Registry
	// to negate this, we create a simple manifest config with some build metadata
	manifestConfig := v1.ConfigFile{
		Architecture: p.cfg.Pkg.Build.Architecture,
		Author:       p.cfg.Pkg.Build.User,
		Variant:      "zarf-package",
	}
	manifestConfigBytes, err := json.Marshal(manifestConfig)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	err = os.WriteFile(filepath.Join(p.tmp.Base, "config.json"), manifestConfigBytes, 0600)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	manifestConfigPath := filepath.Join(p.tmp.Base, "config.json")
	manifestConfigDesc, err := store.Add(ctx, "config.json", ocispec.MediaTypeImageConfig, manifestConfigPath)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	return manifestConfigDesc, nil
}

func (p *Packager) publish(ref v1name.Reference, paths []string, spinner *message.Spinner) error {
	message.Debugf("Publishing package to %s", ref)
	spinner.Updatef("Publishing package to: %s", ref)

	fullname := fmt.Sprintf("%s/%s", p.cfg.PublishOpts.Namespace, p.cfg.Pkg.Metadata.Name)
	ctx := utils.CtxWithScopes(fullname)

	dst, err := remote.NewRepository(ref.String())
	if err != nil {
		return err
	}
	authClient, err := utils.AuthClient(ref)
	if err != nil {
		return err
	}
	dst.Client = authClient

	if p.cfg.PublishOpts.PlainHTTP {
		dst.PlainHTTP = true
	}

	store, err := file.New("")
	if err != nil {
		return err
	}
	defer store.Close()

	manifestConfigDesc, err := p.generateManifestConfigFile(ctx, store)
	if err != nil {
		return err
	}

	var descs []ocispec.Descriptor

	for _, path := range paths {
		name, err := filepath.Rel(p.tmp.Base, path)
		if err != nil {
			return err
		}

		mediaType := utils.ParseZarfLayerMediaType(name)

		desc, err := store.Add(ctx, name, mediaType, path)
		if err != nil {
			return err
		}
		descs = append(descs, desc)
	}
	packOpts := oras.PackOptions{}
	packOpts.ConfigDescriptor = &manifestConfigDesc
	pack := func() (ocispec.Descriptor, error) {
		// note the empty string for the artifactType
		// this is because oras handles this type under the hood if left blank
		root, err := oras.Pack(ctx, store, "", descs, packOpts)
		if err != nil {
			return ocispec.Descriptor{}, err
		}
		if err = store.Tag(ctx, root, root.Digest.String()); err != nil {
			return ocispec.Descriptor{}, err
		}
		return root, nil
	}

	copyOpts := oras.DefaultCopyOptions
	if p.cfg.PublishOpts.Concurrency > copyOpts.Concurrency {
		copyOpts.Concurrency = p.cfg.PublishOpts.Concurrency
	}
	copyOpts.OnCopySkipped = func(ctx context.Context, desc ocispec.Descriptor) error {
		message.Debug("layer", desc.Digest.Hex()[:12], "exists")
		return nil
	}
	copyOpts.PostCopy = func(ctx context.Context, desc ocispec.Descriptor) error {
		if desc.Annotations[ocispec.AnnotationTitle] != "" {
			message.SuccessF("%s %s", desc.Digest.Hex()[:12], desc.Annotations[ocispec.AnnotationTitle])
		} else {
			message.SuccessF("%s [%s]", desc.Digest.Hex()[:12], desc.MediaType)
		}
		return nil
	}

	push := func(root ocispec.Descriptor) error {
		message.Debugf("root descriptor: %v\n", root)
		tag := dst.Reference.Reference
		_, err := oras.Copy(ctx, store, root.Digest.String(), dst, tag, copyOpts)
		return err
	}

	// first attempt to do a ArtifactManifest push
	root, err := pack()
	if err != nil {
		return err
	}

	copyRootAttempted := false
	preCopy := copyOpts.PreCopy
	copyOpts.PreCopy = func(ctx context.Context, desc ocispec.Descriptor) error {
		message.Debug("layer", desc.Digest.Hex()[:12], "is being pushed")
		if desc.Annotations[ocispec.AnnotationTitle] != "" {
			spinner.Updatef("%s %s", desc.Digest.Hex()[:12], desc.Annotations[ocispec.AnnotationTitle])
		} else {
			spinner.Updatef("%s [%s]", desc.Digest.Hex()[:12], desc.MediaType)
		}
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

	// attempt to push the artifact manifest
	if err = push(root); err == nil {
		spinner.Updatef("Published: %s [%s]", ref, root.MediaType)
		message.SuccessF("Published: %s [%s]", ref, root.MediaType)
		message.SuccessF("Digest: %s", root.Digest)
		return nil
	}
	// log the error, the expected error is a 400 manifest invalid
	message.Debug("ArtifactManifest push failed with the following error, falling back to an ImageManifest push:", err)

	// if copyRootAttempted is false here, then there was an error generated before
	// the root was copied. This is unexpected, so return the error.
	if !copyRootAttempted {
		return err
	}

	// due to oras handling of the root descriptor, if the code reaches this point then
	// the root.MediaType cannot be an artifact manifest.
	if root.MediaType != ocispec.MediaTypeArtifactManifest {
		return fmt.Errorf("artifact manifest push already failed, yet the root media type is still an artifact manifest %w", err)
	}

	// if the error returned from the push is not an expected error, then return the error
	if !utils.IsManifestUnsupported(err) {
		return err
	}

	// assumes referrers API is not supported since OCI artifact
	// media type is not supported
	dst.SetReferrersCapability(false)

	// fallback to an ImageManifest push
	packOpts.PackImageManifest = true
	root, err = pack()
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

	if err = push(root); err != nil {
		return err
	}
	spinner.Updatef("Published: %s [%s]", ref, root.MediaType)
	message.SuccessF("Published: %s [%s]", ref, root.MediaType)
	message.SuccessF("Digest: %s", root.Digest)
	return nil
}

// ref returns a v1name.Reference using metadata from the package's build config and the PublishOpts
//
// if skeleton is not empty, the architecture will be replaced with the skeleton string (e.g. "skeleton")
func (p *Packager) ref(skeleton string) (v1name.Reference, error) {
	name := p.cfg.Pkg.Metadata.Name
	ver := p.cfg.Pkg.Build.Version
	arch := p.cfg.Pkg.Build.Architecture
	// changes package ref from "name:version-arch" to "name:version-skeleton"
	if len(skeleton) > 0 {
		arch = skeleton
	}
	ns := p.cfg.PublishOpts.Namespace
	registry := p.cfg.PublishOpts.RegistryURL
	ref, err := v1name.ParseReference(fmt.Sprintf("%s/%s/%s:%s-%s", registry, ns, name, ver, arch), v1name.StrictValidation)
	if err != nil {
		return nil, err
	}
	return ref, nil
}
