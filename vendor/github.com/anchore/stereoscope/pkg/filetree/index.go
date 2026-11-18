package filetree

import (
	"fmt"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/scylladb/go-set/strset"

	"github.com/anchore/stereoscope/internal/log"
	"github.com/anchore/stereoscope/pkg/file"
)

type Index interface {
	IndexReader
	IndexWriter
}

type IndexReader interface {
	Exists(f file.Reference) bool
	Get(f file.Reference) (IndexEntry, error)
	GetByMIMEType(mTypes ...string) ([]IndexEntry, error)
	GetByFileType(fTypes ...file.Type) ([]IndexEntry, error)
	GetByExtension(extensions ...string) ([]IndexEntry, error)
	GetByBasename(basenames ...string) ([]IndexEntry, error)
	// GetByBasenameGlob returns the entries whose basename matches any of the given doublestar globs. A glob
	// matches a single path segment, so one containing '/' or '**' is rejected. Rejected and invalid globs return an
	// error wrapping doublestar.ErrBadPattern.
	GetByBasenameGlob(globs ...string) ([]IndexEntry, error)
}

type IndexWriter interface {
	Add(f file.Reference, m file.Metadata)
}

// Index represents all file metadata and source tracing for all files contained within the image layer
// blobs (i.e. everything except for the image index/manifest/metadata files).
type index struct {
	*sync.RWMutex
	index       map[file.ID]IndexEntry
	byFileType  map[file.Type]file.IDSet
	byMIMEType  map[string]file.IDSet
	byExtension map[string]file.IDSet
	byBasename  map[string]file.IDSet
	basenames   *strset.Set
}

// NewIndex returns an empty Index.
func NewIndex() Index {
	return &index{
		RWMutex:     &sync.RWMutex{},
		index:       make(map[file.ID]IndexEntry),
		byFileType:  make(map[file.Type]file.IDSet),
		byMIMEType:  make(map[string]file.IDSet),
		byExtension: make(map[string]file.IDSet),
		byBasename:  make(map[string]file.IDSet),
		basenames:   strset.New(),
	}
}

// IndexEntry represents all stored metadata for a single file reference.
type IndexEntry struct {
	file.Reference
	file.Metadata
}

// Add creates a new IndexEntry for the given file reference and metadata, cataloged by the ID of the
// file reference (overwriting any existing entries without warning).
func (c *index) Add(f file.Reference, m file.Metadata) {
	c.Lock()
	defer c.Unlock()

	id := f.ID()

	if existing, ok := c.index[id]; ok {
		log.WithFields("id", id, "path", f.RealPath).Debug("overwriting existing file index entry")
		// the path (and so basename and extensions) is fixed by the ID, but the metadata may differ (e.g. a layer
		// tar holding the same path twice), so drop the ID from the buckets keyed by the old metadata
		if existing.MIMEType != "" {
			c.byMIMEType[existing.MIMEType].Remove(id)
		}
		c.byFileType[existing.Type].Remove(id)
	}

	if m.MIMEType != "" {
		if _, ok := c.byMIMEType[m.MIMEType]; !ok {
			c.byMIMEType[m.MIMEType] = file.NewIDSet()
		}
		// an empty MIME type means that we didn't have the contents of the file to determine the MIME type. If we have
		// the contents and the MIME type could not be determined then the default value is application/octet-stream.
		c.byMIMEType[m.MIMEType].Add(id)
	}

	// the root has no basename (path.Base gives "/"), and indexing it would let a class like [!.] match a
	// separator that getByBasename then rejects, failing the whole basename glob search
	if basename := path.Base(string(f.RealPath)); basename != "/" {
		if _, ok := c.byBasename[basename]; !ok {
			c.byBasename[basename] = file.NewIDSet()
		}

		c.byBasename[basename].Add(id)
		c.basenames.Add(basename)
	}

	for _, ext := range fileExtensions(string(f.RealPath)) {
		if _, ok := c.byExtension[ext]; !ok {
			c.byExtension[ext] = file.NewIDSet()
		}
		c.byExtension[ext].Add(id)
	}

	if _, ok := c.byFileType[m.Type]; !ok {
		c.byFileType[m.Type] = file.NewIDSet()
	}
	c.byFileType[m.Type].Add(id)

	c.index[id] = IndexEntry{
		Reference: f,
		Metadata:  m,
	}
}

// Remove drops the entry for the given file reference, if any. It is deliberately not on IndexWriter, which
// outside code may implement: callers that need it assert for it.
func (c *index) Remove(f file.Reference) {
	c.Lock()
	defer c.Unlock()

	id := f.ID()
	existing, ok := c.index[id]
	if !ok {
		return
	}
	if existing.MIMEType != "" {
		removeFromBucket(c.byMIMEType, existing.MIMEType, id)
	}
	removeFromBucket(c.byFileType, existing.Type, id)
	if basename := path.Base(string(f.RealPath)); basename != "/" {
		if removeFromBucket(c.byBasename, basename, id) {
			c.basenames.Remove(basename)
		}
	}
	for _, ext := range fileExtensions(string(f.RealPath)) {
		removeFromBucket(c.byExtension, ext, id)
	}
	delete(c.index, id)
}

// removeFromBucket removes id from the set at key, dropping the set once it is empty so lookups and listings of the
// keys never see one with nothing behind it. It reports whether the set was dropped.
func removeFromBucket[K comparable](buckets map[K]file.IDSet, key K, id file.ID) bool {
	set, ok := buckets[key]
	if !ok {
		return false
	}
	set.Remove(id)
	if len(set) > 0 {
		return false
	}
	delete(buckets, key)
	return true
}

// Exists indicates if the given file reference exists in the index.
func (c *index) Exists(f file.Reference) bool {
	c.RLock()
	defer c.RUnlock()
	_, ok := c.index[f.ID()]
	return ok
}

// Get fetches a IndexEntry for the given file reference, or returns an error if the file reference has not
// been added to the index.
func (c *index) Get(f file.Reference) (IndexEntry, error) {
	c.RLock()
	defer c.RUnlock()
	value, ok := c.index[f.ID()]
	if !ok {
		return IndexEntry{}, os.ErrNotExist
	}
	return value, nil
}

func (c *index) GetByFileType(fTypes ...file.Type) ([]IndexEntry, error) {
	c.RLock()
	defer c.RUnlock()

	var entries []IndexEntry

	for _, fType := range fTypes {
		fileIDs, ok := c.byFileType[fType]
		if !ok {
			continue
		}

		for _, id := range fileIDs.Sorted() {
			entry, ok := c.index[id]
			if !ok {
				return nil, os.ErrNotExist
			}
			entries = append(entries, entry)
		}
	}

	return entries, nil
}

func (c *index) GetByMIMEType(mTypes ...string) ([]IndexEntry, error) {
	c.RLock()
	defer c.RUnlock()

	var entries []IndexEntry

	for _, mType := range mTypes {
		fileIDs, ok := c.byMIMEType[mType]
		if !ok {
			continue
		}

		for _, id := range fileIDs.Sorted() {
			entry, ok := c.index[id]
			if !ok {
				return nil, os.ErrNotExist
			}
			entries = append(entries, entry)
		}
	}

	return entries, nil
}

func (c *index) GetByExtension(extensions ...string) ([]IndexEntry, error) {
	c.RLock()
	defer c.RUnlock()

	var entries []IndexEntry

	for _, extension := range extensions {
		fileIDs, ok := c.byExtension[extension]
		if !ok {
			continue
		}

		for _, id := range fileIDs.Sorted() {
			entry, ok := c.index[id]
			if !ok {
				return nil, os.ErrNotExist
			}
			entries = append(entries, entry)
		}
	}

	return entries, nil
}

func (c *index) GetByBasename(basenames ...string) ([]IndexEntry, error) {
	c.RLock()
	defer c.RUnlock()
	return c.getByBasename(basenames...)
}

// getByBasename is the lock-free implementation of GetByBasename. The caller must hold at least a read lock.
func (c *index) getByBasename(basenames ...string) ([]IndexEntry, error) {
	var entries []IndexEntry

	for _, basename := range basenames {
		if strings.Contains(basename, "/") {
			return nil, fmt.Errorf("found directory separator in a basename")
		}

		fileIDs, ok := c.byBasename[basename]
		if !ok {
			continue
		}

		for _, id := range fileIDs.Sorted() {
			entry, ok := c.index[id]
			if !ok {
				return nil, os.ErrNotExist
			}
			entries = append(entries, entry)
		}
	}

	return entries, nil
}

func (c *index) GetByBasenameGlob(globs ...string) ([]IndexEntry, error) {
	// validate every glob up front, so that every basename is matched against a known-good pattern
	for _, glob := range globs {
		if strings.Contains(glob, "**") {
			return nil, fmt.Errorf("%w: basename glob %q contains '**'", doublestar.ErrBadPattern, glob)
		}
		if strings.Contains(glob, "/") {
			return nil, fmt.Errorf("%w: basename glob %q contains a directory separator", doublestar.ErrBadPattern, glob)
		}
		if !doublestar.ValidatePattern(glob) {
			return nil, fmt.Errorf("%w: invalid basename glob %q", doublestar.ErrBadPattern, glob)
		}
	}

	c.RLock()
	defer c.RUnlock()

	var entries []IndexEntry
	for _, glob := range globs {
		var e error
		c.basenames.Each(func(b string) bool {
			if doublestar.MatchUnvalidated(glob, b) {
				bns, err := c.getByBasename(b)
				if err != nil {
					e = fmt.Errorf("unable to fetch file references by basename (%q): %w", b, err)
					return false
				}
				entries = append(entries, bns...)
			}
			return true
		})
		if e != nil {
			return nil, e
		}
	}

	return entries, nil
}

func fileExtensions(p string) []string {
	var exts []string
	p = strings.TrimSpace(p)

	// ignore oddities
	if strings.HasSuffix(p, ".") {
		return exts
	}

	// ignore directories
	if strings.HasSuffix(p, "/") {
		return exts
	}

	// ignore . which indicate a hidden file
	p = strings.TrimLeft(path.Base(p), ".")
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '.' {
			exts = append(exts, p[i:])
		}
	}
	return exts
}
