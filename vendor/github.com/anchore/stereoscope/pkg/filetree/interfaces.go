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

type PathReader interface {
	File(path file.Path, options ...LinkResolutionOption) (bool, *file.Resolution, error)
	FilesByGlob(query string, options ...LinkResolutionOption) ([]file.Resolution, error)
	AllRealPaths() []file.Path
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
