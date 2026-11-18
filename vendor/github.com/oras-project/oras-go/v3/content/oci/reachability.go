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

package oci

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strings"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/oras-project/oras-go/v3/content"
	"github.com/oras-project/oras-go/v3/errdef"
	"github.com/oras-project/oras-go/v3/internal/container/set"
	"github.com/oras-project/oras-go/v3/internal/manifestutil"
	"github.com/oras-project/oras-go/v3/internal/resolver"
)

var (
	digestAlgorithmPattern = regexp.MustCompile(
		`^[a-z0-9]+([+._-][a-z0-9]+)*$`,
	)
	digestEncodedPattern = regexp.MustCompile(`^[a-f0-9]+$`)
)

// referenceEdge is a descriptor and the path of what references it.
type referenceEdge struct {
	desc ocispec.Descriptor
	from string
}

// verifyReachability records in acc the blobs that index.json references but
// that are absent (Missing), and, if every reference could be followed, the
// verified blobs that are not reachable (Unreferenced). A reference that cannot
// be followed is recorded as Unverifiable, with the reason.
//
// The two come from different traversals on purpose. The graph that
// gcReachable builds is neither "everything referenced" (it silently
// skips manifests that are not found) nor "everything on disk" (it never
// fetches blobs that are not manifests), so Missing is computed here as the
// referenced set minus present, which is what the sweep found on disk.
// Unreferenced uses gcReachable, the rule [Store.GC] uses.
//
// present holds the path of every entry the sweep found in a digest directory,
// and unlisted the algorithm directories that could not be listed, whose
// content is unknown rather than absent.
func verifyReachability(ctx context.Context, fsys fs.FS, index *ocispec.Index, present map[string]struct{}, unlisted map[string]error, acc *verifyAccumulator) error {
	storage := NewStorageFromFS(fsys)

	incomplete, err := walkIndexReferences(ctx, storage, index, present, unlisted, acc)
	if err != nil {
		return err
	}
	if incomplete {
		return nil
	}

	refMap, ok := resolveIndexReferences(ctx, index, present, unlisted, acc)
	if !ok {
		return nil
	}

	return reportUnreferenced(ctx, storage, refMap, acc)
}

// walkIndexReferences verifies the references reachable from index.json.
// It reports whether any reference could not be followed completely.
func walkIndexReferences(ctx context.Context, storage content.Fetcher, index *ocispec.Index, present map[string]struct{}, unlisted map[string]error, acc *verifyAccumulator) (bool, error) {
	incomplete := reportUnlistedDirectories(unlisted, acc)

	queue := make([]referenceEdge, 0, len(index.Manifests))
	for _, desc := range index.Manifests {
		queue = append(queue, referenceEdge{
			desc: desc,
			from: ocispec.ImageIndexFile,
		})
	}

	seen := set.New[digest.Digest]()
	for len(queue) > 0 {
		if err := isContextDone(ctx); err != nil {
			return incomplete, err
		}

		edge := queue[0]
		queue = queue[1:]

		desc := edge.desc
		if seen.Contains(desc.Digest) {
			continue
		}
		seen.Add(desc.Digest)

		p, alg, ok := blobPathOf(desc.Digest)
		if !ok {
			acc.addUnverifiable(
				edge.from,
				fmt.Errorf("invalid digest %q: %w", desc.Digest, errdef.ErrInvalidDigest),
			)
			incomplete = true
			continue
		}

		if _, unknown := unlisted[alg]; unknown {
			continue // already reported by the sweep
		}

		if _, exists := present[p]; !exists {
			acc.addMissing(p, fmt.Errorf("referenced by %s: %w", edge.from, errdef.ErrNotFound))
			continue
		}

		successors, err := verifiySuccessors(ctx, storage, desc, present)
		if err != nil {
			if errors.Is(err, context.Canceled) ||
				errors.Is(err, context.DeadlineExceeded) {
				return incomplete, err
			}

			acc.addUnverifiable(p, err)
			incomplete = true
			continue
		}

		for _, successor := range successors {
			queue = append(queue, referenceEdge{
				desc: successor,
				from: p,
			})
		}
	}

	return incomplete, nil
}

// reportUnlistedDirectories records blob directories that could not be listed.
func reportUnlistedDirectories(unlisted map[string]error, acc *verifyAccumulator) bool {
	for alg, err := range unlisted {
		acc.addUnverifiable(
			path.Join(ocispec.ImageBlobsDir, alg),
			fmt.Errorf("reachability unknown: %w", err),
		)
	}
	return len(unlisted) > 0
}

// verifiySuccessors returns a manifest's outgoing references, excluding its
// subject when that subject is absent from the layout.
func verifiySuccessors(ctx context.Context, storage content.Fetcher, desc ocispec.Descriptor, present map[string]struct{}) ([]ocispec.Descriptor, error) {
	successors, err := content.Successors(ctx, storage, desc)
	if err != nil {
		return nil, err
	}

	subject, err := manifestutil.Subject(ctx, storage, desc)
	if err != nil {
		return nil, err
	}

	if subject != nil &&
		len(successors) > 0 &&
		successors[0].Digest == subject.Digest {
		p, _, _ := blobPathOf(subject.Digest)
		if _, exists := present[p]; !exists {
			successors = successors[1:]
		}
	}

	return successors, nil
}

// resolveIndexReferences resolves index references using the same rules as GC.
// Missing untagged entries are excluded because GC cannot retain them.
func resolveIndexReferences(ctx context.Context, index *ocispec.Index, present map[string]struct{}, unlisted map[string]error, acc *verifyAccumulator) (map[string]ocispec.Descriptor, bool) {
	tagResolver := resolver.NewMemory()
	if _, err := tagIndex(ctx, index, tagResolver); err != nil {
		acc.addUnverifiable(ocispec.ImageIndexFile, err)
		return nil, false
	}

	refMap := tagResolver.Map()
	for ref, desc := range refMap {
		if ref != desc.Digest.String() {
			continue
		}

		p, alg, ok := blobPathOf(desc.Digest)
		if !ok {
			continue
		}
		if _, unknown := unlisted[alg]; unknown {
			continue
		}
		if _, exists := present[p]; !exists {
			delete(refMap, ref)
		}
	}
	return refMap, true
}

// reportUnreferenced reports verified blobs that are not reachable from the
// references retained by GC.
func reportUnreferenced(ctx context.Context, storage content.Fetcher, refMap map[string]ocispec.Descriptor, acc *verifyAccumulator) error {
	_, graph, err := gcReachable(ctx, storage, refMap)
	if err != nil {
		if errors.Is(err, context.Canceled) ||
			errors.Is(err, context.DeadlineExceeded) {
			return err
		}

		acc.addUnverifiable(
			ocispec.ImageIndexFile,
			fmt.Errorf("compute reachability: %w", err),
		)
		return nil
	}

	reachable := graph.DigestSet()
	for _, entry := range acc.report().Verified {
		dgst := digest.NewDigestFromEncoded(
			digest.Algorithm(path.Base(path.Dir(entry.Path))),
			path.Base(entry.Path),
		)
		if !reachable.Contains(dgst) {
			acc.addUnreferenced(entry.Path)
		}
	}

	return nil
}

// blobPathOf returns the path in a layout of the blob that dgst names, and its
// algorithm directory, without requiring that the algorithm is supported.
// It reports false for a malformed digest.
func blobPathOf(dgst digest.Digest) (p, alg string, ok bool) {
	alg, encoded, found := strings.Cut(string(dgst), ":")
	if !found ||
		!digestAlgorithmPattern.MatchString(alg) ||
		!digestEncodedPattern.MatchString(encoded) {
		return "", "", false
	}

	algorithm := digest.Algorithm(alg)
	if algorithm.Available() && algorithm.Validate(encoded) != nil {
		return "", "", false
	}

	return path.Join(ocispec.ImageBlobsDir, alg, encoded), alg, true
}
