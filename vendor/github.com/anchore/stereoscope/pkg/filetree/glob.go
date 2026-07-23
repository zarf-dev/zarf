package filetree

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/anchore/stereoscope/pkg/file"
	"github.com/anchore/stereoscope/pkg/filetree/filenode"
)

// basic interface assertion
var _ fs.File = (*fileAdapter)(nil)
var _ fs.ReadDirFile = (*fileAdapter)(nil)
var _ fs.FS = (*osAdapter)(nil)
var _ fs.FileInfo = (*fileinfoAdapter)(nil)
var _ fs.DirEntry = (*fileinfoAdapter)(nil)

// fileAdapter is an object meant to implement the doublestar.File for getting Lstat results for an entire directory.
type fileAdapter struct {
	os       *osAdapter
	filetree *FileTree
	name     string
}

// Close implements io.Closer but is a nop
func (f *fileAdapter) Close() error {
	return nil
}

func (f *fileAdapter) Read([]byte) (int, error) {
	panic("not implemented")
}

func (f *fileAdapter) Stat() (fs.FileInfo, error) {
	return f.os.Stat(f.name)
}

// isInPathResolutionLoop is meant to detect if the current path doubles back on a node that is an ancestor of the
// current path.
func isInPathResolutionLoop(path string, ft *FileTree) (bool, error) {
	allPathSet := file.NewPathSet()
	allPaths := file.Path(path).AllPaths()
	for _, p := range allPaths {
		fna, err := ft.node(p, linkResolutionStrategy{
			FollowAncestorLinks: true,
			FollowBasenameLinks: true,
		})
		if err != nil {
			return false, err
		}
		if fna.HasFileNode() {
			allPathSet.Add(file.Path(fna.FileNode.ID()))
		}
	}
	// we want to allow for getting children out of the first iteration of a infinite path, but NOT allowing
	// beyond the second iteration down an infinite path.
	diff := len(allPaths) - len(allPathSet)
	return diff > 1, nil
}

// Readdir reads the contents of the directory associated with file and
// returns a slice of up to n FileInfo values, as would be returned
// by Lstat, in directory order. Subsequent calls on the same file will yield
// further FileInfos.
//
// If n > 0, Readdir returns at most n FileInfo structures. In this case, if
// Readdir returns an empty slice, it will return a non-nil error
// explaining why. At the end of a directory, the error is io.EOF.
//
// If n <= 0, Readdir returns all the FileInfo from the directory in
// a single slice. In this case, if Readdir succeeds (reads all
// the way to the end of the directory), it returns the slice and a
// nil error. If it encounters an error before the end of the
// directory, Readdir returns the FileInfo read until that point
// and a non-nil error.
//
// In order to prevent infinite recursion into paths with self-referential
// links (and similar cases) it is important that this function not return
// children for paths where we have doubled back on ourselves. The FIRST
// time through should be faithful to the return, but not the SECOND time
// around.
func (f *fileAdapter) ReadDir(n int) ([]fs.DirEntry, error) {
	if f == nil {
		return nil, os.ErrInvalid
	}
	var ret = make([]fs.DirEntry, 0)
	fna, err := f.filetree.node(file.Path(f.name), linkResolutionStrategy{
		FollowAncestorLinks: true,
		FollowBasenameLinks: true,
	})
	if err != nil {
		return ret, err
	}
	if !fna.HasFileNode() {
		return ret, nil
	}

	isInLoop, err := isInPathResolutionLoop(f.name, f.filetree)
	if err != nil || isInLoop {
		return ret, err
	}

	for idx, child := range f.filetree.tree.Children(fna.FileNode) {
		if idx == n && n != -1 {
			break
		}
		requestPath := path.Join(f.name, filepath.Base(string(child.ID())))
		r, err := f.os.Lstat(requestPath)
		if err == nil {
			// Lstat by default returns an error when the path cannot be found
			// TODO: go 1.17 will have fs.FileInfoToDirEntry helper function to prevent type assertion here
			ret = append(ret, r.(*fileinfoAdapter))
		}
	}
	return ret, nil
}

// fileAdapter is an object meant to implement the doublestar.OS for basic file queries (stat, lstat, and open).
type osAdapter struct {
	filetree                     *FileTree
	doNotFollowDeadBasenameLinks bool
}

func (a *osAdapter) ReadDir(name string) ([]fs.DirEntry, error) {
	var ret = make([]fs.DirEntry, 0)
	fna, err := a.filetree.node(file.Path(name), linkResolutionStrategy{
		FollowAncestorLinks: true,
		FollowBasenameLinks: true,
	})
	if err != nil {
		return ret, err
	}
	if !fna.HasFileNode() {
		return ret, nil
	}

	isInLoop, err := isInPathResolutionLoop(name, a.filetree)
	if err != nil || isInLoop {
		return ret, err
	}

	for _, child := range a.filetree.tree.Children(fna.FileNode) {
		requestPath := path.Join(name, filepath.Base(string(child.ID())))
		r, err := a.Lstat(requestPath)
		if err == nil {
			// Lstat by default returns an error when the path cannot be found
			// TODO: go 1.17 will have fs.FileInfoToDirEntry helper function to prevent type assertion here
			ret = append(ret, r.(*fileinfoAdapter))
		}
	}

	return ret, nil
}

// Lstat returns a FileInfo describing the named file. If the file is a symbolic link, the returned
// FileInfo describes the symbolic link. Lstat makes no attempt to follow the link.
func (a *osAdapter) Lstat(name string) (fs.FileInfo, error) {
	fna, err := a.filetree.node(file.Path(name), linkResolutionStrategy{
		FollowAncestorLinks: true,
		// Lstat by definition requires that basename symlinks are not followed
		FollowBasenameLinks:          false,
		DoNotFollowDeadBasenameLinks: false,
	})
	if err != nil {
		return &fileinfoAdapter{}, err
	}
	if !fna.HasFileNode() {
		return &fileinfoAdapter{}, os.ErrNotExist
	}

	return &fileinfoAdapter{
		VirtualPath: file.Path(name),
		Node:        *fna.FileNode,
	}, nil
}

// Open the given file path and return a doublestar.File.
func (a *osAdapter) Open(name string) (fs.File, error) {
	return &fileAdapter{
		os:       a,
		filetree: a.filetree,
		name:     name,
	}, nil
}

// Stat returns a FileInfo describing the named file.
func (a *osAdapter) Stat(name string) (fs.FileInfo, error) {
	fna, err := a.filetree.node(file.Path(name), linkResolutionStrategy{
		FollowAncestorLinks:          true,
		FollowBasenameLinks:          true,
		DoNotFollowDeadBasenameLinks: a.doNotFollowDeadBasenameLinks,
	})
	if err != nil {
		return &fileinfoAdapter{}, err
	}
	if !fna.HasFileNode() {
		return &fileinfoAdapter{}, os.ErrNotExist
	}
	return &fileinfoAdapter{
		VirtualPath: file.Path(name),
		Node:        *fna.FileNode,
	}, nil
}

// fileinfoAdapter is meant to implement the os.FileInfo interface intended only for glob searching. This does NOT
// report correct metadata for all behavior.
type fileinfoAdapter struct {
	VirtualPath file.Path
	Node        filenode.FileNode
}

func (a *fileinfoAdapter) Type() fs.FileMode {
	return a.Mode()
}

func (a *fileinfoAdapter) Info() (fs.FileInfo, error) {
	return a, nil
}

// Name base name of the file
func (a *fileinfoAdapter) Name() string {
	return a.VirtualPath.Basename()
}

// Size is a dummy return value (since it is not important for globbing). Traditionally this would be the length in
// bytes for regular files.
func (a *fileinfoAdapter) Size() int64 {
	panic("not implemented")
}

// Mode returns the file mode bits for the given file. Note that the only important bits in the bitset is the
// dir and symlink indicators; no other values can be used.
func (a *fileinfoAdapter) Mode() os.FileMode {
	// default to a typical mode value
	mode := os.FileMode(0o755)
	if a.IsDir() {
		mode |= os.ModeDir
	}
	// the underlying implementation for symlinks and hardlinks share the same semantics in the tree implementation
	// (meaning resolution is required) where as in a real file system this is taken care of by the driver
	// by making the file point to the same inode as another --making the indirection transparent to applications.
	if a.Node.FileType == file.TypeSymLink || a.Node.FileType == file.TypeHardLink {
		mode |= os.ModeSymlink
	}
	return mode
}

// ModTime returns a dummy value. Traditionally would be the modification time for the given file.
func (a *fileinfoAdapter) ModTime() time.Time {
	panic("not implemented")
}

// IsDir is an abbreviation for Mode().IsDir().
func (a *fileinfoAdapter) IsDir() bool {
	return a.Node.FileType == file.TypeDirectory
}

// Sys contains underlying data source (nothing in this case).
func (a *fileinfoAdapter) Sys() any {
	panic("not implemented")
}
