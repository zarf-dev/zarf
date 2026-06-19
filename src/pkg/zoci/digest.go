// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package zoci

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	godigest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"
)

// DigestForLayout computes the OCI manifest digest for a local package layout
// without pushing to a registry. The result matches the digest that would be
// assigned to the package if published with PushPackage.
func DigestForLayout(ctx context.Context, pkgLayout *layout.PackageLayout) (string, error) {
	files, err := pkgLayout.Files()
	if err != nil {
		return "", fmt.Errorf("unable to list package files: %w", err)
	}

	var descs []ocispec.Descriptor
	for filePath, name := range files {
		desc, err := descriptorForFile(filePath, name)
		if err != nil {
			return "", fmt.Errorf("unable to compute descriptor for %s: %w", name, err)
		}
		descs = append(descs, desc)
	}
	sort.Slice(descs, func(i, j int) bool {
		return descs[i].Digest < descs[j].Digest
	})

	annotations := annotationsFromMetadata(pkgLayout.Pkg.Metadata)
	t, err := time.Parse(v1alpha1.BuildTimestampFormat, pkgLayout.Pkg.Build.Timestamp)
	if err != nil {
		return "", fmt.Errorf("unable to parse build timestamp: %w", err)
	}
	annotations[ocispec.AnnotationCreated] = t.Format(OCITimestampFormat)

	configBytes, err := json.Marshal(pkgLayout.Pkg)
	if err != nil {
		return "", fmt.Errorf("unable to marshal package config: %w", err)
	}
	configDesc := content.NewDescriptorFromBytes(ZarfConfigMediaType, configBytes)

	packOpts := oras.PackManifestOptions{
		Layers:              descs,
		ConfigDescriptor:    &configDesc,
		ManifestAnnotations: annotations,
	}

	store := memory.New()
	root, err := oras.PackManifest(ctx, store, oras.PackManifestVersion1_1_RC4, "", packOpts)
	if err != nil {
		return "", fmt.Errorf("unable to pack manifest: %w", err)
	}

	return root.Digest.String(), nil
}

func descriptorForFile(filePath, annotationTitle string) (ocispec.Descriptor, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	defer f.Close()

	digester := godigest.SHA256.Digester()
	size, err := io.Copy(digester.Hash(), f)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	return ocispec.Descriptor{
		MediaType: ZarfLayerMediaTypeBlob,
		Digest:    digester.Digest(),
		Size:      size,
		Annotations: map[string]string{
			ocispec.AnnotationTitle: annotationTitle,
		},
	}, nil
}
