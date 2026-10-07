// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package archive writes images held in an OCI image layout out as tar
// archives.
package archive

import (
	"archive/tar"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"

	"github.com/distribution/reference"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/oci"
)

// imageNameAnnotation is the annotation containerd and the Docker containerd
// image store read the full image reference from. It is spelled out here
// rather than imported so that writing an archive does not pull containerd
// into the build; src/pkg/images/unpack.go, which reads the archives this
// package writes, declares the same key for the same reason.
const imageNameAnnotation = "io.containerd.image.name"

// dockerManifest is one entry of the legacy Docker manifest.json list that
// `docker load` and `podman load` read. Every path in it is a blob path
// inside the archive.
type dockerManifest struct {
	Config   string
	RepoTags []string
	Layers   []string
}

// Export writes the image manifest identifies to w as a tar archive holding
// an OCI image layout (oci-layout, index.json, and every blob the image
// needs) plus the Docker compatible manifest.json, which is the layout
// `docker save` produces and `docker load` accepts.
//
// ref names the image in both index.json and manifest.json, so it must parse
// as an image reference. Only manifest, its config, and its layers are
// exported: anything else the store happens to hold is left out.
func Export(ctx context.Context, store *oci.Store, manifest ocispec.Descriptor, ref string, w io.Writer) error {
	manifestBytes, err := content.FetchAll(ctx, store, manifest)
	if err != nil {
		return fmt.Errorf("failed to read manifest %s: %w", manifest.Digest, err)
	}
	var mfst ocispec.Manifest
	if err := json.Unmarshal(manifestBytes, &mfst); err != nil {
		return fmt.Errorf("failed to parse manifest %s: %w", manifest.Digest, err)
	}

	repoTag, err := familiarRef(ref)
	if err != nil {
		return err
	}

	blobs := append([]ocispec.Descriptor{manifest, mfst.Config}, mfst.Layers...)

	docker := dockerManifest{Config: blobPath(mfst.Config.Digest)}
	docker.RepoTags = []string{repoTag}
	for _, layer := range mfst.Layers {
		docker.Layers = append(docker.Layers, blobPath(layer.Digest))
	}

	records := []tarRecord{
		bytesRecord(ocispec.ImageLayoutFile, 0444, ocispec.ImageLayout{Version: ocispec.ImageLayoutVersion}),
		bytesRecord(ocispec.ImageIndexFile, 0644, ocispec.Index{
			Versioned: ocispecs.Versioned{SchemaVersion: 2},
			MediaType: ocispec.MediaTypeImageIndex,
			Manifests: []ocispec.Descriptor{withNameAnnotations(manifest, ref)},
		}),
		bytesRecord("manifest.json", 0644, []dockerManifest{docker}),
		directoryRecord(ocispec.ImageBlobsDir + "/"),
	}

	algorithms := map[string]struct{}{}
	for _, desc := range blobs {
		if err := desc.Digest.Validate(); err != nil {
			return err
		}
		algorithms[desc.Digest.Algorithm().String()] = struct{}{}
		records = append(records, blobRecord(store, desc))
	}
	for alg := range algorithms {
		records = append(records, directoryRecord(path.Join(ocispec.ImageBlobsDir, alg)+"/"))
	}

	tw := tar.NewWriter(w)
	if err := writeRecords(ctx, tw, records); err != nil {
		return err
	}
	return tw.Close()
}

// withNameAnnotations returns desc carrying the two annotations that name an
// image inside index.json: the full reference, and the OCI reference name,
// which the spec asks to be the tag alone. desc's own annotation map is never
// written to.
func withNameAnnotations(desc ocispec.Descriptor, ref string) ocispec.Descriptor {
	annotations := make(map[string]string, len(desc.Annotations)+2)
	for k, v := range desc.Annotations {
		annotations[k] = v
	}
	annotations[imageNameAnnotation] = ref
	annotations[ocispec.AnnotationRefName] = ociRefName(ref)
	desc.Annotations = annotations
	return desc
}

// ociRefName reduces ref to the tag the OCI image layout spec wants in
// org.opencontainers.image.ref.name. A reference with no tag - a bare name,
// or one pinned to a digest - has no tag to reduce to and is returned whole,
// since the digest form is itself valid for the annotation.
func ociRefName(ref string) string {
	parsed, err := reference.Parse(ref)
	if err != nil {
		return ref
	}
	if tagged, ok := parsed.(reference.Tagged); ok {
		return tagged.Tag()
	}
	return ref
}

// familiarRef normalizes ref the way the Docker CLI displays it, which is the
// form manifest.json's RepoTags is expected to carry: "nginx:1.29.2" rather
// than "docker.io/library/nginx:1.29.2", with an implied ":latest" made
// explicit.
func familiarRef(ref string) (string, error) {
	named, err := reference.ParseNormalizedNamed(ref)
	if err != nil {
		return "", fmt.Errorf("failed to parse image reference %q: %w", ref, err)
	}
	return reference.FamiliarString(reference.TagNameOnly(named)), nil
}

// blobPath is where the blob dgst identifies lives inside the archive.
func blobPath(dgst digest.Digest) string {
	return path.Join(ocispec.ImageBlobsDir, dgst.Algorithm().String(), dgst.Encoded())
}

// tarRecord is a single entry of the archive. copyTo is nil for entries with
// no payload, such as directories.
type tarRecord struct {
	header *tar.Header
	copyTo func(context.Context, io.Writer) (int64, error)
}

// bytesRecord holds a JSON document written inline.
func bytesRecord(name string, mode int64, v any) tarRecord {
	b, err := json.Marshal(v)
	if err != nil {
		// Every caller marshals a struct of strings, descriptors, and maps of
		// strings, none of which json.Marshal can fail on.
		panic(err)
	}
	return tarRecord{
		header: &tar.Header{
			Name:     name,
			Mode:     mode,
			Size:     int64(len(b)),
			Typeflag: tar.TypeReg,
		},
		copyTo: func(_ context.Context, w io.Writer) (int64, error) {
			n, err := w.Write(b)
			return int64(n), err
		},
	}
}

// directoryRecord holds a directory entry, which extractors that create
// parent directories lazily do not need but `docker load` expects to find.
func directoryRecord(name string) tarRecord {
	return tarRecord{header: &tar.Header{Name: name, Mode: 0755, Typeflag: tar.TypeDir}}
}

// blobRecord holds a blob streamed out of the store, verified against the
// digest it is filed under as it is copied.
func blobRecord(store *oci.Store, desc ocispec.Descriptor) tarRecord {
	return tarRecord{
		header: &tar.Header{
			Name:     blobPath(desc.Digest),
			Mode:     0444,
			Size:     desc.Size,
			Typeflag: tar.TypeReg,
		},
		copyTo: func(ctx context.Context, w io.Writer) (_ int64, err error) {
			rc, err := store.Fetch(ctx, desc)
			if err != nil {
				return 0, fmt.Errorf("failed to read blob %s: %w", desc.Digest, err)
			}
			defer func() { err = errors.Join(err, rc.Close()) }()

			digester := desc.Digest.Algorithm().Digester()
			n, err := io.Copy(io.MultiWriter(w, digester.Hash()), rc)
			if err != nil {
				return 0, fmt.Errorf("failed to write blob %s: %w", desc.Digest, err)
			}
			if got := digester.Digest(); got != desc.Digest {
				return 0, fmt.Errorf("blob %s hashed to %s while being written", desc.Digest, got)
			}
			return n, nil
		},
	}
}

// writeRecords writes records in name order, which keeps an archive of the
// same image byte for byte the same however the records were assembled.
// Records that repeat a name are written once.
func writeRecords(ctx context.Context, tw *tar.Writer, records []tarRecord) error {
	sort.Slice(records, func(i, j int) bool {
		return records[i].header.Name < records[j].header.Name
	})

	var last string
	for _, record := range records {
		if record.header.Name == last {
			continue
		}
		last = record.header.Name

		if err := tw.WriteHeader(record.header); err != nil {
			return err
		}
		if record.copyTo == nil {
			continue
		}
		n, err := record.copyTo(ctx, tw)
		if err != nil {
			return err
		}
		if n != record.header.Size {
			return fmt.Errorf("wrote %d bytes to %s, expected %d", n, record.header.Name, record.header.Size)
		}
	}
	return nil
}
