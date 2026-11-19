/*
Copyright The ORAS Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package oras

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"time"

	"github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/oras-project/oras-go/v3/content"
	"github.com/oras-project/oras-go/v3/errdef"
	"github.com/oras-project/oras-go/v3/internal/descriptor"
	"github.com/oras-project/oras-go/v3/internal/spec"
)

const (
	// MediaTypeUnknownConfig is the default config mediaType used
	//   - for [Pack] when PackOptions.PackImageManifest is true and
	//     PackOptions.ConfigDescriptor is not specified.
	//   - for [PackManifest] when packManifestVersion is PackManifestVersion1_0
	//     and PackManifestOptions.ConfigDescriptor is not specified.
	MediaTypeUnknownConfig = "application/vnd.unknown.config.v1+json"

	// MediaTypeUnknownArtifact is the default artifactType used for [Pack]
	// when PackOptions.PackImageManifest is false and artifactType is
	// not specified.
	MediaTypeUnknownArtifact = "application/vnd.unknown.artifact.v1"
)

var (
	// ErrInvalidDateTimeFormat is returned by [Pack], [PackManifest], and [PackIndex] when
	// "org.opencontainers.artifact.created" or "org.opencontainers.image.created"
	// is provided, but its value is not in RFC 3339 format.
	// Reference: https://www.rfc-editor.org/rfc/rfc3339#section-5.6
	ErrInvalidDateTimeFormat = errors.New("invalid date and time format")

	// ErrMissingArtifactType is returned by [PackManifest] when
	// packManifestVersion is PackManifestVersion1_1 and artifactType is
	// empty and the config media type is set to
	// "application/vnd.oci.empty.v1+json".
	ErrMissingArtifactType = errors.New("missing artifact type")
)

// PackManifestVersion represents the manifest version used for [PackManifest].
type PackManifestVersion int

const (
	// PackManifestVersion1_0 represents the OCI Image Manifest defined in
	// image-spec v1.0.2.
	// Reference: https://github.com/opencontainers/image-spec/blob/v1.0.2/manifest.md
	PackManifestVersion1_0 PackManifestVersion = 1

	// PackManifestVersion1_1_RC4 represents the OCI Image Manifest defined
	// in image-spec v1.1.0-rc4.
	// Reference: https://github.com/opencontainers/image-spec/blob/v1.1.0-rc4/manifest.md
	//
	// Deprecated: This constant is deprecated and not recommended for future use.
	// Use [PackManifestVersion1_1] instead.
	PackManifestVersion1_1_RC4 PackManifestVersion = PackManifestVersion1_1

	// PackManifestVersion1_1 represents the OCI Image Manifest defined in
	// image-spec v1.1.1.
	// Reference: https://github.com/opencontainers/image-spec/blob/v1.1.1/manifest.md
	PackManifestVersion1_1 PackManifestVersion = 2
)

// PackManifestOptions contains optional parameters for [PackManifest].
type PackManifestOptions struct {
	// Subject is the subject of the manifest.
	// This option is only valid when PackManifestVersion is
	// NOT PackManifestVersion1_0.
	Subject *ocispec.Descriptor

	// Layers is the layers of the manifest.
	Layers []ocispec.Descriptor

	// ManifestAnnotations is the annotation map of the manifest. In order to
	// make [PackManifest] reproducible, set the key ocispec.AnnotationCreated
	// (i.e. "org.opencontainers.image.created") to a fixed value. The value
	// must conform to RFC 3339.
	ManifestAnnotations map[string]string

	// ConfigDescriptor is a pointer to the descriptor of the config blob.
	// If not nil, the caller is responsible for pushing the config blob to
	// pusher and populating Digest and Size. ConfigAnnotations will be ignored.
	ConfigDescriptor *ocispec.Descriptor

	// ConfigAnnotations is the annotation map of the config descriptor.
	// This option is valid only when ConfigDescriptor is nil.
	ConfigAnnotations map[string]string

	// DigestAlgorithm is the algorithm used to compute the digest of the
	// manifest, and of the config and layer blobs that [PackManifest]
	// generates. Descriptors supplied in Subject, Layers and ConfigDescriptor
	// keep their own digests.
	// If empty, [digest.Canonical] (SHA-256) is used. SHA-384 and SHA-512 are
	// the other supported algorithms, and must be available, which requires
	// their hash package to be linked into the binary (for example by
	// importing crypto/sha512).
	// With a non-canonical algorithm, the generated empty config and layer
	// digests differ from [ocispec.DescriptorEmptyJSON].
	DigestAlgorithm digest.Algorithm
}

// PackIndexOptions contains optional parameters for [PackIndex].
type PackIndexOptions struct {
	// Subject is the subject of the index.
	Subject *ocispec.Descriptor

	// Manifests is the descriptors of the manifests or indexes referenced by
	// the index. The caller is responsible for pushing the referenced content
	// to the target before calling [PackIndex]; PackIndex only pushes the
	// index itself.
	Manifests []ocispec.Descriptor

	// IndexAnnotations is the annotation map of the index. In order to
	// make [PackIndex] reproducible, set the key ocispec.AnnotationCreated
	// (i.e. "org.opencontainers.image.created") to a fixed value. The value
	// must conform to RFC 3339.
	IndexAnnotations map[string]string

	// DigestAlgorithm is the algorithm used to compute the digest of the
	// index. Descriptors supplied in Subject and Manifests keep their own
	// digests.
	// If empty, [digest.Canonical] (SHA-256) is used. SHA-384 and SHA-512 are
	// the other supported algorithms, and must be available, which requires
	// their hash package to be linked into the binary (for example by
	// importing crypto/sha512).
	DigestAlgorithm digest.Algorithm
}

// mediaTypeRegexp checks the format of media types.
// References:
//   - https://github.com/opencontainers/image-spec/blob/v1.1.1/schema/defs-descriptor.json#L7
//   - https://datatracker.ietf.org/doc/html/rfc6838#section-4.2
var mediaTypeRegexp = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9!#$&^_.+-]{0,126}/[A-Za-z0-9][A-Za-z0-9!#$&^_.+-]{0,126}$`)

// PackManifest generates an OCI Image Manifest based on the given parameters
// and pushes the packed manifest to a content storage using pusher. The version
// of the manifest to be packed is determined by packManifestVersion
// (Recommended value: PackManifestVersion1_1).
//
//   - If packManifestVersion is [PackManifestVersion1_1]:
//     artifactType MUST NOT be empty unless opts.ConfigDescriptor is specified.
//   - If packManifestVersion is [PackManifestVersion1_0]:
//     if opts.ConfigDescriptor is nil, artifactType will be used as the
//     config media type; if artifactType is empty,
//     "application/vnd.unknown.config.v1+json" will be used.
//     if opts.ConfigDescriptor is NOT nil, artifactType will be ignored.
//
// artifactType and opts.ConfigDescriptor.MediaType MUST comply with RFC 6838.
//
// Each time when PackManifest is called, if a time stamp is not specified, a new time
// stamp is generated in the manifest annotations with the key ocispec.AnnotationCreated
// (i.e. "org.opencontainers.image.created"). To make [PackManifest] reproducible,
// set the key ocispec.AnnotationCreated to a fixed value in
// opts.ManifestAnnotations. The value MUST conform to RFC 3339.
//
// If succeeded, returns a descriptor of the packed manifest.
func PackManifest(ctx context.Context, pusher content.Pusher, packManifestVersion PackManifestVersion, artifactType string, opts PackManifestOptions) (ocispec.Descriptor, error) {
	return packManifest(ctx, pusher, packManifestVersion, artifactType, opts)
}

// PackManifestN packs an OCI Image Manifest like [PackManifest], pushes it to
// target, and tags it with each of the given references.
//
// If target implements [registry.ReferencePusher] (as a remote repository
// does), the manifest is pushed once per reference with a single request, so
// no separate tag request is needed. Otherwise, the manifest is pushed and
// then tagged with [content.Tagger].
//
// If references is empty, PackManifestN behaves like [PackManifest]. If the
// packed manifest already exists in target, it is still tagged with
// references. Only the manifest is tagged; config and layers are pushed as in
// [PackManifest]. If tagging with a reference fails, the returned error names
// that reference, and some other references may already have been applied.
//
// Duplicate references are applied only once. Digest-form references are passed
// to target unchanged; validation follows target's Tag or PushReference behavior.
//
// If succeeded, returns a descriptor of the packed manifest.
func PackManifestN(ctx context.Context, target Target, packManifestVersion PackManifestVersion, artifactType string, references []string, opts PackManifestOptions) (ocispec.Descriptor, error) {
	return packManifest(ctx, withTags(target, references), packManifestVersion, artifactType, opts)
}

func packManifest(ctx context.Context, pusher content.Pusher, packManifestVersion PackManifestVersion, artifactType string, opts PackManifestOptions) (ocispec.Descriptor, error) {
	if opts.DigestAlgorithm == "" {
		opts.DigestAlgorithm = digest.Canonical
	}
	if !descriptor.IsSupportedAlgorithm(opts.DigestAlgorithm) || !opts.DigestAlgorithm.Available() {
		return ocispec.Descriptor{}, fmt.Errorf("digest algorithm %q: %w", opts.DigestAlgorithm, errdef.ErrUnsupported)
	}
	switch packManifestVersion {
	case PackManifestVersion1_0:
		return packManifestV1_0(ctx, pusher, artifactType, opts)
	case PackManifestVersion1_1:
		return packManifestV1_1(ctx, pusher, artifactType, opts)
	default:
		return ocispec.Descriptor{}, fmt.Errorf("PackManifestVersion(%v): %w", packManifestVersion, errdef.ErrUnsupported)
	}
}

// PackIndex generates an OCI Image Index based on the given parameters and
// pushes the packed index to a content storage using pusher.
//
// artifactType is optional. If not empty, it MUST comply with RFC 6838.
//
// PackIndex only pushes the index itself. It does not push, nor verify the
// existence or validity of, opts.Subject and the descriptors in
// opts.Manifests. The caller is responsible for making the referenced content
// available in the target; some targets, such as registries, may reject an
// index that references missing content.
//
// Each time when PackIndex is called, if a time stamp is not specified, a new
// time stamp is generated in the index annotations with the key
// ocispec.AnnotationCreated (i.e. "org.opencontainers.image.created"), which
// changes the digest of the packed index. To make [PackIndex] reproducible,
// set the key ocispec.AnnotationCreated to a fixed value in
// opts.IndexAnnotations. The value MUST conform to RFC 3339.
//
// If succeeded, returns a descriptor of the packed index.
func PackIndex(ctx context.Context, pusher content.Pusher, artifactType string, opts PackIndexOptions) (ocispec.Descriptor, error) {
	return packIndex(ctx, pusher, artifactType, opts)
}

// PackIndexN packs an OCI Image Index like [PackIndex], pushes it to target,
// and tags it with each of the given references. See [PackManifestN] for how
// the references are applied.
//
// If succeeded, returns a descriptor of the packed index.
func PackIndexN(ctx context.Context, target Target, artifactType string, references []string, opts PackIndexOptions) (ocispec.Descriptor, error) {
	return packIndex(ctx, withTags(target, references), artifactType, opts)
}

func packIndex(ctx context.Context, pusher content.Pusher, artifactType string, opts PackIndexOptions) (ocispec.Descriptor, error) {
	if opts.DigestAlgorithm == "" {
		opts.DigestAlgorithm = digest.Canonical
	}
	if !descriptor.IsSupportedAlgorithm(opts.DigestAlgorithm) || !opts.DigestAlgorithm.Available() {
		return ocispec.Descriptor{}, fmt.Errorf("digest algorithm %q: %w", opts.DigestAlgorithm, errdef.ErrUnsupported)
	}

	if artifactType != "" {
		if err := validateMediaType(artifactType); err != nil {
			return ocispec.Descriptor{}, fmt.Errorf("invalid artifactType format: %w", err)
		}
	}

	annotations, err := ensureAnnotationCreated(opts.IndexAnnotations, ocispec.AnnotationCreated)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	if opts.Manifests == nil {
		opts.Manifests = []ocispec.Descriptor{}
	}

	index := ocispec.Index{
		Versioned: specs.Versioned{
			SchemaVersion: 2,
		},
		MediaType:    ocispec.MediaTypeImageIndex,
		Manifests:    opts.Manifests,
		Subject:      opts.Subject,
		ArtifactType: artifactType,
		Annotations:  annotations,
	}

	return pushManifest(ctx, pusher, index, index.MediaType, index.ArtifactType, index.Annotations, opts.DigestAlgorithm)
}

// PackOptions contains optional parameters for [Pack].
//
// Deprecated: This type is deprecated and not recommended for future use.
// Use [PackManifestOptions] instead.
type PackOptions struct {
	// Subject is the subject of the manifest.
	Subject *ocispec.Descriptor

	// ManifestAnnotations is the annotation map of the manifest.
	ManifestAnnotations map[string]string

	// PackImageManifest controls whether to pack an OCI Image Manifest or not.
	//   - If true, pack an OCI Image Manifest.
	//   - If false, pack an OCI Artifact Manifest (deprecated).
	//
	// Default value: false.
	PackImageManifest bool

	// ConfigDescriptor is a pointer to the descriptor of the config blob.
	// If not nil, the caller is responsible for pushing the config blob to
	// pusher and populating Digest and Size. artifactType will be implied by
	// the mediaType of the specified ConfigDescriptor, and ConfigAnnotations
	// will be ignored.
	// This option is valid only when PackImageManifest is true.
	ConfigDescriptor *ocispec.Descriptor

	// ConfigAnnotations is the annotation map of the config descriptor.
	// This option is valid only when PackImageManifest is true
	// and ConfigDescriptor is nil.
	ConfigAnnotations map[string]string
}

// Pack packs the given blobs, generates a manifest for the pack,
// and pushes it to a content storage.
//
// When opts.PackImageManifest is true, artifactType will be used as the
// the config descriptor mediaType of the image manifest.
//
// If succeeded, returns a descriptor of the manifest.
//
// Deprecated: This method is deprecated and not recommended for future use.
// Use [PackManifest] instead.
func Pack(ctx context.Context, pusher content.Pusher, artifactType string, blobs []ocispec.Descriptor, opts PackOptions) (ocispec.Descriptor, error) {
	if opts.PackImageManifest {
		return packManifestV1_1_RC2(ctx, pusher, artifactType, blobs, opts)
	}
	return packArtifact(ctx, pusher, artifactType, blobs, opts)
}

// packArtifact packs an Artifact manifest as defined in image-spec v1.1.0-rc2.
// Reference: https://github.com/opencontainers/image-spec/blob/v1.1.0-rc2/artifact.md
func packArtifact(ctx context.Context, pusher content.Pusher, artifactType string, blobs []ocispec.Descriptor, opts PackOptions) (ocispec.Descriptor, error) {
	if artifactType == "" {
		artifactType = MediaTypeUnknownArtifact
	}

	annotations, err := ensureAnnotationCreated(opts.ManifestAnnotations, spec.AnnotationArtifactCreated)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	manifest := spec.Artifact{
		MediaType:    spec.MediaTypeArtifactManifest,
		ArtifactType: artifactType,
		Blobs:        blobs,
		Subject:      opts.Subject,
		Annotations:  annotations,
	}
	return pushManifest(ctx, pusher, manifest, manifest.MediaType, manifest.ArtifactType, manifest.Annotations, digest.Canonical)
}

// packManifestV1_0 packs an image manifest defined in image-spec v1.0.2.
// Reference: https://github.com/opencontainers/image-spec/blob/v1.0.2/manifest.md
func packManifestV1_0(ctx context.Context, pusher content.Pusher, artifactType string, opts PackManifestOptions) (ocispec.Descriptor, error) {
	if opts.Subject != nil {
		return ocispec.Descriptor{}, fmt.Errorf("subject is not supported for manifest version %v: %w", PackManifestVersion1_0, errdef.ErrUnsupported)
	}

	// prepare config
	var configDesc ocispec.Descriptor
	if opts.ConfigDescriptor != nil {
		if err := validateConfigDescriptor(*opts.ConfigDescriptor); err != nil {
			return ocispec.Descriptor{}, err
		}
		configDesc = *opts.ConfigDescriptor
	} else {
		if artifactType == "" {
			artifactType = MediaTypeUnknownConfig
		} else if err := validateMediaType(artifactType); err != nil {
			return ocispec.Descriptor{}, fmt.Errorf("invalid artifactType format: %w", err)
		}
		var err error
		configDesc, err = pushCustomEmptyConfig(ctx, pusher, artifactType, opts.ConfigAnnotations, opts.DigestAlgorithm)
		if err != nil {
			return ocispec.Descriptor{}, err
		}
	}

	annotations, err := ensureAnnotationCreated(opts.ManifestAnnotations, ocispec.AnnotationCreated)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	if opts.Layers == nil {
		opts.Layers = []ocispec.Descriptor{} // make it an empty array to prevent potential server-side bugs
	}
	manifest := ocispec.Manifest{
		Versioned: specs.Versioned{
			SchemaVersion: 2, // historical value. does not pertain to OCI or docker version
		},
		Config:      configDesc,
		MediaType:   ocispec.MediaTypeImageManifest,
		Layers:      opts.Layers,
		Annotations: annotations,
	}
	return pushManifest(ctx, pusher, manifest, manifest.MediaType, manifest.Config.MediaType, manifest.Annotations, opts.DigestAlgorithm)
}

// packManifestV1_1_RC2 packs an image manifest as defined in image-spec
// v1.1.0-rc2.
// Reference: https://github.com/opencontainers/image-spec/blob/v1.1.0-rc2/manifest.md
func packManifestV1_1_RC2(ctx context.Context, pusher content.Pusher, configMediaType string, layers []ocispec.Descriptor, opts PackOptions) (ocispec.Descriptor, error) {
	if configMediaType == "" {
		configMediaType = MediaTypeUnknownConfig
	}

	// prepare config
	var configDesc ocispec.Descriptor
	if opts.ConfigDescriptor != nil {
		if err := validateConfigDescriptor(*opts.ConfigDescriptor); err != nil {
			return ocispec.Descriptor{}, err
		}
		configDesc = *opts.ConfigDescriptor
	} else {
		var err error
		configDesc, err = pushCustomEmptyConfig(ctx, pusher, configMediaType, opts.ConfigAnnotations, digest.Canonical)
		if err != nil {
			return ocispec.Descriptor{}, err
		}
	}

	annotations, err := ensureAnnotationCreated(opts.ManifestAnnotations, ocispec.AnnotationCreated)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	if layers == nil {
		layers = []ocispec.Descriptor{} // make it an empty array to prevent potential server-side bugs
	}
	manifest := ocispec.Manifest{
		Versioned: specs.Versioned{
			SchemaVersion: 2, // historical value. does not pertain to OCI or docker version
		},
		Config:      configDesc,
		MediaType:   ocispec.MediaTypeImageManifest,
		Layers:      layers,
		Subject:     opts.Subject,
		Annotations: annotations,
	}
	return pushManifest(ctx, pusher, manifest, manifest.MediaType, manifest.Config.MediaType, manifest.Annotations, digest.Canonical)
}

// packManifestV1_1 packs an image manifest defined in image-spec v1.1.1.
// Reference: https://github.com/opencontainers/image-spec/blob/v1.1.1/manifest.md#guidelines-for-artifact-usage
func packManifestV1_1(ctx context.Context, pusher content.Pusher, artifactType string, opts PackManifestOptions) (ocispec.Descriptor, error) {
	if artifactType == "" && (opts.ConfigDescriptor == nil || opts.ConfigDescriptor.MediaType == ocispec.MediaTypeEmptyJSON) {
		// artifactType MUST be set when config.mediaType is set to the empty value
		return ocispec.Descriptor{}, ErrMissingArtifactType
	}
	if artifactType != "" {
		if err := validateMediaType(artifactType); err != nil {
			return ocispec.Descriptor{}, fmt.Errorf("invalid artifactType format: %w", err)
		}
	}

	// prepare config
	var emptyBlobExists bool
	var configDesc ocispec.Descriptor
	if opts.ConfigDescriptor != nil {
		if err := validateConfigDescriptor(*opts.ConfigDescriptor); err != nil {
			return ocispec.Descriptor{}, err
		}
		configDesc = *opts.ConfigDescriptor
	} else {
		// use the empty descriptor for config
		configDesc = emptyJSONDescriptor(opts.DigestAlgorithm)
		configDesc.Annotations = opts.ConfigAnnotations
		configBytes := configDesc.Data
		// push config
		if err := pushIfNotExist(ctx, pusher, configDesc, configBytes); err != nil {
			return ocispec.Descriptor{}, fmt.Errorf("failed to push config: %w", err)
		}
		emptyBlobExists = true
	}

	annotations, err := ensureAnnotationCreated(opts.ManifestAnnotations, ocispec.AnnotationCreated)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	if len(opts.Layers) == 0 {
		// use the empty descriptor as the single layer
		layerDesc := emptyJSONDescriptor(opts.DigestAlgorithm)
		layerData := layerDesc.Data
		if !emptyBlobExists {
			if err := pushIfNotExist(ctx, pusher, layerDesc, layerData); err != nil {
				return ocispec.Descriptor{}, fmt.Errorf("failed to push layer: %w", err)
			}
		}
		opts.Layers = []ocispec.Descriptor{layerDesc}
	}

	manifest := ocispec.Manifest{
		Versioned: specs.Versioned{
			SchemaVersion: 2, // historical value. does not pertain to OCI or docker version
		},
		Config:       configDesc,
		MediaType:    ocispec.MediaTypeImageManifest,
		Layers:       opts.Layers,
		Subject:      opts.Subject,
		ArtifactType: artifactType,
		Annotations:  annotations,
	}
	return pushManifest(ctx, pusher, manifest, manifest.MediaType, manifest.ArtifactType, manifest.Annotations, opts.DigestAlgorithm)
}

// emptyJSONDescriptor returns the descriptor of the empty JSON blob with its
// digest computed using alg.
func emptyJSONDescriptor(alg digest.Algorithm) ocispec.Descriptor {
	desc := ocispec.DescriptorEmptyJSON
	desc.Digest = alg.FromBytes(desc.Data)
	return desc
}

// pushIfNotExist pushes data described by desc if it does not exist in the
// target.
func pushIfNotExist(ctx context.Context, pusher content.Pusher, desc ocispec.Descriptor, data []byte) error {
	if ros, ok := pusher.(content.ReadOnlyStorage); ok {
		exists, err := ros.Exists(ctx, desc)
		if err != nil {
			return fmt.Errorf("failed to check existence: %s: %s: %w", desc.Digest.String(), desc.MediaType, err)
		}
		if exists {
			return nil
		}
	}

	if err := pusher.Push(ctx, desc, bytes.NewReader(data)); err != nil && !errors.Is(err, errdef.ErrAlreadyExists) {
		return fmt.Errorf("failed to push: %s: %s: %w", desc.Digest.String(), desc.MediaType, err)
	}
	return nil
}

// pushManifest marshals manifest into JSON bytes and pushes it.
func pushManifest(ctx context.Context, pusher content.Pusher, manifest any, mediaType string, artifactType string, annotations map[string]string, alg digest.Algorithm) (ocispec.Descriptor, error) {
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("failed to marshal manifest: %w", err)
	}
	manifestDesc, err := content.NewDescriptorFromBytesWithAlgorithm(mediaType, manifestJSON, alg)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("failed to create manifest descriptor: %w", err)
	}
	// populate ArtifactType and Annotations of the manifest into manifestDesc
	manifestDesc.ArtifactType = artifactType
	manifestDesc.Annotations = annotations
	if tp, ok := pusher.(*taggingPusher); ok && len(tp.references) > 0 {
		// push the manifest and tag it
		if err := tagBytesN(ctx, tp.target, manifestDesc, manifestJSON, tp.references, TagBytesNOptions{Concurrency: defaultTagConcurrency}); err != nil {
			return ocispec.Descriptor{}, fmt.Errorf("failed to push manifest: %w", err)
		}
		return manifestDesc, nil
	}
	// push manifest
	if err := pusher.Push(ctx, manifestDesc, bytes.NewReader(manifestJSON)); err != nil && !errors.Is(err, errdef.ErrAlreadyExists) {
		return ocispec.Descriptor{}, fmt.Errorf("failed to push manifest: %w", err)
	}
	return manifestDesc, nil
}

// pushCustomEmptyConfig generates and pushes an empty config blob.
func pushCustomEmptyConfig(ctx context.Context, pusher content.Pusher, mediaType string, annotations map[string]string, alg digest.Algorithm) (ocispec.Descriptor, error) {
	// Use an empty JSON object here, because some registries may not accept
	// empty config blob.
	// As of September 2022, GAR is known to return 400 on empty blob upload.
	// See https://github.com/oras-project/oras-go/issues/294 for details.
	configBytes := []byte("{}")
	configDesc, err := content.NewDescriptorFromBytesWithAlgorithm(mediaType, configBytes, alg)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	configDesc.Annotations = annotations
	// push config
	if err := pushIfNotExist(ctx, pusher, configDesc, configBytes); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("failed to push config: %w", err)
	}
	return configDesc, nil
}

// ensureAnnotationCreated ensures that annotationCreatedKey is in annotations,
// and that its value conforms to RFC 3339. Otherwise returns a new annotation
// map with annotationCreatedKey created.
func ensureAnnotationCreated(annotations map[string]string, annotationCreatedKey string) (map[string]string, error) {
	if createdTime, ok := annotations[annotationCreatedKey]; ok {
		// if annotationCreatedKey is provided, validate its format
		if _, err := time.Parse(time.RFC3339, createdTime); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidDateTimeFormat, err)
		}
		return annotations, nil
	}

	// copy the original annotation map
	copied := make(map[string]string, len(annotations)+1)
	maps.Copy(copied, annotations)

	// set creation time in RFC 3339 format
	// reference: https://github.com/opencontainers/image-spec/blob/v1.1.0-rc2/annotations.md#pre-defined-annotation-keys
	now := time.Now().UTC()
	copied[annotationCreatedKey] = now.Format(time.RFC3339)
	return copied, nil
}

// validateMediaType validates the format of mediaType.
func validateMediaType(mediaType string) error {
	if !mediaTypeRegexp.MatchString(mediaType) {
		return fmt.Errorf("%s: %w", mediaType, errdef.ErrInvalidMediaType)
	}
	return nil
}

// validateConfigDescriptor validates a caller-supplied config descriptor.
func validateConfigDescriptor(desc ocispec.Descriptor) error {
	if err := validateMediaType(desc.MediaType); err != nil {
		return fmt.Errorf("invalid config mediaType format: %w", err)
	}
	if err := desc.Digest.Validate(); err != nil {
		return fmt.Errorf("invalid config digest %q: %v: %w", desc.Digest, err, errdef.ErrInvalidDigest)
	}
	return nil
}

// taggingPusher is a [content.Pusher] that also carries the references the
// packed manifest or index is to be tagged with. Everything except the final
// manifest is pushed to the target as usual.
type taggingPusher struct {
	Target
	target     Target
	references []string
}

func withTags(target Target, references []string) *taggingPusher {
	seen := make(map[string]struct{}, len(references))
	unique := make([]string, 0, len(references))
	for _, ref := range references {
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}
		unique = append(unique, ref)
	}
	return &taggingPusher{Target: target, target: target, references: unique}
}
