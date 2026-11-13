package filetree

import (
	"github.com/anchore/stereoscope/pkg/file"
	"github.com/anchore/stereoscope/pkg/filetree/filenode"
	"github.com/anchore/stereoscope/pkg/tree"
)

type ReadWriter interface {
	Reader
	Writer
}

type Reader interface {
	AllFiles(types ...file.Type) []file.Reference
	TreeReader() tree.Reader
	PathReader
	Walker
	Copier
}

// PathReader note: the methods here split on whether they resolve one path or enumerate many. File resolves one
// path and reports a malformed link (see IsUnresolvableLink) as an error, since the caller asked about exactly
// that path. The enumerating methods skip such paths instead, because one bad link in an image must not cost the
// caller every other result. A nil result from those therefore means "absent or unresolvable", not just "absent".
type PathReader interface {
	// File resolves a single path. A malformed link is returned as an error.
	File(path file.Path, options ...LinkResolutionOption) (bool, *file.Resolution, error)
	// FilesByGlob enumerates matching paths. Matches with a malformed link are omitted rather than erroring.
	FilesByGlob(query string, options ...LinkResolutionOption) ([]file.Resolution, error)
	AllRealPaths() []file.Path
	// ListPaths enumerates a directory. A malformed link yields an empty listing rather than an error.
	ListPaths(dir file.Path) ([]file.Path, error)
	HasPath(path file.Path, options ...LinkResolutionOption) bool
}

type Copier interface {
	Copy() (ReadWriter, error)
}

type Walker interface {
	Walk(fn func(path file.Path, f filenode.FileNode) error, conditions *WalkConditions) error
}

type Writer interface {
	AddFile(realPath file.Path) (*file.Reference, error)
	AddSymLink(realPath file.Path, linkPath file.Path) (*file.Reference, error)
	AddHardLink(realPath file.Path, linkPath file.Path) (*file.Reference, error)
	AddDir(realPath file.Path) (*file.Reference, error)
	RemovePath(path file.Path) error
	Merge(upper Reader) error
}
