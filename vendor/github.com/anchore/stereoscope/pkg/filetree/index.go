package filetree

import (
	"fmt"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/becheran/wildmatch-go"
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

	if _, ok := c.index[id]; ok {
		log.WithFields("id", id, "path", f.RealPath).Debug("overwriting existing file index entry")
	}

	if m.MIMEType != "" {
		if _, ok := c.byMIMEType[m.MIMEType]; !ok {
			c.byMIMEType[m.MIMEType] = file.NewIDSet()
		}
		// an empty MIME type means that we didn't have the contents of the file to determine the MIME type. If we have
		// the contents and the MIME type could not be determined then the default value is application/octet-stream.
		c.byMIMEType[m.MIMEType].Add(id)
	}

	basename := path.Base(string(f.RealPath))

	if _, ok := c.byBasename[basename]; !ok {
		c.byBasename[basename] = file.NewIDSet()
	}

	c.byBasename[basename].Add(id)
	c.basenames.Add(basename)

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
	c.RLock()
	defer c.RUnlock()

	var entries []IndexEntry
	for _, glob := range globs {
		if strings.Contains(glob, "**") {
			return nil, fmt.Errorf("basename glob patterns with '**' are not supported")
		}
		if strings.Contains(glob, "/") {
			return nil, fmt.Errorf("found directory separator in a basename")
		}

		patternObj := wildmatch.NewWildMatch(glob)
		var e error
		c.basenames.Each(func(b string) bool {
			if patternObj.IsMatch(b) {
				bns, err := c.GetByBasename(b)
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
