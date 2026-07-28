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
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/defenseunicorns/pkg/helpers/v2"
	godigest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/internal/pkgcfg"
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

// AnnotationsFromMetadata extracts OCI manifest annotations from Zarf package metadata.
func AnnotationsFromMetadata(metadata v1alpha1.ZarfMetadata) map[string]string {
	annotations := map[string]string{
		ocispec.AnnotationTitle:       metadata.Name,
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
	// annotations explicitly defined in metadata.Annotations take precedence over legacy fields.
	maps.Copy(annotations, metadata.Annotations)
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

	// Read the zarf.yaml from disk rather than using p.Pkg, which may have been
	// component-filtered or otherwise mutated after load.
	zarfYAMLBytes, err := os.ReadFile(filepath.Join(p.dirPath, ZarfYAML))
	if err != nil {
		return fmt.Errorf("reading %s for manifest: %w", ZarfYAML, err)
	}
	zarfPkg, err := pkgcfg.ParseMultiDoc(ctx, zarfYAMLBytes)
	if err != nil {
		return fmt.Errorf("parsing %s for manifest: %w", ZarfYAML, err)
	}
	configBytes, err := json.Marshal(zarfPkg)
	if err != nil {
		return err
	}
	configDesc := content.NewDescriptorFromBytes(ZarfConfigMediaType, configBytes)

	annotations := AnnotationsFromMetadata(zarfPkg.Metadata)

	// Back-compatible timestamp parsing → OCI format. Fall back to zero time (epoch) if the timestamp is absent.
	t, parseErr := time.Parse(v1alpha1.BuildTimestampFormat, zarfPkg.Build.Timestamp)
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
	if reference == p.digest || reference == p.Pkg.Metadata.Name {
		return p.cache.desc, nil
	}
	return ocispec.Descriptor{}, errdef.ErrNotFound
}
