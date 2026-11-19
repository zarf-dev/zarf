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

package graph

import (
	"context"
	"errors"
	"sync"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/oras-project/oras-go/v3/content"
	"github.com/oras-project/oras-go/v3/errdef"
	"github.com/oras-project/oras-go/v3/internal/container/set"
	"github.com/oras-project/oras-go/v3/internal/status"
	"github.com/oras-project/oras-go/v3/internal/syncutil"
	"golang.org/x/sync/semaphore"
)

// defaultIndexConcurrency is the default number of nodes that IndexAll will
// index concurrently. Unlike CopyGraph's defaultConcurrency, this bounds
// local, disk-bound work (opening and parsing files) rather than network
// round trips, so a higher value is used. It exists to keep the number of
// concurrently open file descriptors and in-flight goroutines bounded on
// stores with large indexes, not to saturate any particular resource.
const defaultIndexConcurrency int64 = 64

// Memory is a memory based PredecessorFinder.
type Memory struct {
	// nodes has the following properties and behaviors:
	//  1. a node exists in Memory.nodes if and only if it exists in the memory
	//  2. Memory.nodes saves the ocispec.Descriptor indexed by digest, which are used by
	//    the other fields.
	nodes map[digest.Digest]ocispec.Descriptor

	// predecessors has the following properties and behaviors:
	//  1. a node exists in Memory.predecessors if it has at least one predecessor
	//    in the memory, regardless of whether or not the node itself exists in
	//    the memory.
	//  2. a node does not exist in Memory.predecessors, if it doesn't have any predecessors
	//    in the memory.
	predecessors map[digest.Digest]set.Set[digest.Digest]

	// successors has the following properties and behaviors:
	//  1. a node exists in Memory.successors if and only if it exists in the memory.
	//  2. a node's entry in Memory.successors is always consistent with the actual
	//    content of the node, regardless of whether or not each successor exists
	//    in the memory.
	successors map[digest.Digest]set.Set[digest.Digest]

	lock sync.RWMutex

	// indexLimiter bounds the number of nodes IndexAll indexes concurrently,
	// so opening a store with a large index does not fan out an unbounded
	// number of goroutines and file descriptors.
	indexLimiter *semaphore.Weighted
}

// NewMemory creates a new memory PredecessorFinder.
func NewMemory() *Memory {
	return &Memory{
		nodes:        make(map[digest.Digest]ocispec.Descriptor),
		predecessors: make(map[digest.Digest]set.Set[digest.Digest]),
		successors:   make(map[digest.Digest]set.Set[digest.Digest]),
		indexLimiter: semaphore.NewWeighted(defaultIndexConcurrency),
	}
}

// Index indexes predecessors for each direct successor of the given node.
func (m *Memory) Index(ctx context.Context, fetcher content.Fetcher, node ocispec.Descriptor) error {
	_, err := m.index(ctx, fetcher, node)
	return err
}

// IndexAll indexes predecessors for all successors of the given nodes.
func (m *Memory) IndexAll(ctx context.Context, fetcher content.Fetcher, nodes ...ocispec.Descriptor) error {
	// track content status
	tracker := status.NewTracker()
	var fn syncutil.GoFunc[ocispec.Descriptor]
	fn = func(ctx context.Context, region *syncutil.LimitedRegion, desc ocispec.Descriptor) error {
		// skip the node if other go routine is working on it
		_, committed := tracker.TryCommit(desc)
		if !committed {
			return nil
		}
		successors, err := m.index(ctx, fetcher, desc)
		if err != nil {
			if errors.Is(err, errdef.ErrNotFound) {
				// skip the node if it does not exist
				return nil
			}
			return err
		}
		if len(successors) > 0 {
			// release this node's slot before waiting on its successors,
			// which acquire from the same limiter: holding it here would
			// deadlock any chain deeper than defaultIndexConcurrency, since
			// the parent would sit on a slot its own descendants need.
			region.End()
			// traverse and index successors, bounded by indexLimiter so a
			// large fan-out does not spawn an unbounded number of goroutines
			return syncutil.Go(ctx, m.indexLimiter, fn, successors...)
		}
		return nil
	}
	return syncutil.Go(ctx, m.indexLimiter, fn, nodes...)
}

// Predecessors returns the nodes directly pointing to the current node.
// Predecessors returns nil without error if the node does not exists in the
// store. Like other operations, calling Predecessors() is go-routine safe.
// However, it does not necessarily correspond to any consistent snapshot of
// the stored contents.
func (m *Memory) Predecessors(_ context.Context, node ocispec.Descriptor) ([]ocispec.Descriptor, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	set, exists := m.predecessors[node.Digest]
	if !exists {
		return nil, nil
	}
	var res []ocispec.Descriptor
	for digest := range set {
		res = append(res, m.nodes[digest])
	}
	return res, nil
}

// Remove removes the node from its predecessors and successors, and returns the
// dangling root nodes caused by the deletion.
func (m *Memory) Remove(node ocispec.Descriptor) []ocispec.Descriptor {
	m.lock.Lock()
	defer m.lock.Unlock()

	var danglings []ocispec.Descriptor
	// remove the node from its successors' predecessor list
	for successorDigest := range m.successors[node.Digest] {
		predecessorEntry := m.predecessors[successorDigest]
		predecessorEntry.Delete(node.Digest)

		// if none of the predecessors of the node still exists, we remove the
		// predecessors entry and return it as a dangling node. Otherwise, we do
		// not remove the entry.
		if len(predecessorEntry) == 0 {
			delete(m.predecessors, successorDigest)
			if _, exists := m.nodes[successorDigest]; exists {
				danglings = append(danglings, m.nodes[successorDigest])
			}
		}
	}
	delete(m.successors, node.Digest)
	delete(m.nodes, node.Digest)
	return danglings
}

// DigestSet returns the set of node digest in memory.
func (m *Memory) DigestSet() set.Set[digest.Digest] {
	m.lock.RLock()
	defer m.lock.RUnlock()

	s := set.New[digest.Digest]()
	for digest := range m.nodes {
		s.Add(digest)
	}
	return s
}

// index indexes predecessors for each direct successor of the given node.
func (m *Memory) index(ctx context.Context, fetcher content.Fetcher, node ocispec.Descriptor) ([]ocispec.Descriptor, error) {
	successors, err := content.Successors(ctx, fetcher, node)
	if err != nil {
		return nil, err
	}
	m.lock.Lock()
	defer m.lock.Unlock()

	// index the node
	m.nodes[node.Digest] = node

	// for each successor, put it into the node's successors list, and
	// put node into the succeesor's predecessors list
	successorSet := set.New[digest.Digest]()
	m.successors[node.Digest] = successorSet
	for _, successor := range successors {
		successorSet.Add(successor.Digest)
		predecessorSet, exists := m.predecessors[successor.Digest]
		if !exists {
			predecessorSet = set.New[digest.Digest]()
			m.predecessors[successor.Digest] = predecessorSet
		}
		predecessorSet.Add(node.Digest)
	}
	return successors, nil
}

// Exists checks if the node exists in the graph
func (m *Memory) Exists(node ocispec.Descriptor) bool {
	m.lock.RLock()
	defer m.lock.RUnlock()

	_, exists := m.nodes[node.Digest]
	return exists
}

// Node returns the descriptor indexed for dgst, if present in the graph.
func (m *Memory) Node(dgst digest.Digest) (ocispec.Descriptor, bool) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	desc, exists := m.nodes[dgst]
	return desc, exists
}
