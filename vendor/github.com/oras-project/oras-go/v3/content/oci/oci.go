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

// Package oci provides access to an OCI content store.
// Reference: https://github.com/opencontainers/image-spec/blob/v1.1.1/image-layout.md
package oci

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/oras-project/oras-go/v3/content"
	"github.com/oras-project/oras-go/v3/errdef"
	"github.com/oras-project/oras-go/v3/internal/container/set"
	"github.com/oras-project/oras-go/v3/internal/descriptor"
	"github.com/oras-project/oras-go/v3/internal/graph"
	"github.com/oras-project/oras-go/v3/internal/ioutil"
	"github.com/oras-project/oras-go/v3/internal/manifestutil"
	"github.com/oras-project/oras-go/v3/internal/resolver"
	"github.com/oras-project/oras-go/v3/registry"
)

// Store implements `oras.Target`, and represents a content store
// based on file system with the OCI-Image layout.
// Reference: https://github.com/opencontainers/image-spec/blob/v1.1.1/image-layout.md
type Store struct {
	// AutoSaveIndex controls if the OCI store will automatically save the index
	// file when needed.
	//   - If AutoSaveIndex is set to true, the OCI store will automatically save
	//     the changes to `index.json` when
	//      1. pushing a manifest
	//      2. calling Tag(), Delete(), or GC()
	//   - If AutoSaveIndex is set to false, it's the caller's responsibility
	//     to manually call SaveIndex() when needed. This switch is store-wide,
	//     so it also defers the saves of every other goroutine using the same
	//     Store. Use it only when the Store is used exclusively by the caller.
	//   - Callers doing bulk work on a shared Store should prefer
	//     WithDeferredIndex, which defers only the saves made with its own
	//     context and always saves the index when it returns.
	//   - Default value: true.
	AutoSaveIndex bool

	// AutoGC controls if the OCI store will automatically clean dangling
	// (unreferenced) blobs created by the Delete() operation. This includes the
	// referrers and the unreferenced successor blobs of the deleted content.
	// Tagged manifests will not be deleted.
	//   - Default value: true.
	AutoGC bool

	// SyncIndex controls whether writes to `index.json` are synchronized to
	// stable storage before replacing the previous index. The containing
	// directory is then synchronized on a best-effort basis; this is not
	// supported on Windows. Synchronization can significantly increase latency.
	// This applies to both automatic saves and SaveIndex, but does not flush
	// blobs or metadata already written by New. Set it before using the store
	// concurrently. Temporary-file replacement is used regardless of this value;
	// without synchronization, delayed write failures and crash recovery are
	// subject to the file system's writeback behavior.
	//   - Default value: false.
	SyncIndex bool

	// MaxUnsafePushBytes is the maximum number of bytes UnsafePush accepts for
	// both known and unknown sizes. If <= 0, the default is used. Set it before
	// using the store concurrently.
	//   - Default value: 4 GiB.
	MaxUnsafePushBytes int64

	root        string
	indexPath   string
	index       *ocispec.Index
	storage     *Storage
	tagResolver *resolver.Memory
	graph       *graph.Memory

	// manifests is the manifest list of the index. It mirrors the resolver it
	// was made for, which is s.tagResolver except right after a GC that did not
	// save the index. It is guarded by indexLock.
	manifests *manifestCollection

	// sync ensures that most operations can be done concurrently, while Delete
	// has the exclusive access to Store if a delete operation is underway.
	// Operations such as Fetch, Push use sync.RLock(), while Delete uses
	// sync.Lock().
	sync sync.RWMutex
	// indexLock ensures that only one go-routine is writing to the index.
	indexLock sync.Mutex
	// indexBuf is reused to encode the index on every save. It is guarded by
	// indexLock.
	indexBuf bytes.Buffer
}

// indexDeferral is the state of one WithDeferredIndex scope. It travels in the
// context passed to the scope's function.
type indexDeferral struct {
	store       *Store
	mu          sync.Mutex
	active      bool // saves are deferred while true
	pendingSave bool // a save was deferred and has not been flushed
}

type indexDeferralKey struct{}

// flush ends the deferral and saves the index if a save was deferred.
func (d *indexDeferral) flush() error {
	d.mu.Lock()
	d.active = false
	pendingSave := d.pendingSave
	d.pendingSave = false
	d.mu.Unlock()

	if !pendingSave {
		return nil
	}
	return d.store.SaveIndex()
}

// tryDefer records that a save is deferred and reports true while the scope is
// active.
func (d *indexDeferral) tryDefer() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.active {
		return false
	}
	d.pendingSave = true
	return true
}

// New creates a new OCI store with context.Background().
//
// If `index.json` is present but empty, which a write interrupted by a
// full file system can leave behind, it is reinitialized rather than reported as
// an error. The blobs of the store are kept, but the tags recorded in the lost
// index are not recoverable, and the blobs they referenced become unreferenced
// and are removed by the next call to GC.
//
// Only a zero-length file is recovered this way. A write interrupted part-way
// can also leave a short, undecodable `index.json` behind, and that is still
// reported as an error, since it may be corruption whose surviving tags are
// worth keeping for inspection rather than discarding.
func New(root string) (*Store, error) {
	return NewWithContext(context.Background(), root)
}

// NewWithContext creates a new OCI store.
//
// See New for the handling of an empty `index.json`.
func NewWithContext(ctx context.Context, root string) (*Store, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path for %s: %w", root, err)
	}
	storage, err := NewStorage(rootAbs)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	tagResolver := resolver.NewMemory()
	store := &Store{
		AutoSaveIndex: true,
		AutoGC:        true,
		root:          rootAbs,
		indexPath:     filepath.Join(rootAbs, ocispec.ImageIndexFile),
		storage:       storage,
		tagResolver:   tagResolver,
		graph:         graph.NewMemory(),
		manifests:     newManifestCollection(tagResolver),
	}

	if err := ensureDir(filepath.Join(rootAbs, ocispec.ImageBlobsDir)); err != nil {
		return nil, err
	}
	if err := store.ensureOCILayoutFile(); err != nil {
		return nil, fmt.Errorf("invalid OCI Image Layout: %w", err)
	}
	if err := store.loadIndexFile(ctx); err != nil {
		return nil, fmt.Errorf("invalid OCI Image Index: %w", err)
	}

	return store, nil
}

// Fetch fetches the content identified by the descriptor. It returns an io.ReadCloser.
// It's recommended to close the io.ReadCloser before a Delete operation, otherwise
// Delete may fail (for example on NTFS file systems).
func (s *Store) Fetch(ctx context.Context, target ocispec.Descriptor) (io.ReadCloser, error) {
	s.sync.RLock()
	defer s.sync.RUnlock()

	return s.storage.Fetch(ctx, target)
}

// Push pushes the content, matching the expected descriptor.
func (s *Store) Push(ctx context.Context, expected ocispec.Descriptor, reader io.Reader) error {
	s.sync.RLock()
	defer s.sync.RUnlock()

	if err := s.storage.Push(ctx, expected, reader); err != nil {
		if errors.Is(err, errdef.ErrAlreadyExists) {
			if indexErr := s.ensureIndexed(ctx, expected); indexErr != nil {
				return indexErr
			}
		}
		return err
	}
	return s.indexNode(ctx, expected)
}

// UnsafePush pushes content whose digest or size may be unknown, subject to
// MaxUnsafePushBytes. See content.UnsafePusher for verification and error semantics.
func (s *Store) UnsafePush(ctx context.Context, expected ocispec.Descriptor, r io.Reader) (ocispec.Descriptor, error) {
	if err := ctx.Err(); err != nil {
		return ocispec.Descriptor{}, err
	}
	s.sync.RLock()
	defer s.sync.RUnlock()

	desc, err := s.storage.unsafePush(expected, ioutil.NewContextReader(ctx, r), s.MaxUnsafePushBytes)
	if err != nil {
		if errors.Is(err, errdef.ErrAlreadyExists) {
			if indexErr := s.ensureIndexed(ctx, desc); indexErr != nil {
				return ocispec.Descriptor{}, indexErr
			}
		}
		return desc, err
	}
	if err := s.indexNode(ctx, desc); err != nil {
		return ocispec.Descriptor{}, err
	}
	return desc, nil
}

// indexNode indexes desc in s.graph and records its digest in s.tagResolver
// when desc is a manifest.
func (s *Store) indexNode(ctx context.Context, desc ocispec.Descriptor) error {
	if err := s.graph.Index(ctx, s.storage, desc); err != nil {
		return err
	}
	if descriptor.IsManifest(desc) {
		// tag by digest
		return s.tag(ctx, desc, desc.Digest.String())
	}
	return nil
}

// ensureIndexed ensures that an already-stored blob (for example, one spared on
// disk by a prior GC within leftoverExpiry) is indexed in s.graph and, if it is
// a manifest, recorded by digest in s.tagResolver.
func (s *Store) ensureIndexed(ctx context.Context, desc ocispec.Descriptor) error {
	if !s.graph.Exists(desc) {
		if err := s.graph.Index(ctx, s.storage, desc); err != nil {
			return err
		}
	}
	if descriptor.IsManifest(desc) {
		dgst := desc.Digest.String()
		if _, err := s.tagResolver.Resolve(ctx, dgst); errors.Is(err, errdef.ErrNotFound) {
			return s.tag(ctx, desc, dgst)
		}
	}
	return nil
}

// Exists returns true if the described content exists.
func (s *Store) Exists(ctx context.Context, target ocispec.Descriptor) (bool, error) {
	s.sync.RLock()
	defer s.sync.RUnlock()

	return s.storage.Exists(ctx, target)
}

// Delete deletes the content matching the descriptor from the store. Delete may
// fail on certain systems (i.e. NTFS), if there is a process (i.e. an unclosed
// Reader) using target.
//   - If s.AutoGC is set to true, Delete will recursively remove the dangling
//     blobs caused by the current delete and the referrers of the manifests
//     being deleted.
func (s *Store) Delete(ctx context.Context, target ocispec.Descriptor) error {
	s.sync.Lock()
	defer s.sync.Unlock()

	deleteQueue := []ocispec.Descriptor{target}
	for len(deleteQueue) > 0 {
		head := deleteQueue[0]
		deleteQueue = deleteQueue[1:]

		// get referrers if applicable
		if s.AutoGC && descriptor.IsManifest(head) {
			referrers, err := registry.Referrers(ctx, &unsafeStore{s}, head, "")
			if err != nil {
				return err
			}
			deleteQueue = append(deleteQueue, referrers...)
		}

		// delete the head of queue
		danglings, err := s.delete(ctx, head)
		if err != nil {
			return err
		}
		if s.AutoGC {
			for _, d := range danglings {
				// do not delete existing tagged manifests
				if !s.isTagged(d) {
					deleteQueue = append(deleteQueue, d)
				}
			}
		}
	}

	return nil
}

// delete deletes one node and returns the dangling nodes caused by the delete.
func (s *Store) delete(ctx context.Context, target ocispec.Descriptor) ([]ocispec.Descriptor, error) {
	tagSet := s.tagResolver.TagSet(target)
	for reference := range tagSet {
		s.tagResolver.Untag(reference)
	}
	danglings := s.graph.Remove(target)
	if len(tagSet) > 0 && s.AutoSaveIndex {
		err := s.saveIndex()
		if err != nil {
			return nil, err
		}
	}
	if err := s.storage.Delete(ctx, target); err != nil {
		return nil, err
	}
	return danglings, nil
}

// Tag associates a reference string (e.g. "latest") with the descriptor.
// The reference string is recorded in the "org.opencontainers.image.ref.name"
// annotation of the descriptor. When saved, the updated descriptor is persisted
// in the `index.json` file.
//
//   - If the same reference string is tagged multiple times on different
//     descriptors, the descriptor from the last call will be stored.
//   - If the same descriptor is tagged multiple times with different reference
//     strings, multiple copies of the descriptor with different reference tags
//     will be stored in the `index.json` file.
//
// Reference: https://github.com/opencontainers/image-spec/blob/v1.1.1/image-layout.md#indexjson-file
func (s *Store) Tag(ctx context.Context, desc ocispec.Descriptor, reference string) error {
	s.sync.RLock()
	defer s.sync.RUnlock()

	if err := validateReference(reference); err != nil {
		return err
	}

	exists, err := s.storage.Exists(ctx, desc)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("%s: %s: %w", desc.Digest, desc.MediaType, errdef.ErrNotFound)
	}
	if descriptor.IsManifest(desc) && !s.graph.Exists(desc) {
		if err := s.graph.IndexAll(ctx, s.storage, descriptor.Plain(desc)); err != nil {
			return err
		}
	}

	return s.tag(ctx, desc, reference)
}

// tag tags a descriptor with a reference string.
func (s *Store) tag(ctx context.Context, desc ocispec.Descriptor, reference string) error {
	dgst := desc.Digest.String()
	if reference != dgst {
		// also tag desc by its digest
		if err := s.tagResolver.Tag(ctx, desc, dgst); err != nil {
			return err
		}
	}
	if err := s.tagResolver.Tag(ctx, desc, reference); err != nil {
		return err
	}
	return s.autoSaveIndex(ctx)
}

// Resolve resolves a reference to a descriptor.
//   - If the reference to be resolved is a tag, the returned descriptor will be
//     a full descriptor declared by github.com/opencontainers/image-spec/specs-go/v1.
//   - If the reference is a digest, the returned descriptor will be a
//     plain descriptor (containing only the digest, media type and size).
func (s *Store) Resolve(ctx context.Context, reference string) (ocispec.Descriptor, error) {
	s.sync.RLock()
	defer s.sync.RUnlock()

	if reference == "" {
		return ocispec.Descriptor{}, errdef.ErrMissingReference
	}

	// attempt resolving manifest
	desc, err := s.tagResolver.Resolve(ctx, reference)
	if err != nil {
		if errors.Is(err, errdef.ErrNotFound) {
			// attempt resolving blob
			return resolveBlob(os.DirFS(s.root), reference)
		}
		return ocispec.Descriptor{}, err
	}

	if reference == desc.Digest.String() {
		return descriptor.Plain(desc), nil
	}

	return desc, nil
}

// Untag disassociates a reference string from its descriptor.
// When saved, the descriptor entry cotanining the reference in the
// "org.opencontainers.image.ref.name" annotation is removed from the
// `index.json` file.
// The actual content identified by the descriptor is NOT deleted.
//
// Reference: https://github.com/opencontainers/image-spec/blob/v1.1.1/image-layout.md#indexjson-file
func (s *Store) Untag(ctx context.Context, reference string) error {
	if reference == "" {
		return errdef.ErrMissingReference
	}

	s.sync.RLock()
	defer s.sync.RUnlock()

	desc, err := s.tagResolver.Resolve(ctx, reference)
	if err != nil {
		return fmt.Errorf("resolving reference %q: %w", reference, err)
	}
	if reference == desc.Digest.String() {
		return fmt.Errorf("reference %q is a digest and not a tag: %w", reference, errdef.ErrInvalidReference)
	}

	s.tagResolver.Untag(reference)
	return s.autoSaveIndex(ctx)
}

// Predecessors returns the nodes directly pointing to the current node.
// Predecessors returns nil without error if the node does not exists in the
// store.
func (s *Store) Predecessors(ctx context.Context, node ocispec.Descriptor) ([]ocispec.Descriptor, error) {
	s.sync.RLock()
	defer s.sync.RUnlock()

	return s.graph.Predecessors(ctx, node)
}

// Tags lists the tags presented in the `index.json` file of the OCI layout,
// returned in ascending order.
// If `last` is NOT empty, the entries in the response start after the tag
// specified by `last`. Otherwise, the response starts from the top of the tags
// list.
//
// See also `Tags()` in the package `registry`.
func (s *Store) Tags(ctx context.Context, last string, fn func(tags []string) error) error {
	s.sync.RLock()
	defer s.sync.RUnlock()

	return listTags(s.tagResolver, last, fn)
}

// ensureOCILayoutFile ensures the `oci-layout` file.
func (s *Store) ensureOCILayoutFile() error {
	layoutFilePath := filepath.Join(s.root, ocispec.ImageLayoutFile)
	layoutFile, err := os.Open(layoutFilePath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to open OCI layout file: %w", err)
		}
		return writeOCILayoutFile(layoutFilePath)
	}
	defer layoutFile.Close()

	// An empty file is what an interrupted write leaves behind. It carries no
	// information, and it describes the same state as a missing file, so it is
	// treated in the same way and rewritten, rather than failing the store for
	// as long as it is there.
	empty, err := isEmptyFile(layoutFile)
	if err != nil {
		return fmt.Errorf("failed to stat OCI layout file: %w", err)
	}
	if empty {
		// the file is closed before it is replaced, since on Windows a file
		// cannot be renamed over while it is open.
		if err := layoutFile.Close(); err != nil {
			return fmt.Errorf("failed to close OCI layout file: %w", err)
		}
		return writeOCILayoutFile(layoutFilePath)
	}

	var layout ocispec.ImageLayout
	if err := json.NewDecoder(layoutFile).Decode(&layout); err != nil {
		return fmt.Errorf("failed to decode OCI layout file: %w", err)
	}
	return validateOCILayout(&layout)
}

// writeOCILayoutFile writes the `oci-layout` file at the given path.
func writeOCILayoutFile(layoutFilePath string) error {
	layout := ocispec.ImageLayout{
		Version: ocispec.ImageLayoutVersion,
	}
	layoutJSON, err := json.Marshal(layout)
	if err != nil {
		return fmt.Errorf("failed to marshal OCI layout file: %w", err)
	}
	return writeFileAtomic(layoutFilePath, layoutJSON, false)
}

// isEmptyFile reports whether the open file is zero-length.
func isEmptyFile(file *os.File) (bool, error) {
	fi, err := file.Stat()
	if err != nil {
		return false, err
	}
	return fi.Size() == 0, nil
}

// loadIndexFile reads index.json from the file system.
// Create index.json if it does not exist.
func (s *Store) loadIndexFile(ctx context.Context) error {
	indexFile, err := os.Open(s.indexPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to open index file: %w", err)
		}

		// write index.json if it does not exist
		return s.resetIndexFile()
	}
	defer indexFile.Close()

	// An empty file is what an interrupted write leaves behind. It is not a
	// valid index, and it describes the same state as a missing index file, so
	// it is treated in the same way and rewritten.
	empty, err := isEmptyFile(indexFile)
	if err != nil {
		return fmt.Errorf("failed to stat index file: %w", err)
	}
	if empty {
		// the file is closed before it is replaced, since on Windows a file
		// cannot be renamed over while it is open.
		if err := indexFile.Close(); err != nil {
			return fmt.Errorf("failed to close index file: %w", err)
		}
		return s.resetIndexFile()
	}

	var index ocispec.Index
	if err := json.NewDecoder(indexFile).Decode(&index); err != nil {
		return fmt.Errorf("failed to decode index file: %w", err)
	}
	s.index = &index
	return loadIndex(ctx, s.index, s.storage, s.tagResolver, s.graph)
}

// resetIndexFile sets the index to an empty index and writes it to the file
// system, replacing the `index.json` file if it is already present.
func (s *Store) resetIndexFile() error {
	s.index = &ocispec.Index{
		Versioned: specs.Versioned{
			SchemaVersion: 2, // historical value
		},
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: []ocispec.Descriptor{},
	}
	return s.writeIndexFile()
}

// SaveIndex writes the `index.json` file to the file system.
//   - If AutoSaveIndex is set to true (default value),
//     the OCI store will automatically save the changes to `index.json`
//     on Tag(), Delete(), and GC() calls, and when pushing a manifest.
//   - If AutoSaveIndex is set to false, it's the caller's responsibility
//     to manually call this method when needed. Callers doing bulk work on a
//     shared Store should prefer WithDeferredIndex.
func (s *Store) SaveIndex() error {
	s.sync.RLock()
	defer s.sync.RUnlock()

	return s.saveIndex()
}

func (s *Store) saveIndex() error {
	return s.saveIndexFrom(s.tagResolver)
}

// autoSaveIndex saves the index after an operation if AutoSaveIndex is set,
// unless ctx belongs to an active WithDeferredIndex scope of s. The operation
// has always updated the in-memory state before calling it, so a flush that
// observes the deferral also observes that update.
func (s *Store) autoSaveIndex(ctx context.Context) error {
	if !s.AutoSaveIndex {
		return nil
	}
	if d, ok := ctx.Value(indexDeferralKey{}).(*indexDeferral); ok && d.store == s && d.tryDefer() {
		return nil
	}
	return s.saveIndex()
}

// saveIndexFrom synchronizes index.json with the supplied tag resolver.
//
// The store's resolver uses incremental updates. A different resolver, such
// as the one constructed during garbage collection, requires its own manifest
// collection so that changes are not applied to the wrong reference state.
//
// The manifest slice is detached before mutation when it shares storage with
// the current index. If saving the index fails, the previous in-memory index
// is restored. The collection for the store's resolver is retained so pending
// changes can be retried.
func (s *Store) saveIndexFrom(tagResolver *resolver.Memory) error {
	s.indexLock.Lock()
	defer s.indexLock.Unlock()

	collection := s.manifests
	if collection.resolver != tagResolver {
		collection = newManifestCollection(tagResolver)
	} else if len(collection.manifests) > 0 && len(s.index.Manifests) > 0 &&
		&collection.manifests[0] == &s.index.Manifests[0] {
		// Preserve the committed index if syncing or writing fails.
		collection.detach()
	}

	collection.sync()

	oldManifests := s.index.Manifests
	s.index.Manifests = collection.manifests
	if err := s.writeIndexFile(); err != nil {
		s.index.Manifests = oldManifests
		if tagResolver == s.tagResolver {
			s.manifests = collection
		}
		return err
	}

	s.manifests = collection
	return nil
}

// writeIndexFile writes the `index.json` file. The caller must hold indexLock,
// or otherwise have exclusive access to the Store.
func (s *Store) writeIndexFile() error {
	// reuse the buffer of the previous save, rather than allocating a new one
	// the size of the whole index
	s.indexBuf.Reset()
	if err := json.NewEncoder(&s.indexBuf).Encode(s.index); err != nil {
		return fmt.Errorf("failed to marshal index file: %w", err)
	}
	// Encode ends the output with a newline, which json.Marshal does not
	indexJSON := bytes.TrimSuffix(s.indexBuf.Bytes(), []byte("\n"))
	return writeFileAtomic(s.indexPath, indexJSON, s.SyncIndex)
}

// GC removes garbage from Store. Unsaved index will be lost. To prevent unexpected
// loss, call SaveIndex() before GC or set AutoSaveIndex to true.
// The garbage to be cleaned are:
//   - unreferenced (dangling) blobs in Store which have no predecessors
//   - garbage blobs in the storage whose metadata is not stored in Store
//   - temporary files left behind by interrupted blob and metadata writes
//
// Unreachable blobs and temporary files modified within the last hour are kept,
// so blobs pushed by a multi-step copy (for example, layers pushed before their
// manifest is pushed and tagged) or by another Store on the same directory are
// not removed while the operation completes within that window. What a kept
// manifest pushed through this Store references is kept with it, whatever its
// modification time, so that a copy which skipped pushing a layer already on
// disk does not lose it. GC does not otherwise coordinate with concurrent
// writers: it rebuilds the index from this Store's in-memory references and
// overwrites index.json, which drops tags and manifests written by another
// Store since this Store was opened, and does not protect writes that take
// longer than the grace period, nor existing blobs that a copy skips pushing
// unless the manifest referencing them was pushed through this Store, since
// their modification time is not refreshed.
func (s *Store) GC(ctx context.Context) error {
	s.sync.Lock()
	defer s.sync.Unlock()

	// get reachable nodes by reloading the index
	tagResolver, newGraph, err := s.gcIndex(ctx)
	if err != nil {
		return fmt.Errorf("unable to reload index: %w", err)
	}
	// replace the resolver and graph only once the index is saved, so that a
	// failed save leaves the store as it was
	if s.AutoSaveIndex {
		if err := s.saveIndexFrom(tagResolver); err != nil {
			return fmt.Errorf("unable to save index: %w", err)
		}
	}
	oldGraph := s.graph
	s.tagResolver = tagResolver
	s.graph = newGraph
	reachableNodes := s.graph.DigestSet()

	// clean up garbage blobs in the storage
	stale := olderThan(leftoverExpiry)
	rootpath := filepath.Join(s.root, ocispec.ImageBlobsDir)
	algDirs, err := os.ReadDir(rootpath)
	if err != nil {
		return err
	}
	stalePaths := make(map[digest.Digest]string)
	var sparedManifests []ocispec.Descriptor
	for _, algDir := range algDirs {
		if !algDir.IsDir() {
			continue
		}
		alg := algDir.Name()
		// skip unsupported directories
		if !descriptor.IsSupportedAlgorithm(digest.Algorithm(alg)) {
			continue
		}
		algPath := path.Join(rootpath, alg)
		digestEntries, err := os.ReadDir(algPath)
		if err != nil {
			return err
		}
		for _, digestEntry := range digestEntries {
			if err := isContextDone(ctx); err != nil {
				return err
			}
			if digestEntry.IsDir() {
				continue
			}
			dgst := digestEntry.Name()
			blobDigest := digest.NewDigestFromEncoded(digest.Algorithm(alg), dgst)
			if err := blobDigest.Validate(); err != nil {
				// skip irrelevant content
				continue
			}
			if !reachableNodes.Contains(blobDigest) {
				if stale(digestEntry) {
					// the blob does not exist in Store, so collect it for
					// removal once the spared manifests are known
					stalePaths[blobDigest] = path.Join(algPath, dgst)
				} else if desc, ok := oldGraph.Node(blobDigest); ok {
					// keep spared blobs indexed so Predecessors still sees
					// manifests pushed before GC
					if err := s.graph.Index(ctx, s.storage, desc); err != nil {
						return err
					}
					if descriptor.IsManifest(desc) {
						// keep the digest reference so Resolve does not fall
						// back to an untyped blob descriptor
						if err := s.tagResolver.Tag(ctx, deleteAnnotationRefName(desc), blobDigest.String()); err != nil {
							return err
						}
						sparedManifests = append(sparedManifests, desc)
					}
				}
			}
		}
	}

	// A spared manifest is kept on its own modification time, but a blob it
	// references is judged on that blob's: a copy that finds a layer already on
	// disk skips pushing it, so the layer can be stale while the manifest is
	// not. Keep it too, or tagging the manifest later leaves content missing.
	if len(stalePaths) > 0 && len(sparedManifests) > 0 {
		referenced := graph.NewMemory()
		if err := referenced.IndexAll(ctx, s.storage, sparedManifests...); err != nil {
			return err
		}
		for dgst := range referenced.DigestSet() {
			delete(stalePaths, dgst)
		}
	}
	for _, blobPath := range stalePaths {
		if err := isContextDone(ctx); err != nil {
			return err
		}
		if err := os.Remove(blobPath); err != nil {
			return fmt.Errorf("unable to remove %s: %w", blobPath, err)
		}
	}

	// Re-tagging the spared manifests changed the resolver after the index was
	// saved above, so index.json lacks their digest entries until the next
	// save. Write them now, so that the index lists every manifest the store
	// holds, as it does after any other operation.
	//
	// This is opportunistic, like reclaiming leftovers: the collection above
	// has already removed blobs, so a failed save must not turn a completed GC
	// into a failed one. The entries stay pending in the collection and are
	// written by the next save, and without AutoSaveIndex that is SaveIndex.
	if len(sparedManifests) > 0 && s.AutoSaveIndex {
		_ = s.saveIndexFrom(s.tagResolver)
	}

	// Reclaiming leftovers is opportunistic: the collection above has already
	// completed, so a temporary file that cannot be removed must not turn a
	// successful GC into a failed one.
	s.gcLeftovers()
	return nil
}

// leftoverExpiry is how long an unreachable blob or temporary file must have
// been untouched before GC reclaims it, giving a write or multi-step copy in
// progress -- on this Store or another Store on the same directory -- a grace
// period to finish and record its references before its files can be collected.
const leftoverExpiry = time.Hour

// gcLeftovers removes the temporary files that interrupted writes leave
// behind: the ingest files of blob writes that did not complete, and the
// temporary files of metadata writes that were never renamed into place.
// Nothing else refers to them, and no other code path removes them.
func (s *Store) gcLeftovers() {
	// A write removes its own temporary file when it fails, so anything still
	// here is the residue of a process that died mid-write -- or a write that
	// another Store on the same directory is performing right now, which the
	// lock of this Store does not serialize against. Only entries that have
	// not been touched for leftoverExpiry are reclaimed, since the modification
	// time of a write in progress keeps advancing.
	stale := olderThan(leftoverExpiry)

	removeFiles(s.storage.ingestRoot, stale)
	// the temporary files of metadata writes are created next to the file that
	// they replace, in the root of the store.
	removeFiles(s.root, func(entry fs.DirEntry) bool {
		name := entry.Name()
		if !isTempFileOf(name, ocispec.ImageIndexFile) && !isTempFileOf(name, ocispec.ImageLayoutFile) {
			return false
		}
		return stale(entry)
	})
}

// olderThan returns a matcher accepting the directory entries that have not
// been modified for at least d. An entry whose information cannot be read is
// not matched: it has either just been removed by someone else, or it is not
// ours to reason about.
func olderThan(d time.Duration) func(entry fs.DirEntry) bool {
	return func(entry fs.DirEntry) bool {
		fi, err := entry.Info()
		if err != nil {
			return false
		}
		return time.Since(fi.ModTime()) >= d
	}
}

// removeFiles removes the files in dir that are matched by match. Directories,
// entries that are not matched, and entries that cannot be removed are left
// alone.
func removeFiles(dir string, match func(entry fs.DirEntry) bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || !match(entry) {
			continue
		}
		_ = os.Remove(filepath.Join(dir, entry.Name()))
	}
}

// gcIndex reloads the index and returns the rebuilt metadata without applying
// it. Information of untagged blobs are cleaned and only tagged blobs remain.
func (s *Store) gcIndex(ctx context.Context) (*resolver.Memory, *graph.Memory, error) {
	return gcReachable(ctx, s.storage, s.tagResolver.Map())
}

// gcReachable is the rule that decides which content a Store keeps when it is
// garbage collected. [Store.GC] and [Verify] both apply it, so that what Verify
// reports as unreferenced is what GC removes (excluding recent blobs within the
// GC grace period).
//
// refMap holds the references of the store, as returned by
// [resolver.Memory.Map]: a tag maps to its descriptor, and an entry whose key is
// the digest of its descriptor is an untagged manifest. The reachable content is
// the tagged manifests and what they reference, plus the untagged manifests
// whose chain of subjects reaches one of those. An untagged manifest that is not
// such a referrer is not reachable.
func gcReachable(ctx context.Context, fetcher content.Fetcher, refMap map[string]ocispec.Descriptor) (*resolver.Memory, *graph.Memory, error) {
	tagResolver := resolver.NewMemory()
	graph := graph.NewMemory()
	tagged := set.New[digest.Digest]()

	// index tagged manifests
	for ref, desc := range refMap {
		if ref == desc.Digest.String() {
			continue
		}
		if err := tagResolver.Tag(ctx, deleteAnnotationRefName(desc), desc.Digest.String()); err != nil {
			return nil, nil, err
		}
		if err := tagResolver.Tag(ctx, desc, ref); err != nil {
			return nil, nil, err
		}
		plain := descriptor.Plain(desc)
		if err := graph.IndexAll(ctx, fetcher, plain); err != nil {
			return nil, nil, err
		}
		tagged.Add(desc.Digest)
	}

	// index referrer manifests
	for ref, desc := range refMap {
		if ref != desc.Digest.String() || tagged.Contains(desc.Digest) {
			continue
		}
		// check if the referrers manifest can traverse to the existing graph
		subject := &desc
		for {
			next, err := manifestutil.Subject(ctx, fetcher, *subject)
			if err != nil {
				if errors.Is(err, errdef.ErrNotFound) {
					// The manifest or a subject in its chain is missing; the
					// chain cannot reach a tagged manifest.
					break
				}
				return nil, nil, err
			}
			if next == nil {
				break
			}
			if graph.Exists(*next) {
				if err := tagResolver.Tag(ctx, deleteAnnotationRefName(desc), desc.Digest.String()); err != nil {
					return nil, nil, err
				}
				plain := descriptor.Plain(desc)
				if err := graph.IndexAll(ctx, fetcher, plain); err != nil {
					return nil, nil, err
				}
				break
			}
			subject = next
		}
	}
	// Keep digest references for manifests reachable from tagged roots and
	// referrers, so saving the rebuilt index preserves their descriptors.
	for ref, desc := range refMap {
		if ref == desc.Digest.String() && graph.Exists(desc) {
			if err := tagResolver.Tag(ctx, deleteAnnotationRefName(desc), ref); err != nil {
				return nil, nil, err
			}
		}
	}
	return tagResolver, graph, nil
}

// isTagged checks if the blob given by the descriptor is tagged.
func (s *Store) isTagged(desc ocispec.Descriptor) bool {
	return s.tagResolver.Tagged(desc)
}

// WithDeferredIndex calls fn with a context derived from ctx, and defers the
// saves of `index.json` made by Push and Tag (and Untag) on s with that
// context, or a context derived from it, until fn returns. It then saves the
// index once. For a bulk operation such as copying an image index with many
// manifests, this replaces one whole-file write per manifest with a single
// write.
//
// fn must not return until all operations it starts with the provided context
// have completed. In particular, fn must wait for any goroutines that perform
// store operations with that context. Operations still in flight when fn
// returns are not covered by the final flush.
//
// The index is saved when fn returns, whether it succeeded, failed or
// panicked, since the blobs fn pushed are already on disk and would otherwise
// be left unreferenced and be removed by the next GC. The error of that save is
// joined with the error of fn. Nothing is saved if no save was deferred, and
// nothing is deferred (or saved) if AutoSaveIndex is false, since the caller
// then owns saving the index.
//
// The context passed to fn can outlive the call; calls made with it afterwards
// save immediately.
//
// The index file is only guaranteed to be up to date once WithDeferredIndex
// returns. A process that exits before then loses the tags of the scope, but
// never leaves a torn file.
func (s *Store) WithDeferredIndex(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	d := &indexDeferral{store: s, active: true}
	// deferred, so that the index is also saved if fn panics
	defer func() {
		err = errors.Join(err, d.flush())
	}()
	return fn(context.WithValue(ctx, indexDeferralKey{}, d))
}

// unsafeStore is used to bypass lock restrictions in Delete.
type unsafeStore struct {
	*Store
}

func (s *unsafeStore) Fetch(ctx context.Context, target ocispec.Descriptor) (io.ReadCloser, error) {
	return s.storage.Fetch(ctx, target)
}

func (s *unsafeStore) Predecessors(ctx context.Context, node ocispec.Descriptor) ([]ocispec.Descriptor, error) {
	return s.graph.Predecessors(ctx, node)
}

// isContextDone returns an error if the context is done.
// Reference: https://pkg.go.dev/context#Context
func isContextDone(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// validateReference validates ref.
func validateReference(ref string) error {
	if ref == "" {
		return errdef.ErrMissingReference
	}

	// TODO: may enforce more strict validation if needed.
	return nil
}
