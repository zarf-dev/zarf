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
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/oras-project/oras-go/v3/internal/resolver"
)

// manifestCollection is the `manifests` field of `index.json`, kept in step with the
// references of a [resolver.Memory].
//
// Each reference of the resolver owns at most one entry:
//   - a tag owns an entry that carries the tag in its
//     "org.opencontainers.image.ref.name" annotation;
//   - a digest reference owns a plain entry, unless its digest is also tagged,
//     in which case the tagged entries already describe it.
//
// Entries are added, replaced and removed one reference at a time, so syncing
// after a few changes costs a few operations rather than a rebuild. The order
// of the entries is not specified.
//
// A manifestCollection is not safe for concurrent use.
type manifestCollection struct {
	// resolver is the resolver the list mirrors.
	resolver *resolver.Memory
	// manifests are the entries, in the form that is written to `index.json`.
	manifests []ocispec.Descriptor
	// refs[i] is the reference that owns manifests[i].
	refs []string
	// refIndexes maps a reference to the position of its entry in manifests.
	refIndexes map[string]int
	// primed reports whether the list was filled from a snapshot of the
	// resolver, after which only changes are applied. It is tracked here, not
	// inferred from the state of the resolver, so a list created for a resolver
	// that was flushed before, by another list, is still filled completely.
	primed bool
}

// newManifestCollection returns an empty list that mirrors src, whatever the
// state of src. It is filled by the first call of sync.
func newManifestCollection(src *resolver.Memory) *manifestCollection {
	return &manifestCollection{
		resolver: src,
		// not nil, so that an empty list is written as `[]` rather than `null`
		manifests:  []ocispec.Descriptor{},
		refIndexes: make(map[string]int),
	}
}

// sync brings the collection in step with the resolver. The first call takes a
// snapshot of all references of the resolver; later calls apply only the
// changes since the previous call. Changes are coalesced by reference, so each
// reference is reconciled against its latest resolver state rather than
// replaying intermediate operations.
func (m *manifestCollection) sync() {
	if !m.primed {
		m.resolver.Snapshot(m.apply)
		m.primed = true
		return
	}
	m.resolver.Flush(m.apply)
}

// apply reconciles a single reference with its current resolver state.
// It adds, replaces, or removes the reference's manifest entry as needed.
func (m *manifestCollection) apply(c resolver.Change) {
	entry, ok := manifestEntry(c)
	i, exists := m.refIndexes[c.Ref]
	switch {
	case ok && exists:
		m.manifests[i] = entry
	case ok:
		m.refIndexes[c.Ref] = len(m.manifests)
		m.manifests = append(m.manifests, entry)
		m.refs = append(m.refs, c.Ref)
	case exists:
		m.remove(i)
	}
}

// remove removes the entry at position i in constant time.
// The last entry is moved into the removed position when necessary, so
// manifest ordering is not preserved.
func (m *manifestCollection) remove(i int) {
	last := len(m.manifests) - 1
	delete(m.refIndexes, m.refs[i])
	if i != last {
		m.manifests[i] = m.manifests[last]
		m.refs[i] = m.refs[last]
		m.refIndexes[m.refs[i]] = i
	}
	// zero the vacated slot, so that it does not retain the removed descriptor
	m.manifests[last] = ocispec.Descriptor{}
	m.manifests = m.manifests[:last]
	m.refs = m.refs[:last]
}

// detach copies the manifest slice so subsequent incremental updates cannot
// mutate a slice that is also referenced by a previously committed index.
func (m *manifestCollection) detach() {
	manifests := make([]ocispec.Descriptor, len(m.manifests))
	copy(manifests, m.manifests)
	m.manifests = manifests
}

// manifestEntry returns the entry that the reference of c owns, and whether it
// owns one.
func manifestEntry(c resolver.Change) (ocispec.Descriptor, bool) {
	if !c.Exists {
		return ocispec.Descriptor{}, false
	}
	if c.Ref == c.Desc.Digest.String() {
		if c.Tagged {
			return ocispec.Descriptor{}, false
		}
		return deleteAnnotationRefName(c.Desc), true
	}
	return withAnnotationRefName(c.Desc, c.Ref), true
}
