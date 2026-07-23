package filenode

import (
	"path"
	"path/filepath"

	"github.com/anchore/stereoscope/pkg/file"
	"github.com/anchore/stereoscope/pkg/tree/node"
)

type FileNode struct {
	RealPath  file.Path // all constituent paths cannot have links (the base may be a link however)
	FileType  file.Type
	LinkPath  file.Path // a relative or absolute path to another file
	Reference *file.Reference
}

func NewDir(p file.Path, ref *file.Reference) *FileNode {
	return &FileNode{
		RealPath:  p,
		FileType:  file.TypeDirectory,
		Reference: ref,
	}
}

func NewFile(p file.Path, ref *file.Reference) *FileNode {
	return &FileNode{
		RealPath:  p,
		FileType:  file.TypeRegular,
		Reference: ref,
	}
}

func NewSymLink(p, linkPath file.Path, ref *file.Reference) *FileNode {
	return &FileNode{
		RealPath:  p,
		FileType:  file.TypeSymLink,
		LinkPath:  linkPath,
		Reference: ref,
	}
}

func NewHardLink(p, linkPath file.Path, ref *file.Reference) *FileNode {
	// hard link MUST be interpreted as an absolute path
	linkPath = file.Path(path.Clean(file.DirSeparator + string(linkPath)))
	return &FileNode{
		RealPath:  p,
		FileType:  file.TypeHardLink,
		LinkPath:  linkPath,
		Reference: ref,
	}
}

func (n *FileNode) ID() node.ID {
	return IDByPath(n.RealPath)
}

func (n *FileNode) Copy() node.Node {
	return &FileNode{
		RealPath:  n.RealPath,
		FileType:  n.FileType,
		LinkPath:  n.LinkPath,
		Reference: n.Reference,
	}
}

func (n *FileNode) IsLink() bool {
	return n.FileType == file.TypeHardLink || n.FileType == file.TypeSymLink
}

func IDByPath(p file.Path) node.ID {
	return node.ID(p)
}

func (n *FileNode) RenderLinkDestination() file.Path {
	if !n.IsLink() {
		return ""
	}

	if n.LinkPath.IsAbsolutePath() {
		// use links with absolute paths blindly
		return n.LinkPath
	}

	// resolve relative link paths
	var parentDir string
	parentDir, _ = filepath.Split(string(n.RealPath)) // TODO: alex: should this be path.Split, not filepath.Split?

	// assemble relative link path by normalizing: "/cur/dir/../file1.txt" --> "/cur/file1.txt"
	return file.Path(path.Clean(path.Join(parentDir, string(n.LinkPath))))
}
