package file

import (
	"io/fs"
	"os"

	"github.com/sylabs/squashfs"
)

// SquashFSVisitor is the type of the function called by WalkSquashFS to visit each file or
// directory.
//
// The sqfsPath argument contains the path to the SquashFS filesystem that was passed to
// WalkSquashFS. The filePath argument contains the full path of the file or directory within the
// SquashFS filesystem.
//
// The error result returned by the function controls how WalkSquashFS continues. If the function
// returns the special value fs.SkipDir, WalkSquashFS skips the current directory (filePath if
// d.IsDir() is true, otherwise filePath's parent directory). Otherwise, if the function returns a
// non-nil error, WalkSquashFS stops entirely and returns that error.
type SquashFSVisitor func(fsys fs.FS, sqfsPath, filePath string) error

// WalkSquashFS walks the file tree within the SquashFS filesystem at sqfsPath, calling fn for each
// file or directory in the tree, including root.
func WalkSquashFS(sqfsPath string, fn SquashFSVisitor) error {
	f, err := os.Open(sqfsPath)
	if err != nil {
		return err
	}
	defer f.Close()

	fsys, err := squashfs.NewReader(f)
	if err != nil {
		return err
	}

	return fs.WalkDir(fsys, ".", walkDir(fsys, sqfsPath, fn))
}

// walkDir returns a fs.WalkDirFunc bound to fn.
func walkDir(fsys fs.FS, sqfsPath string, fn SquashFSVisitor) fs.WalkDirFunc {
	return func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		return fn(fsys, sqfsPath, path)
	}
}
