// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2024-Present Defense Unicorns

package oci

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"

	orasCache "github.com/defenseunicorns/pkg/oci/cache"

	"github.com/defenseunicorns/pkg/helpers/v2"
)

// FileDescriptorExists returns true if the given file exists in the given directory with the expected SHA.
func (o *OrasRemote) FileDescriptorExists(desc ocispec.Descriptor, destinationDir string) bool {
	rel := desc.Annotations[ocispec.AnnotationTitle]
	destinationPath := filepath.Join(destinationDir, rel)

	info, err := os.Stat(destinationPath)
	if err != nil {
		return false
	}
	if info.IsDir() {
		return false
	}
	if info.Size() != desc.Size {
		return false
	}

	f, err := os.Open(destinationPath)
	if err != nil {
		return false
	}
	defer f.Close()

	actual, err := helpers.GetSHA256Hash(f)
	if err != nil {
		return false
	}
	return actual == desc.Digest.Encoded()
}

// CopyToTarget copies the given layers from the remote repository to the given target
func (o *OrasRemote) CopyToTarget(ctx context.Context, layers []ocispec.Descriptor, target oras.Target, copyOpts oras.CopyOptions) error {
	shas := []string{}
	for _, layer := range layers {
		if len(layer.Digest.String()) > 0 {
			shas = append(shas, layer.Digest.Encoded())
		}
	}

	preCopy := copyOpts.PreCopy
	copyOpts.PreCopy = func(ctx context.Context, desc ocispec.Descriptor) error {
		if preCopy != nil {
			if err := preCopy(ctx, desc); err != nil {
				return err
			}
		}

		if slices.Contains(shas, desc.Digest.Encoded()) {
			return nil
		}

		return oras.SkipNode
	}

	var src oras.ReadOnlyTarget
	src = o.repo
	if o.cache != nil {
		src = orasCache.New(o.repo, o.cache)
	}

	_, err := oras.Copy(ctx, src, o.repo.Reference.String(), target, o.repo.Reference.String(), copyOpts)
	if err != nil {
		return err
	}
	return nil
}

// PullPath pulls a layer from the remote repository and saves it to `destinationDir/annotationTitle`.
func (o *OrasRemote) PullPath(ctx context.Context, destinationDir string, desc ocispec.Descriptor) error {
	vr, err := o.FetchLayerReader(ctx, desc)
	if err != nil {
		return err
	}

	rel := desc.Annotations[ocispec.AnnotationTitle]
	if rel == "" {
		return errors.New("failed to pull layer: layer is not a file")
	}

	fullPath := filepath.Join(destinationDir, rel)
	dirPath := filepath.Dir(fullPath)
	if err := helpers.CreateDirectory(dirPath, helpers.ReadExecuteAllWriteUser); err != nil {
		return err
	}

	file, err := os.OpenFile(fullPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, helpers.ReadWriteUser)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := io.Copy(file, vr); err != nil {
		return fmt.Errorf("read failed: %w", err)
	}

	return vr.Verify()
}

// PullPaths pulls multiple files from the remote repository and saves them to `destinationDir`.
func (o *OrasRemote) PullPaths(ctx context.Context, destinationDir string, paths []string) ([]ocispec.Descriptor, error) {
	paths = helpers.Unique(paths)
	root, err := o.FetchRoot(ctx)
	if err != nil {
		return nil, err
	}
	layersPulled := []ocispec.Descriptor{}
	for _, path := range paths {
		desc := root.Locate(path)
		if !IsEmptyDescriptor(desc) {
			layersPulled = append(layersPulled, desc)
			if o.FileDescriptorExists(desc, destinationDir) {
				continue
			}
			err = o.PullPath(ctx, destinationDir, desc)
			if err != nil {
				return nil, err
			}
		}
	}
	return layersPulled, nil
}
