package image

import (
	"fmt"
	"io"
	"sync"

	"github.com/anchore/stereoscope/pkg/file"
	"github.com/anchore/stereoscope/pkg/filetree"
)

type FileCatalogReader interface {
	Layer(file.Reference) *Layer
	Open(file.Reference) (io.ReadCloser, error)
	filetree.IndexReader
}

// FileCatalog represents all file metadata and source tracing for all files contained within the image layer
// blobs (i.e. everything except for the image index/manifest/metadata files).
type FileCatalog struct {
	*sync.RWMutex
	filetree.Index
	layerByID  map[file.ID]*Layer
	openerByID map[file.ID]file.Opener
}

// NewFileCatalog returns an empty FileCatalog.
func NewFileCatalog() *FileCatalog {
	return &FileCatalog{
		RWMutex:    &sync.RWMutex{},
		Index:      filetree.NewIndex(),
		layerByID:  make(map[file.ID]*Layer),
		openerByID: make(map[file.ID]file.Opener),
	}
}

// Add creates a new FileCatalogEntry for the given file reference and metadata, cataloged by the ID of the
// file reference (overwriting any existing entries without warning).
func (c *FileCatalog) Add(f file.Reference, m file.Metadata, l *Layer, opener file.Opener) {
	c.Index.Add(f, m) // note: the index is already thread-safe
	c.addImageReferences(f.ID(), l, opener)
}

func (c *FileCatalog) AssociateOpener(f file.Reference, opener file.Opener) {
	c.addImageReferences(f.ID(), nil, opener)
}

func (c *FileCatalog) AssociateLayer(f file.Reference, l *Layer) {
	c.addImageReferences(f.ID(), l, nil)
}

func (c *FileCatalog) addImageReferences(id file.ID, l *Layer, opener file.Opener) {
	c.Lock()
	defer c.Unlock()
	if l != nil {
		c.layerByID[id] = l
	}
	if opener != nil {
		c.openerByID[id] = opener
	}
}

func (c *FileCatalog) Layer(f file.Reference) *Layer {
	c.RLock()
	defer c.RUnlock()

	return c.layerByID[f.ID()]
}

// Open returns a io.ReadCloser for the given file reference. The underlying io.ReadCloser will not attempt to
// allocate resources until the first read is performed.
func (c *FileCatalog) Open(f file.Reference) (io.ReadCloser, error) {
	c.RLock()
	defer c.RUnlock()

	opener, ok := c.openerByID[f.ID()]
	if !ok {
		return nil, fmt.Errorf("could not find file: %+v", f.RealPath)
	}

	if opener == nil {
		return nil, fmt.Errorf("no contents available for file: %+v", f.RealPath)
	}

	return opener()
}
