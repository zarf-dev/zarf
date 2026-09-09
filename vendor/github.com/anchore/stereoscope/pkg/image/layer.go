package image

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/scylladb/go-set/strset"
	"github.com/sylabs/squashfs"
	"github.com/wagoodman/go-partybus"
	"github.com/wagoodman/go-progress"

	"github.com/anchore/stereoscope/internal/bus"
	"github.com/anchore/stereoscope/internal/log"
	"github.com/anchore/stereoscope/pkg/event"
	"github.com/anchore/stereoscope/pkg/file"
	"github.com/anchore/stereoscope/pkg/filetree"
)

const (
	SingularitySquashFSLayer       types.MediaType = "application/vnd.sylabs.sif.layer.v1.squashfs"
	BuildKitZstdCompressedLayer    types.MediaType = "application/vnd.docker.image.rootfs.diff.tar.zstd"
	BuildKitZstdCompressedLayerAlt types.MediaType = "application/vnd.docker.image.rootfs.diff.tar+zstd" // we're future proofing against a possible media type variation
)

// standardLayerMediaTypes are tar-based layer media types that can be processed with standard tar indexing.
var standardLayerMediaTypes = strset.New(
	string(types.OCILayer),
	string(types.OCIUncompressedLayer),
	string(types.OCIRestrictedLayer),
	string(types.OCIUncompressedRestrictedLayer),
	string(types.OCILayerZStd),
	string(types.DockerLayer),
	string(types.DockerForeignLayer),
	string(types.DockerUncompressedLayer),
	string(BuildKitZstdCompressedLayer),
	string(BuildKitZstdCompressedLayerAlt),
)

// singularityLayerMediaTypes are SquashFS-based layer media types.
var singularityLayerMediaTypes = strset.New(
	string(SingularitySquashFSLayer),
)

// isSupportedLayerMediaType returns true if the given media type is supported for layer processing.
func isSupportedLayerMediaType(mt types.MediaType) bool {
	return standardLayerMediaTypes.Has(string(mt)) || singularityLayerMediaTypes.Has(string(mt))
}

// validateLayerMediaTypes checks all layers have supported media types before processing.
func validateLayerMediaTypes(layers []v1.Layer) error {
	var unsupported []string
	for idx, layer := range layers {
		mt, err := layer.MediaType()
		if err != nil {
			return fmt.Errorf("unable to get media type for layer %d: %w", idx, err)
		}
		if !isSupportedLayerMediaType(mt) {
			unsupported = append(unsupported, fmt.Sprintf("layer %d: %s", idx, mt))
		}
	}
	if len(unsupported) > 0 {
		return fmt.Errorf("unsupported layer media type(s): %s", strings.Join(unsupported, ", "))
	}
	return nil
}

// Layer represents a single layer within a container image.
type Layer struct {
	// layer is the raw layer metadata and content provider from the GCR lib
	layer v1.Layer
	// indexedContent provides index access to the cached and unzipped layer tar
	indexedContent *file.TarIndex
	// Metadata contains select layer attributes
	Metadata LayerMetadata
	// Tree is a filetree that represents the structure of the layer tar contents ("diff tree")
	Tree filetree.Reader
	// SquashedTree is a filetree that represents the combination of this layers diff tree and all diff trees
	// in lower layers relative to this one.
	SquashedTree filetree.Reader
	// fileCatalog contains all file metadata for all files in all layers (not just this layer)
	fileCatalog           *FileCatalog
	SquashedSearchContext filetree.Searcher
	SearchContext         filetree.Searcher
}

// NewLayer provides a new, unread layer object.
func NewLayer(layer v1.Layer) *Layer {
	return &Layer{
		layer: layer,
	}
}

func (l *Layer) uncompressedCache(uncompressedLayersCacheDir string) (string, error) {
	if uncompressedLayersCacheDir == "" {
		return "", fmt.Errorf("no cache directory given")
	}

	path := path.Join(uncompressedLayersCacheDir, l.Metadata.Digest)

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return path, nil
	}

	log.WithFields("index", l.Metadata.Index, "path", path).Trace("start uncompressed layer cache")
	startTime := time.Now()

	rawReader, err := l.layer.Uncompressed()
	if err != nil {
		return "", err
	}
	defer rawReader.Close()

	fh, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("unable to create layer cache dir=%q : %w", path, err)
	}
	defer fh.Close()

	if _, err := io.Copy(fh, rawReader); err != nil {
		return "", fmt.Errorf("unable to populate layer cache dir=%q : %w", path, err)
	}
	log.WithFields("index", l.Metadata.Index, "path", path, "time", time.Since(startTime)).Trace("completed uncompressed layer cache")

	return path, nil
}

// Read parses information from the underlying layer tar into this struct. This includes layer metadata, the layer
// file tree, and the layer squash tree.
func (l *Layer) Read(catalog *FileCatalog, idx int, uncompressedLayersCacheDir string) error {
	mediaType, err := l.layer.MediaType()
	if err != nil {
		return err
	}
	tree := filetree.New()
	l.Tree = tree
	l.fileCatalog = catalog

	var readErr error
	switch {
	case standardLayerMediaTypes.Has(string(mediaType)):
		readErr = l.readStandardImageLayer(idx, uncompressedLayersCacheDir, tree)
	case singularityLayerMediaTypes.Has(string(mediaType)):
		readErr = l.readSingularityImageLayer(idx, uncompressedLayersCacheDir, tree)
	default:
		return fmt.Errorf("unknown layer media type: %+v", mediaType)
	}
	if readErr != nil {
		return readErr
	}

	startTime := time.Now()
	l.SearchContext = filetree.NewSearchContext(l.Tree, l.fileCatalog.Index)
	log.WithFields("index", idx, "time", time.Since(startTime)).Trace("completed layer search context")

	return nil
}

func (l *Layer) readStandardImageLayer(idx int, uncompressedLayersCacheDir string, tree *filetree.FileTree) error {
	var err error
	l.Metadata, err = newLayerMetadata(l.layer, idx)
	monitor := trackReadProgress(l.Metadata)
	if err != nil {
		return err
	}

	log.WithFields("index", l.Metadata.Index, "digest", l.Metadata.Digest, "mediaType", l.Metadata.MediaType).Trace("reading uncompressed image layer")

	tarFilePath, err := l.uncompressedCache(uncompressedLayersCacheDir)
	if err != nil {
		return err
	}

	startTime := time.Now()
	l.indexedContent, err = file.NewTarIndex(
		tarFilePath,
		layerTarIndexer(tree, l.fileCatalog, &l.Metadata.Size, l, monitor),
	)
	if err != nil {
		return fmt.Errorf("failed to read layer=%q tar : %w", l.Metadata.Digest, err)
	}
	log.WithFields("index", l.Metadata.Index, "digest", l.Metadata.Digest, "mediaType", l.Metadata.MediaType, "time", time.Since(startTime)).Trace("completed indexing image layer")

	monitor.SetCompleted()
	return nil
}

func (l *Layer) readSingularityImageLayer(idx int, uncompressedLayersCacheDir string, tree *filetree.FileTree) error {
	var err error
	l.Metadata, err = newLayerMetadata(l.layer, idx)
	if err != nil {
		return err
	}

	log.Debugf("layer metadata: index=%+v digest=%+v mediaType=%+v",
		l.Metadata.Index,
		l.Metadata.Digest,
		l.Metadata.MediaType)

	monitor := trackReadProgress(l.Metadata)
	sqfsFilePath, err := l.uncompressedCache(uncompressedLayersCacheDir)
	if err != nil {
		return err
	}

	if err := file.WalkSquashFS(sqfsFilePath, squashfsVisitor(tree, l.fileCatalog, &l.Metadata.Size, l, monitor)); err != nil {
		return fmt.Errorf("failed to walk layer=%q: %w", l.Metadata.Digest, err)
	}

	monitor.SetCompleted()
	return nil
}

// OpenPath reads the file contents for the given path from the underlying layer blob, relative to the layers "diff tree".
// An error is returned if there is no file at the given path and layer or the read operation cannot continue.
func (l *Layer) OpenPath(path file.Path) (io.ReadCloser, error) {
	return fetchReaderByPath(l.Tree, l.fileCatalog, path)
}

// OpenPathFromSquash reads the file contents for the given path from the underlying layer blob, relative to the layers squashed file tree.
// An error is returned if there is no file at the given path and layer or the read operation cannot continue.
func (l *Layer) OpenPathFromSquash(path file.Path) (io.ReadCloser, error) {
	return fetchReaderByPath(l.SquashedTree, l.fileCatalog, path)
}

// FileContents reads the file contents for the given path from the underlying layer blob, relative to the layers "diff tree".
// An error is returned if there is no file at the given path and layer or the read operation cannot continue.
//
// Deprecated: use OpenPath() instead.
func (l *Layer) FileContents(path file.Path) (io.ReadCloser, error) {
	return fetchReaderByPath(l.Tree, l.fileCatalog, path)
}

// FileContentsFromSquash reads the file contents for the given path from the underlying layer blob, relative to the layers squashed file tree.
// An error is returned if there is no file at the given path and layer or the read operation cannot continue.
//
// Deprecated: use OpenPathFromSquash() instead.
func (l *Layer) FileContentsFromSquash(path file.Path) (io.ReadCloser, error) {
	return fetchReaderByPath(l.SquashedTree, l.fileCatalog, path)
}

// FilesByMIMEType returns file references for files that match at least one of the given MIME types relative to each layer tree.
//
// Deprecated: use SearchContext().SearchByMIMEType() instead.
func (l *Layer) FilesByMIMEType(mimeTypes ...string) ([]file.Reference, error) {
	var refs []file.Reference
	refVias, err := l.SearchContext.SearchByMIMEType(mimeTypes...)
	if err != nil {
		return nil, err
	}
	for _, refVia := range refVias {
		if refVia.HasReference() {
			refs = append(refs, *refVia.Reference)
		}
	}
	return refs, nil
}

// FilesByMIMETypeFromSquash returns file references for files that match at least one of the given MIME types relative to the squashed file tree representation.
//
// Deprecated: use SquashedSearchContext().SearchByMIMEType() instead.
func (l *Layer) FilesByMIMETypeFromSquash(mimeTypes ...string) ([]file.Reference, error) {
	var refs []file.Reference
	refVias, err := l.SquashedSearchContext.SearchByMIMEType(mimeTypes...)
	if err != nil {
		return nil, err
	}
	for _, refVia := range refVias {
		if refVia.HasReference() {
			refs = append(refs, *refVia.Reference)
		}
	}
	return refs, nil
}

// adoptHardLinkInode rewrites metadata for a hardlink tar header to describe the file it names, and
// returns that file's opener. Returns false if that file is not in this layer (a malformed archive),
// leaving the entry to be indexed from its own header.
//
// a hardlink is a second name for an inode already in this layer, not a path to resolve later, and
// its header carries no data section:
//
//   - adopted from the target: size, MIME type, type, link destination, and the file type bits of
//     the mode, which tar does not record on a link header. A hardlink naming a symlink is itself a
//     symlink.
//   - kept from this header: permission bits, uid, gid, mtime, which tar already records correctly.
//
// "inode" is an analogy. There is no such object here: each name keeps its own file.Reference, ID
// and catalog entry, and what is shared is the opener plus a copy of the values above. Pointing both
// names at one reference would model the filesystem more literally, but it would leave a node's
// RealPath disagreeing with its reference's, and drop the second name as a location callers can
// address at all.
//
// Binding here rather than at resolution time is what makes the name immune to later layers: the
// catalog is keyed by file.ID and is never squashed or merged, so replacing or deleting the other
// name cannot change what this one refers to, which is what the filesystem does.
//
// where this model bends the truth:
//
//   - on disk neither name is "the hardlink". Which one a tar marks is an artifact of directory walk
//     order (alphabetical in practice), so it is often the one a person would call the original.
//   - adopting erases that mark, so the names come out symmetric. But nothing can then answer "is
//     this path also known as something else", the way nlink would.
//   - we trust the headers. Data on a link header with an empty regular header would be adopted
//     backwards and serve nothing.
//   - when adoption fails, or is undone by the caller because the adopted type collides with a node
//     already at that path, that one name keeps TypeHardLink, size 0 and an empty data section: the
//     lopsided picture this function exists to avoid.
//   - only this layer is searched. extraction resolves a link name against everything applied so
//     far, so a name pointing into a lower layer does work in a real runtime and is not adopted
//     here. matching that needs the squash as of the previous layer, not a walk of each lower tree,
//     which would resolve names that an intervening whiteout had already deleted. squashing runs
//     after all layers are read, so threading it in would make every layer's indexing wait on the
//     one below it. no mainstream builder emits these: they keep the inode table per archive.
func adoptHardLinkInode(ft filetree.Reader, fileCatalog *FileCatalog, metadata *file.Metadata) (file.Opener, bool) {
	// a hardlink's link name refers to an earlier member of this same archive, relative to its root.
	// the lookup below is not literal: a miss retries following ancestor symlinks, which is what
	// link(2) does when extracting the archive for real
	linkPath := file.Path(path.Clean(file.DirSeparator + metadata.LinkDestination))

	// an empty link name cleans to the tree root, which would otherwise adopt the root directory
	if metadata.LinkDestination == "" || linkPath == file.Path(file.DirSeparator) {
		return nil, false
	}

	exists, resolution, err := ft.File(linkPath)
	if err != nil || !exists || resolution == nil || resolution.Reference == nil {
		return nil, false
	}

	target, err := fileCatalog.Get(*resolution.Reference)
	if err != nil {
		return nil, false
	}

	// link(2) refuses a directory, so an archive naming one is malformed. adopting it anyway would
	// describe this name as a directory that cannot be listed or walked.
	// a target that is still a hardlink is one that failed to adopt, and taking its description would
	// spread that failure while reporting success, down to its link destination
	if target.Type == file.TypeDirectory || target.Type == file.TypeHardLink {
		return nil, false
	}

	opener := fileCatalog.opener(resolution.ID())
	if opener == nil {
		return nil, false
	}

	metadata.FileInfo = file.ManualInfo{
		NameValue: path.Base(metadata.Path),
		SizeValue: target.Size(),
		// permission bits stay from this header, but the file type bits come from the target: tar
		// records no type bits on a link header, and two names for one inode share a mode
		ModeValue:    metadata.Mode()&^fs.ModeType | target.Mode()&fs.ModeType,
		ModTimeValue: metadata.ModTime(),
		// deliberately the un-adopted header: it is the escape hatch to what the tar actually said,
		// so it still reports TypeLink, size 0 and this name's own link name
		SysValue: metadata.Sys(),
	}
	metadata.MIMEType = target.MIMEType
	metadata.Type = target.Type
	metadata.LinkDestination = target.LinkDestination

	return opener, true
}

func layerTarIndexer(ft filetree.ReadWriter, fileCatalog *FileCatalog, size *int64, layerRef *Layer, monitor *progress.Manual) file.TarIndexVisitor {
	builder := filetree.NewBuilder(ft, fileCatalog.Index)

	return func(index file.TarIndexEntry) error {
		var err error
		var entry = index.ToTarFileEntry()

		var contents = index.Open()
		defer func() {
			if err := contents.Close(); err != nil {
				log.Warnf("unable to close file while indexing layer: %+v", err)
			}
		}()
		metadata := file.NewMetadata(entry.Header, contents)

		// a hardlink names a file already present in this layer; describe it as that file and read
		// its contents through that file's opener rather than this header's empty data section
		var hardLinkOpener file.Opener
		var preAdoptionMetadata *file.Metadata
		if metadata.Type == file.TypeHardLink {
			original := metadata
			var adopted bool
			hardLinkOpener, adopted = adoptHardLinkInode(ft, fileCatalog, &metadata)
			if adopted {
				// keep the un-adopted description to fall back to: adoption changes the node type this
				// entry claims, which can conflict with what an earlier header put at the same path
				preAdoptionMetadata = &original
			} else {
				// trace, not warn: resolution still works via the link path, and a per-header warning
				// would be noisy on any archive that trips this
				log.WithFields("path", metadata.Path, "linkName", metadata.LinkDestination).
					Trace("hardlink names a file that is not in this layer, indexing it from its own header")
			}
		}

		// note: the tar header name is independent of surrounding structure, for example, there may be a tar header entry
		// for /some/path/to/file.txt without any entries to constituent paths (/some, /some/path, /some/path/to ).
		// This is ok, and the FileTree will account for this by automatically adding directories for non-existing
		// constituent paths. If later there happens to be a tar header entry for an already added constituent path
		// the FileNode will be updated with the new file.Reference. If there is no tar header entry for constituent
		// paths the FileTree is still structurally consistent (all paths can be iterated even though there may not have
		// been a tar header entry for part of the given path).
		//
		// In summary: the set of all FileTrees can have NON-leaf nodes that don't exist in the FileCatalog, but
		// the FileCatalog should NEVER have entries that don't appear in one (or more) FileTree(s).
		ref, err := builder.Add(metadata)
		if err != nil {
			if preAdoptionMetadata == nil {
				return err
			}
			// only an adopted hardlink can be described here as something other than what its own
			// header says, so a malformed archive should cost this one entry its adoption rather than
			// the whole image. errors on the un-adopted retry are still fatal, as they were before.
			log.WithFields("path", metadata.Path, "linkName", metadata.LinkDestination, "error", err).
				Trace("adopted hardlink conflicts with an existing entry, indexing it from its own header")

			metadata = *preAdoptionMetadata
			hardLinkOpener = nil

			ref, err = builder.Add(metadata)
			if err != nil {
				return err
			}
		}

		if size != nil {
			// what this entry contributes to the layer blob, which is its own header's data section.
			// NOT metadata.Size(), which for an adopted hardlink is the size of the file it names
			*(size) += entry.Header.Size
		}
		opener := hardLinkOpener
		if opener == nil {
			opener = func() (io.ReadCloser, error) {
				return index.Open(), nil
			}
		}
		fileCatalog.addImageReferences(ref.ID(), layerRef, opener)

		if monitor != nil {
			monitor.Increment()
		}
		return nil
	}
}

// squashfsReader implements an io.ReadCloser that reads a file from within a SquashFS filesystem.
type squashfsReader struct {
	fs.File
	backingFile *os.File
}

// newSquashfsFileReader returns a io.ReadCloser that reads the file at path within the SquashFS
// filesystem at sqfsPath.
func newSquashfsFileReader(sqfsPath, path string) (io.ReadCloser, error) {
	f, err := os.Open(sqfsPath)
	if err != nil {
		return nil, err
	}

	fsys, err := squashfs.NewReader(f)
	if err != nil {
		return nil, err
	}

	r, err := fsys.Open(path)
	if err != nil {
		return nil, err
	}

	return &squashfsReader{
		File:        r,
		backingFile: f,
	}, nil
}

// Close closes the SquashFS file as well as the backing filesystem.
func (f *squashfsReader) Close() error {
	if err := f.File.Close(); err != nil {
		return err
	}

	return f.backingFile.Close()
}

func squashfsVisitor(ft filetree.Writer, fileCatalog *FileCatalog, size *int64, layerRef *Layer, monitor *progress.Manual) file.SquashFSVisitor {
	builder := filetree.NewBuilder(ft, fileCatalog.Index)

	return func(fsys fs.FS, sqfsPath, path string) error {
		ff, err := fsys.Open(path)
		if err != nil {
			return err
		}
		defer ff.Close()

		f, ok := ff.(*squashfs.File)
		if !ok {
			return errors.New("unexpected file type from squashfs")
		}

		metadata, err := file.NewMetadataFromSquashFSFile(path, f)
		if err != nil {
			return err
		}

		fileReference, err := builder.Add(metadata)
		if err != nil {
			return err
		}

		if size != nil {
			*(size) += metadata.Size()
		}
		fileCatalog.addImageReferences(fileReference.ID(), layerRef, func() (io.ReadCloser, error) {
			return newSquashfsFileReader(sqfsPath, path)
		})

		monitor.Increment()
		return nil
	}
}

func trackReadProgress(metadata LayerMetadata) *progress.Manual {
	p := &progress.Manual{}

	bus.Publish(partybus.Event{
		Type:   event.ReadLayer,
		Source: metadata,
		Value:  progress.Monitorable(p),
	})

	return p
}
