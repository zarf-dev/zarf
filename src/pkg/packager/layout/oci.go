// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/pkg/oci"
	godigest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/convert"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/internal/pkgcfg"
	"github.com/zarf-dev/zarf/src/pkg/images"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/errdef"
)

const (
	// ZarfLayerMediaTypeBlob is the media type for all Zarf package layer blobs.
	ZarfLayerMediaTypeBlob = "application/vnd.zarf.layer.v1.blob"
	// ZarfConfigMediaType is the media type for the Zarf package manifest config.
	ZarfConfigMediaType = "application/vnd.zarf.config.v1+json"
	// ZarfComponentConfigMediaType is the media type for a v1beta1 Zarf component config OCI artifact.
	ZarfComponentConfigMediaType = "application/vnd.zarf.component.config.v1+json"
	// ComponentResourceMountPathAnnotation identifies where a component resource is mounted in its OCI artifact.
	ComponentResourceMountPathAnnotation = "dev.zarf.mountPath"
	// OCITimestampFormat is the format used for the OCI timestamp annotation
	OCITimestampFormat = time.RFC3339
)

// manifestCache holds the computed OCI manifest for the package layout.
// Populated by computeManifest; nil until then.
type manifestCache struct {
	desc         ocispec.Descriptor
	manifestJSON []byte
	configBytes  []byte
	configDigest godigest.Digest
	blobs        map[godigest.Digest]string // layer digest → file path
	totalSize    int64                      // layers + config + manifest
}

// AnnotationsFromMetadata extracts OCI manifest annotations from a package definition.
func AnnotationsFromMetadata(pkg api.Package) map[string]string {
	metadata := pkg.Metadata
	annotations := map[string]string{
		ocispec.AnnotationTitle:       metadata.Name,
		ocispec.AnnotationDescription: metadata.Description,
	}
	if url := metadata.Annotations["metadata.url"]; url != "" {
		annotations[ocispec.AnnotationURL] = url
	}
	if authors := metadata.Annotations["metadata.authors"]; authors != "" {
		annotations[ocispec.AnnotationAuthors] = authors
	}
	if documentation := metadata.Annotations["metadata.documentation"]; documentation != "" {
		annotations[ocispec.AnnotationDocumentation] = documentation
	}
	if source := metadata.Annotations["metadata.source"]; source != "" {
		annotations[ocispec.AnnotationSource] = source
	}
	if vendor := metadata.Annotations["metadata.vendor"]; vendor != "" {
		annotations[ocispec.AnnotationVendor] = vendor
	}
	// FIXME: not sure if this is right
	for key, value := range metadata.Annotations {
		switch key {
		case "metadata.url", "metadata.image", "metadata.authors", "metadata.documentation", "metadata.source", "metadata.vendor":
			continue
		default:
			annotations[key] = value
		}
	}
	return annotations
}

// computeManifest builds the OCI manifest for this layout, caches the result,
// and sets p.digest.
//
// SHA256s for most files are read from checksums.txt (already computed at build
// time), so only the small files excluded from that list (zarf.yaml, checksums.txt
// itself, and post-signing provenance files) are read from disk.
func (p *PackageLayout) computeManifest(ctx context.Context) error {
	// Parse checksums.txt into relpath → sha256hex.
	checksumsPath := filepath.Join(p.dirPath, Checksums)
	checksumsBytes, err := os.ReadFile(checksumsPath)
	if err != nil {
		return fmt.Errorf("reading checksums file: %w", err)
	}
	checksumMap := map[string]string{}
	for _, line := range strings.Split(string(checksumsBytes), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid checksum line: %q", line)
		}
		checksumMap[parts[1]] = parts[0] // relpath → sha256hex
	}

	files, err := p.Files()
	if err != nil {
		return err
	}

	var (
		descs          []ocispec.Descriptor
		totalLayerSize int64
		blobs          = map[godigest.Digest]string{}
	)
	for filePath, name := range files {
		rel, err := filepath.Rel(p.dirPath, filePath)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		var fileDigest godigest.Digest
		var fileSize int64

		switch {
		case checksumMap[rel] != "":
			// Pre-computed hash; only stat for size (no file read).
			fileDigest, err = godigest.Parse("sha256:" + checksumMap[rel])
			if err != nil {
				return fmt.Errorf("invalid checksum for %q: %w", rel, err)
			}
			info, err := os.Stat(filePath)
			if err != nil {
				return err
			}
			fileSize = info.Size()
		case rel == Checksums:
			// checksums.txt is excluded from its own content but is a layer; we
			// already have its bytes from the read above.
			fileDigest = godigest.FromBytes(checksumsBytes)
			fileSize = int64(len(checksumsBytes))
		default:
			// zarf.yaml and post-signing provenance files (signature, bundle) are
			// small — read from disk.
			hex, err := helpers.GetSHA256OfFile(filePath)
			if err != nil {
				return err
			}
			fileDigest, err = godigest.Parse("sha256:" + hex)
			if err != nil {
				return fmt.Errorf("computing sha256 for %q: %w", rel, err)
			}
			info, err := os.Stat(filePath)
			if err != nil {
				return err
			}
			fileSize = info.Size()
		}

		descs = append(descs, ocispec.Descriptor{
			MediaType: ZarfLayerMediaTypeBlob,
			Digest:    fileDigest,
			Size:      fileSize,
			Annotations: map[string]string{
				ocispec.AnnotationTitle: name,
			},
		})
		blobs[fileDigest] = filePath
		totalLayerSize += fileSize
	}

	// Sort by digest for deterministic ordering.
	sort.Slice(descs, func(i, j int) bool {
		return descs[i].Digest.String() < descs[j].Digest.String()
	})

	// Read the zarf.yaml from disk rather than using the in-memory package
	// definition, which may have been component-filtered or otherwise mutated
	// after load.
	zarfYAMLBytes, err := os.ReadFile(filepath.Join(p.dirPath, ZarfYAML))
	if err != nil {
		return fmt.Errorf("reading %s for manifest: %w", ZarfYAML, err)
	}
	config, err := configDefinitionFromZarfYAML(ctx, zarfYAMLBytes)
	if err != nil {
		return fmt.Errorf("parsing %s for manifest: %w", ZarfYAML, err)
	}
	configBytes, err := json.Marshal(config.definition)
	if err != nil {
		return err
	}
	configDesc := content.NewDescriptorFromBytes(ZarfConfigMediaType, configBytes)

	annotations := AnnotationsFromMetadata(config.pkg)

	// Back-compatible timestamp parsing → OCI format. Fall back to zero time (epoch) if the timestamp is absent.
	t, parseErr := time.Parse(api.BuildTimestampFormat, config.pkg.Build.Timestamp)
	if parseErr != nil {
		t = time.Time{}
	}
	annotations[ocispec.AnnotationCreated] = t.UTC().Format(OCITimestampFormat)

	memStore := memory.New()
	root, err := oras.PackManifest(ctx, memStore, oras.PackManifestVersion1_1, "", oras.PackManifestOptions{
		Layers:              descs,
		ConfigDescriptor:    &configDesc,
		ManifestAnnotations: annotations,
	})
	if err != nil {
		return fmt.Errorf("unable to pack manifest: %w", err)
	}

	manifestReader, err := memStore.Fetch(ctx, root)
	if err != nil {
		return fmt.Errorf("fetching packed manifest: %w", err)
	}
	manifestJSON, readErr := io.ReadAll(manifestReader)
	if err := errors.Join(readErr, manifestReader.Close()); err != nil {
		return fmt.Errorf("reading packed manifest: %w", err)
	}

	p.cache = &manifestCache{
		desc:         root,
		manifestJSON: manifestJSON,
		configBytes:  configBytes,
		configDigest: configDesc.Digest,
		blobs:        blobs,
		totalSize:    totalLayerSize + int64(len(configBytes)) + root.Size,
	}
	p.digest = root.Digest.String()
	return nil
}

type ociConfigDefinition struct {
	definition any
	pkg        api.Package
}

func configDefinitionFromZarfYAML(ctx context.Context, definition []byte) (ociConfigDefinition, error) {
	version, err := pkgcfg.SelectVersion(ctx, definition)
	if err != nil {
		return ociConfigDefinition{}, err
	}

	switch version {
	case v1alpha1.APIVersion:
		pkg, err := pkgcfg.ParseAs(ctx, definition, pkgcfg.V1Alpha1)
		if err != nil {
			return ociConfigDefinition{}, err
		}
		return ociConfigDefinition{definition: pkg, pkg: convert.PackageFromV1alpha1(pkg)}, nil
	case v1beta1.APIVersion:
		pkg, err := pkgcfg.ParseAs(ctx, definition, pkgcfg.V1Beta1)
		if err != nil {
			return ociConfigDefinition{}, err
		}
		return ociConfigDefinition{definition: pkg, pkg: convert.PackageFromV1beta1(pkg)}, nil
	default:
		return ociConfigDefinition{}, fmt.Errorf("unsupported package apiVersion %q", version)
	}
}

// SetRegistryDigest records the manifest digest as resolved from a registry.
// It replaces the locally-computed digest and clears the manifest cache, since
// the registry manifest may differ (e.g. partial OCI pulls). After this call
// the layout is no longer usable as an oras.ReadOnlyTarget for pushing.
func (p *PackageLayout) SetRegistryDigest(digest string) {
	p.digest = digest
	p.cache = nil
}

// IsPushable reports whether this layout has a computed manifest cache and can
// be used as a push source. A layout with only a registry digest (e.g. from a
// partial OCI pull via SetRegistryDigest) returns false because the cache is nil.
func (p *PackageLayout) IsPushable() bool {
	return p.cache != nil
}

// TotalSize returns the total bytes that would be pushed for this package (all
// layers + config + manifest). Returns 0 if the manifest has not been computed.
func (p *PackageLayout) TotalSize() int64 {
	if p.cache == nil {
		return 0
	}
	return p.cache.totalSize
}

// Fetch implements oras.ReadOnlyTarget. It serves the manifest, config, or a
// layer blob identified by the descriptor's digest.
func (p *PackageLayout) Fetch(_ context.Context, target ocispec.Descriptor) (io.ReadCloser, error) {
	if p.cache == nil {
		return nil, errdef.ErrNotFound
	}
	switch target.Digest {
	case p.cache.desc.Digest:
		return io.NopCloser(bytes.NewReader(p.cache.manifestJSON)), nil
	case p.cache.configDigest:
		return io.NopCloser(bytes.NewReader(p.cache.configBytes)), nil
	}
	if filePath, ok := p.cache.blobs[target.Digest]; ok {
		return os.Open(filePath)
	}
	return nil, errdef.ErrNotFound
}

// Manifest returns the package's computed OCI manifest for use in zoci functions
func (p *PackageLayout) Manifest() (*oci.Manifest, error) {
	if p.cache == nil {
		return nil, errors.New("package OCI manifest has not been computed")
	}
	var m oci.Manifest
	if err := json.Unmarshal(p.cache.manifestJSON, &m); err != nil {
		return nil, fmt.Errorf("parsing computed manifest: %w", err)
	}
	return &m, nil
}

// Exists implements oras.ReadOnlyTarget.
func (p *PackageLayout) Exists(_ context.Context, target ocispec.Descriptor) (bool, error) {
	if p.cache == nil {
		return false, nil
	}
	if target.Digest == p.cache.desc.Digest || target.Digest == p.cache.configDigest {
		return true, nil
	}
	_, ok := p.cache.blobs[target.Digest]
	return ok, nil
}

// Resolve implements oras.ReadOnlyTarget. It accepts the manifest digest or the
// package name as a reference.
func (p *PackageLayout) Resolve(_ context.Context, reference string) (ocispec.Descriptor, error) {
	if p.cache == nil {
		return ocispec.Descriptor{}, errdef.ErrNotFound
	}
	if reference == p.digest || reference == p.AsV1alpha1().Metadata.Name {
		return p.cache.desc, nil
	}
	return ocispec.Descriptor{}, errdef.ErrNotFound
}

// HasImageIndex reports whether the package layout has a multi-platform image.
// It takes a directory rather than a PackageLayout so callers assembling a
// package layout may use it as well.
func HasImageIndex(imageDir string) (bool, error) {
	idxPath := filepath.Join(imageDir, IndexJSON)
	b, err := os.ReadFile(idxPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("failed to read %s: %w", idxPath, err)
	}
	var idx ocispec.Index
	if err := json.Unmarshal(b, &idx); err != nil {
		return false, fmt.Errorf("failed to parse %s: %w", idxPath, err)
	}
	for _, m := range idx.Manifests {
		if images.IsIndex(m.MediaType) {
			return true, nil
		}
	}
	return false, nil
}
