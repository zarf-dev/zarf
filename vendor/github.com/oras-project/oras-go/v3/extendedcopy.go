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
	"context"
	"encoding/json"
	"errors"
	"regexp"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/oras-project/oras-go/v3/content"
	"github.com/oras-project/oras-go/v3/internal/cas"
	"github.com/oras-project/oras-go/v3/internal/container/set"
	"github.com/oras-project/oras-go/v3/internal/descriptor"
	"github.com/oras-project/oras-go/v3/internal/docker"
	"github.com/oras-project/oras-go/v3/internal/spec"
	"github.com/oras-project/oras-go/v3/internal/status"
	"github.com/oras-project/oras-go/v3/internal/syncutil"
	"github.com/oras-project/oras-go/v3/registry"
	"golang.org/x/sync/semaphore"
)

type findRootsOptions struct {
	Depth            int
	FindPredecessors func(
		context.Context,
		content.ReadOnlyGraphStorage,
		ocispec.Descriptor,
	) ([]ocispec.Descriptor, error)
}

// DefaultExtendedCopyOptions provides the default ExtendedCopyOptions.
// It is the zero value, provided for convenience. Do not modify it: the
// change would apply to every caller in the process that passes it.
var DefaultExtendedCopyOptions ExtendedCopyOptions = ExtendedCopyOptions{
	ExtendedCopyGraphOptions: DefaultExtendedCopyGraphOptions,
}

// ExtendedCopyOptions contains parameters for [oras.ExtendedCopy].
type ExtendedCopyOptions struct {
	ExtendedCopyGraphOptions
}

// DefaultExtendedCopyGraphOptions provides the default ExtendedCopyGraphOptions.
// It is the zero value, provided for convenience. Do not modify it: the
// change would apply to every caller in the process that passes it.
var DefaultExtendedCopyGraphOptions ExtendedCopyGraphOptions = ExtendedCopyGraphOptions{
	CopyGraphOptions: DefaultCopyGraphOptions,
}

// ExtendedCopyGraphOptions contains parameters for [oras.ExtendedCopyGraph]
// and [oras.ExtendedCopyGraphNodes].
type ExtendedCopyGraphOptions struct {
	CopyGraphOptions
	// Depth limits the maximum depth of the directed acyclic graph (DAG) that
	// will be extended-copied.
	// If Depth is no specified, or the specified value is less than or
	// equal to 0, the depth limit will be considered as infinity.
	// When several start nodes are given, the depth is measured from the
	// nearest start node.
	Depth int
	// FindPredecessors finds the predecessors of the current node.
	// It may be called concurrently, so it must be safe for concurrent use.
	// If FindPredecessors is nil, src.Predecessors will be adapted and used.
	FindPredecessors func(ctx context.Context, src content.ReadOnlyGraphStorage, desc ocispec.Descriptor) ([]ocispec.Descriptor, error)
}

// ExtendedCopy copies the directed acyclic graph (DAG) that are reachable from
// the given tagged node from the source GraphTarget to the destination Target.
// In other words, it copies a tagged artifact along with its referrers or
// other predecessor manifests referencing it.
//
// The tagged node (e.g. a tagged manifest of the artifact) is identified by the
// source reference.
// The destination reference will be the same as the source reference if the
// destination reference is left blank.
//
// Returns the descriptor of the tagged node on successful copy.
func ExtendedCopy(ctx context.Context, src ReadOnlyGraphTarget, srcRef string, dst Target, dstRef string, opts ExtendedCopyOptions) (ocispec.Descriptor, error) {
	if src == nil {
		return ocispec.Descriptor{}, newCopyError("ExtendedCopy", CopyErrorOriginSource, ocispec.Descriptor{}, errors.New("nil source target"))
	}
	if dst == nil {
		return ocispec.Descriptor{}, newCopyError("ExtendedCopy", CopyErrorOriginDestination, ocispec.Descriptor{}, errors.New("nil destination target"))
	}
	if dstRef == "" {
		dstRef = srcRef
	}

	node, err := src.Resolve(ctx, srcRef)
	if err != nil {
		return ocispec.Descriptor{}, newCopyError("Resolve", CopyErrorOriginSource, ocispec.Descriptor{}, err)
	}

	if err := ExtendedCopyGraph(ctx, src, dst, node, opts.ExtendedCopyGraphOptions); err != nil {
		return ocispec.Descriptor{}, err
	}

	if err := dst.Tag(ctx, node, dstRef); err != nil {
		return ocispec.Descriptor{}, newCopyError("Tag", CopyErrorOriginDestination, node, err)
	}

	return node, nil
}

// ExtendedCopyGraph copies the directed acyclic graph (DAG) that are reachable
// from the given node from the source GraphStorage to the destination Storage.
// In other words, it copies an artifact along with its referrers or other
// predecessor manifests referencing it.
// The node (e.g. a manifest of the artifact) is identified by a descriptor.
//
// To extended-copy from several nodes in one call, use
// [ExtendedCopyGraphNodes].
func ExtendedCopyGraph(ctx context.Context, src content.ReadOnlyGraphStorage, dst content.Storage, node ocispec.Descriptor, opts ExtendedCopyGraphOptions) error {
	return extendedCopyGraph(ctx, "ExtendedCopyGraph", src, dst, []ocispec.Descriptor{node}, opts)
}

// ExtendedCopyGraphNodes copies the directed acyclic graph (DAG) that are
// reachable from any of the given nodes from the source GraphStorage to the
// destination Storage. It behaves like [ExtendedCopyGraph] started from
// several nodes at once: the nodes share one traversal, one concurrency
// limit, one metadata cache and one content status tracker, so content
// reachable from more than one node is copied only once.
//
// Every start node is at depth 0. If opts.Depth is set, the depth of any other
// node is measured from the nearest start node.
// Duplicate start nodes are visited once, and a start node that is also a
// predecessor of another start node is visited once as well.
//
// It returns an error if nodes is empty.
func ExtendedCopyGraphNodes(ctx context.Context, src content.ReadOnlyGraphStorage, dst content.Storage, nodes []ocispec.Descriptor, opts ExtendedCopyGraphOptions) error {
	return extendedCopyGraph(ctx, "ExtendedCopyGraphNodes", src, dst, nodes, opts)
}

// extendedCopyGraph implements [ExtendedCopyGraph] and
// [ExtendedCopyGraphNodes]. op names the public function in errors.
func extendedCopyGraph(ctx context.Context, op string, src content.ReadOnlyGraphStorage, dst content.Storage, nodes []ocispec.Descriptor, opts ExtendedCopyGraphOptions) error {
	if src == nil {
		return newCopyError(op, CopyErrorOriginSource, ocispec.Descriptor{}, errors.New("nil source target"))
	}
	if dst == nil {
		return newCopyError(op, CopyErrorOriginDestination, ocispec.Descriptor{}, errors.New("nil destination target"))
	}

	// if Concurrency is not set or invalid, use the default concurrency
	if opts.Concurrency <= 0 {
		opts.Concurrency = defaultConcurrency
	}
	limiter := semaphore.NewWeighted(int64(opts.Concurrency))

	roots, err := findRoots(ctx, op, src, nodes, findRootsOptions{
		Depth:            opts.Depth,
		FindPredecessors: opts.FindPredecessors,
	}, limiter)
	if err != nil {
		return err
	}

	// use caching proxy on non-leaf nodes
	if opts.MaxMetadataBytes <= 0 {
		opts.MaxMetadataBytes = defaultCopyMaxMetadataBytes
	}
	proxy := cas.NewProxyWithLimit(src, cas.NewMemory(), opts.MaxMetadataBytes)
	// track content status
	tracker := status.NewTracker()

	// copy the sub-DAGs rooted by the root nodes
	return syncutil.Go(ctx, limiter, func(ctx context.Context, region *syncutil.LimitedRegion, root ocispec.Descriptor) error {
		// As a root can be a predecessor of other roots, release the limit here
		// for dispatching, to avoid dead locks where predecessor roots are
		// handled first and are waiting for its successors to complete.
		region.End()
		if err := copyGraph(ctx, src, dst, root, proxy, limiter, tracker, opts.CopyGraphOptions); err != nil {
			return err
		}
		return region.Start()
	}, roots...)
}

// findRoots finds the root nodes reachable from the given start nodes through
// a single breadth-first search. The predecessors of the nodes at the same
// depth are found concurrently, bounded by limiter.
//
// All start nodes are at depth 0, duplicates are collapsed, and a start node
// that is reachable from another start node is still visited only once.
// It returns an error if nodes is empty.
func findRoots(ctx context.Context, op string, storage content.ReadOnlyGraphStorage, nodes []ocispec.Descriptor, opts findRootsOptions, limiter *semaphore.Weighted) ([]ocispec.Descriptor, error) {
	if len(nodes) == 0 {
		return nil, newCopyError(op, CopyErrorOriginSource, ocispec.Descriptor{}, errors.New("at least one start node is required"))

	}

	visited := set.New[descriptor.Descriptor]()
	rootMap := make(map[descriptor.Descriptor]ocispec.Descriptor)
	addRoot := func(key descriptor.Descriptor, val ocispec.Descriptor) {
		if _, exists := rootMap[key]; !exists {
			rootMap[key] = val
		}
	}

	// if FindPredecessors is not provided, use the default one
	if opts.FindPredecessors == nil {
		opts.FindPredecessors = func(ctx context.Context, src content.ReadOnlyGraphStorage, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
			return src.Predecessors(ctx, desc)
		}
	}

	// Visit nodes level by level so a shared predecessor is processed at its
	// shortest distance from the nearest start node. Seeding visited with
	// every start node keeps duplicates out of the first level and keeps a
	// start node that is a predecessor of another from being queued twice.
	level := make([]ocispec.Descriptor, 0, len(nodes))
	var next []ocispec.Descriptor
	for _, node := range nodes {
		if key := descriptor.FromOCI(node); !visited.Contains(key) {
			visited.Add(key)
			level = append(level, node)
		}
	}

	for depth := 0; len(level) > 0; depth++ {
		// stop finding predecessors if the target depth is reached
		if opts.Depth > 0 && depth == opts.Depth {
			for _, root := range level {
				addRoot(descriptor.FromOCI(root), root)
			}
			break
		}

		// Find the predecessors of every node at this depth concurrently.
		// Each lookup writes only its own slot, so no lock is needed.
		indices := make([]int, len(level))
		for i := range indices {
			indices[i] = i
		}
		results := make([][]ocispec.Descriptor, len(level))
		err := syncutil.Go(ctx, limiter, func(ctx context.Context, _ *syncutil.LimitedRegion, i int) error {
			predecessors, err := opts.FindPredecessors(ctx, storage, level[i])
			if err != nil {
				return newCopyError("FindPredecessors", CopyErrorOriginSource, level[i], err)
			}
			results[i] = predecessors
			return nil
		}, indices...)
		if err != nil {
			// On cancellation syncutil.Go returns the context cause, which
			// drops the CopyError built above.
			var copyErr *CopyError
			if !errors.As(err, &copyErr) {
				return nil, newCopyError("FindPredecessors", CopyErrorOriginSource, ocispec.Descriptor{}, err)
			}
			return nil, err
		}

		// Merge the results in order, so the outcome does not depend on which
		// lookup finished first.
		for i, predecessors := range results {
			// The current node has no predecessor node,
			// which means it is a root node of a sub-DAG.
			if len(predecessors) == 0 {
				addRoot(descriptor.FromOCI(level[i]), level[i])
				continue
			}

			// The current node has predecessor nodes, which means it is NOT a root node.
			// Queue predecessor nodes for the next depth and keep finding.
			for _, predecessor := range predecessors {
				predecessorKey := descriptor.FromOCI(predecessor)
				if !visited.Contains(predecessorKey) {
					visited.Add(predecessorKey)
					next = append(next, predecessor)
				}
			}
		}

		// Move on to the next depth, reusing the backing array of this level.
		// level and next always alternate between two distinct arrays, so the
		// appends above can never overwrite the level entries read above.
		level, next = next, level[:0]
	}

	roots := make([]ocispec.Descriptor, 0, len(rootMap))
	for _, root := range rootMap {
		roots = append(roots, root)
	}
	return roots, nil
}

// FilterAnnotation configures opts.FindPredecessors to filter the predecessors
// whose annotation matches a given regex pattern.
//
// A predecessor is kept if key is in its annotations and the annotation value
// matches regex.
// If regex is nil, predecessors whose annotations contain key will be kept,
// no matter of the annotation value.
//
// For performance consideration, when using both FilterArtifactType and
// FilterAnnotation, it's recommended to call FilterArtifactType first.
func (opts *ExtendedCopyGraphOptions) FilterAnnotation(key string, regex *regexp.Regexp) {
	keep := func(desc ocispec.Descriptor) bool {
		value, ok := desc.Annotations[key]
		return ok && (regex == nil || regex.MatchString(value))
	}

	fp := opts.FindPredecessors
	opts.FindPredecessors = func(ctx context.Context, src content.ReadOnlyGraphStorage, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		var predecessors []ocispec.Descriptor
		var err error
		if fp == nil {
			if rf, ok := src.(registry.ReferrerLister); ok {
				// if src is a ReferrerLister, use Referrers() for possible memory saving
				if err := rf.Referrers(ctx, desc, "", func(referrers []ocispec.Descriptor) error {
					// for each page of the results, filter the referrers
					for _, r := range referrers {
						if keep(r) {
							predecessors = append(predecessors, r)
						}
					}
					return nil
				}); err != nil {
					return nil, err
				}
				return predecessors, nil
			}
			predecessors, err = src.Predecessors(ctx, desc)
		} else {
			predecessors, err = fp(ctx, src, desc)
		}
		if err != nil {
			return nil, err
		}

		// Predecessor descriptors that are not from Referrers API are not
		// guaranteed to include the annotations of the corresponding manifests.
		var kept []ocispec.Descriptor
		for _, p := range predecessors {
			if p.Annotations == nil {
				// If the annotations are not present in the descriptors,
				// fetch it from the manifest content.
				switch p.MediaType {
				case docker.MediaTypeManifest, ocispec.MediaTypeImageManifest,
					docker.MediaTypeManifestList, ocispec.MediaTypeImageIndex,
					spec.MediaTypeArtifactManifest:
					annotations, err := fetchAnnotations(ctx, src, p)
					if err != nil {
						return nil, err
					}
					p.Annotations = annotations
				}
			}
			if keep(p) {
				kept = append(kept, p)
			}
		}
		return kept, nil
	}
}

// fetchAnnotations fetches the annotations of the manifest described by desc.
func fetchAnnotations(ctx context.Context, src content.ReadOnlyGraphStorage, desc ocispec.Descriptor) (map[string]string, error) {
	rc, err := src.Fetch(ctx, desc)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	var manifest struct {
		Annotations map[string]string `json:"annotations"`
	}
	if err := json.NewDecoder(rc).Decode(&manifest); err != nil {
		return nil, err
	}
	if manifest.Annotations == nil {
		// to differentiate with nil
		return make(map[string]string), nil
	}
	return manifest.Annotations, nil
}

// FilterArtifactType configures opts.FindPredecessors to filter the
// predecessors whose artifact type matches a given regex pattern.
//
// A predecessor is kept if its artifact type matches regex.
// If regex is nil, all predecessors will be kept.
//
// For performance consideration, when using both FilterArtifactType and
// FilterAnnotation, it's recommended to call FilterArtifactType first.
func (opts *ExtendedCopyGraphOptions) FilterArtifactType(regex *regexp.Regexp) {
	if regex == nil {
		return
	}
	keep := func(desc ocispec.Descriptor) bool {
		return regex.MatchString(desc.ArtifactType)
	}

	fp := opts.FindPredecessors
	opts.FindPredecessors = func(ctx context.Context, src content.ReadOnlyGraphStorage, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		var predecessors []ocispec.Descriptor
		var err error
		if fp == nil {
			if rf, ok := src.(registry.ReferrerLister); ok {
				// if src is a ReferrerLister, use Referrers() for possible memory saving
				if err := rf.Referrers(ctx, desc, "", func(referrers []ocispec.Descriptor) error {
					// for each page of the results, filter the referrers
					for _, r := range referrers {
						if keep(r) {
							predecessors = append(predecessors, r)
						}
					}
					return nil
				}); err != nil {
					return nil, err
				}
				return predecessors, nil
			}
			predecessors, err = src.Predecessors(ctx, desc)
		} else {
			predecessors, err = fp(ctx, src, desc)
		}
		if err != nil {
			return nil, err
		}

		// predecessor descriptors that are not from Referrers API are not
		// guaranteed to include the artifact type of the corresponding
		// manifests.
		var kept []ocispec.Descriptor
		for _, p := range predecessors {
			if p.ArtifactType == "" {
				// if the artifact type is not present in the descriptors,
				// fetch it from the manifest content.
				switch p.MediaType {
				case spec.MediaTypeArtifactManifest, ocispec.MediaTypeImageManifest, ocispec.MediaTypeImageIndex:
					artifactType, err := fetchArtifactType(ctx, src, p)
					if err != nil {
						return nil, err
					}
					p.ArtifactType = artifactType
				}
			}
			if keep(p) {
				kept = append(kept, p)
			}
		}
		return kept, nil
	}
}

// fetchArtifactType fetches the artifact type of the manifest described by desc.
func fetchArtifactType(ctx context.Context, src content.ReadOnlyGraphStorage, desc ocispec.Descriptor) (string, error) {
	rc, err := src.Fetch(ctx, desc)
	if err != nil {
		return "", err
	}
	defer rc.Close()

	switch desc.MediaType {
	case spec.MediaTypeArtifactManifest:
		var manifest spec.Artifact
		if err := json.NewDecoder(rc).Decode(&manifest); err != nil {
			return "", err
		}
		return manifest.ArtifactType, nil
	case ocispec.MediaTypeImageManifest:
		var manifest ocispec.Manifest
		if err := json.NewDecoder(rc).Decode(&manifest); err != nil {
			return "", err
		}
		if manifest.ArtifactType != "" {
			return manifest.ArtifactType, nil
		}
		return manifest.Config.MediaType, nil
	case ocispec.MediaTypeImageIndex:
		var index ocispec.Index
		if err := json.NewDecoder(rc).Decode(&index); err != nil {
			return "", err
		}
		return index.ArtifactType, nil
	default:
		return "", nil
	}
}
