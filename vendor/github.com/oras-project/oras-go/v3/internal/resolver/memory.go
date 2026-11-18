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

package resolver

import (
	"context"
	"fmt"
	"iter"
	"maps"
	"sync"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/oras-project/oras-go/v3/errdef"
	"github.com/oras-project/oras-go/v3/internal/container/set"
)

// Change describes the current state of a resolver reference, as reported by
// [Memory.Snapshot] and [Memory.Flush].
type Change struct {
	// Ref is the reference whose state changed.
	Ref string
	// Desc is the descriptor associated with Ref when Exists is true.
	Desc ocispec.Descriptor
	// Exists reports whether Ref currently exists in the resolver.
	Exists bool
	// Tagged reports whether the descriptor's digest is referenced by a tag.
	Tagged bool
}

// Memory is a memory based resolver.
type Memory struct {
	lock  sync.RWMutex
	index map[string]ocispec.Descriptor
	tags  map[digest.Digest]set.Set[string]
	// changed holds the references that [Memory.Flush] has yet to report. It is
	// nil until [Memory.Snapshot] starts the tracking, so a Memory that is
	// never snapshotted records nothing.
	changed set.Set[string]
}

// NewMemory creates a new Memory resolver.
func NewMemory() *Memory {
	return &Memory{
		index: make(map[string]ocispec.Descriptor),
		tags:  make(map[digest.Digest]set.Set[string]),
	}
}

// Resolve resolves a reference to a descriptor.
func (m *Memory) Resolve(_ context.Context, reference string) (ocispec.Descriptor, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	desc, ok := m.index[reference]
	if !ok {
		return ocispec.Descriptor{}, fmt.Errorf("%s: %w", reference, errdef.ErrNotFound)
	}
	return desc, nil
}

// Tag tags a descriptor with a reference string.
func (m *Memory) Tag(_ context.Context, desc ocispec.Descriptor, reference string) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	// Drop reference from the tag set of the descriptor it previously
	// pointed to, so that the reverse index stays in sync with index.
	if oldDesc, ok := m.index[reference]; ok {
		oldTagSet := m.tags[oldDesc.Digest]
		oldTagSet.Delete(reference)
		if len(oldTagSet) == 0 {
			delete(m.tags, oldDesc.Digest)
		}
		m.markChanged(oldDesc.Digest.String())
	}
	m.markChanged(reference, desc.Digest.String())
	m.index[reference] = desc
	tagSet, ok := m.tags[desc.Digest]
	if !ok {
		tagSet = set.New[string]()
		m.tags[desc.Digest] = tagSet
	}
	tagSet.Add(reference)
	return nil
}

// Untag removes a reference from index map.
func (m *Memory) Untag(reference string) {
	m.lock.Lock()
	defer m.lock.Unlock()

	desc, ok := m.index[reference]
	if !ok {
		return
	}
	delete(m.index, reference)
	tagSet := m.tags[desc.Digest]
	tagSet.Delete(reference)
	if len(tagSet) == 0 {
		delete(m.tags, desc.Digest)
	}
	m.markChanged(reference, desc.Digest.String())
}

// Map dumps the memory into a built-in map structure.
// Like other operations, calling Map() is go-routine safe.
func (m *Memory) Map() map[string]ocispec.Descriptor {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return maps.Clone(m.index)
}

// All returns an iterator over the references and their descriptors, without
// copying the memory. The read lock is held for the whole iteration, so the loop
// body must not call any method of the Memory, or it may deadlock.
func (m *Memory) All() iter.Seq2[string, ocispec.Descriptor] {
	return func(yield func(string, ocispec.Descriptor) bool) {
		m.lock.RLock()
		defer m.lock.RUnlock()

		for ref, desc := range m.index {
			if !yield(ref, desc) {
				return
			}
		}
	}
}

// TagSet returns the set of tags of the descriptor.
func (m *Memory) TagSet(desc ocispec.Descriptor) set.Set[string] {
	m.lock.RLock()
	defer m.lock.RUnlock()

	tagSet := m.tags[desc.Digest]
	return maps.Clone(tagSet)
}

// Tagged reports whether the descriptor is tagged, that is whether its digest
// is referenced by anything other than the digest itself.
func (m *Memory) Tagged(desc ocispec.Descriptor) bool {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return m.tagged(desc)
}

// tagged implements [Memory.Tagged]. The caller must hold the lock.
func (m *Memory) tagged(desc ocispec.Descriptor) bool {
	tagSet := m.tags[desc.Digest]
	if tagSet.Contains(desc.Digest.String()) {
		return len(tagSet) > 1
	}
	return len(tagSet) > 0
}

// Snapshot delivers every existing reference to fn, and starts recording the
// changes that [Memory.Flush] reports, discarding the ones recorded before.
//
// It gives a consumer the complete state to start from, whether or not the
// Memory was snapshotted or flushed before, so the consumer need not know the
// history of the Memory. The caller tracks whether it has taken a snapshot.
//
// Snapshot invokes fn while holding the resolver's write lock. The callback
// must not call methods on the same resolver, as doing so may deadlock.
// Snapshot and Flush are intended for a single consumer.
func (m *Memory) Snapshot(fn func(Change)) {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.changed = set.New[string]()
	for ref := range m.index {
		m.report(ref, fn)
	}
}

// Flush delivers the references changed since the previous [Memory.Snapshot] or
// Flush to fn, with each reference represented by its latest state. Changes are
// coalesced by reference.
//
// Flush reports nothing until Snapshot was called, because the changes are not
// recorded before then.
//
// Flush invokes fn while holding the resolver's write lock. The callback must
// not call methods on the same resolver, as doing so may deadlock.
func (m *Memory) Flush(fn func(Change)) {
	m.lock.Lock()
	defer m.lock.Unlock()

	for ref := range m.changed {
		m.report(ref, fn)
	}
	clear(m.changed)
}

// report delivers the current state of ref to fn. The caller must hold the
// write lock.
func (m *Memory) report(ref string, fn func(Change)) {
	desc, ok := m.index[ref]
	fn(Change{Ref: ref, Desc: desc, Exists: ok, Tagged: ok && m.tagged(desc)})
}

// markChanged records that the references changed, if the changes are tracked.
// The caller must hold the write lock.
func (m *Memory) markChanged(refs ...string) {
	if m.changed == nil {
		return
	}
	for _, ref := range refs {
		m.changed.Add(ref)
	}
}
