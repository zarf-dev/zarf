package squashfs

import (
	"io"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strings"

	squashfslow "github.com/sylabs/squashfs/low"
	"github.com/sylabs/squashfs/low/directory"
)

// FS is a fs.FS representation of a squashfs directory.
// Implements fs.GlobFS, fs.ReadDirFS, fs.ReadFileFS, fs.StatFS, and fs.SubFS
type FS struct {
	r      *Reader
	parent *FS
	d      squashfslow.Directory
}

// Creates a new *FS from the given squashfs.directory
func (r *Reader) FSFromDirectory(d squashfslow.Directory, parent *FS) *FS {
	return &FS{
		d:      d,
		r:      r,
		parent: parent,
	}
}

// Glob returns the name of the files at the given pattern.
// All paths are relative to the FS.
// Uses filepath.Match to compare names.
func (f *FS) Glob(pattern string) (out []string, err error) {
	pattern = filepath.Clean(pattern)
	if !fs.ValidPath(pattern) {
		return nil, &fs.PathError{
			Op:   "glob",
			Path: pattern,
			Err:  fs.ErrInvalid,
		}
	}
	split := strings.Split(pattern, "/")
	for i := range f.d.Entries {
		if match, _ := path.Match(split[0], f.d.Entries[i].Name); match {
			if len(split) == 1 {
				out = append(out, f.d.Entries[i].Name)
				continue
			}
			sub, err := f.Sub(split[0])
			if err != nil {
				if pathErr, ok := err.(*fs.PathError); ok {
					if pathErr.Err == fs.ErrNotExist {
						continue
					}
					pathErr.Op = "glob"
					pathErr.Path = pattern
					return nil, pathErr
				}
				return nil, &fs.PathError{
					Op:   "glob",
					Path: pattern,
					Err:  err,
				}
			}
			subGlob, err := sub.(fs.GlobFS).Glob(strings.Join(split[1:], "/"))
			if err != nil {
				if pathErr, ok := err.(*fs.PathError); ok {
					if pathErr.Err == fs.ErrNotExist {
						continue
					}
					pathErr.Op = "glob"
					pathErr.Path = pattern
					return nil, pathErr
				}
				return nil, &fs.PathError{
					Op:   "glob",
					Path: pattern,
					Err:  err,
				}
			}
			for i := range subGlob {
				subGlob[i] = f.d.Name + "/" + subGlob[i]
			}
			out = append(out, subGlob...)
		}
	}
	return
}

// Opens the file at name. Returns a *File as an fs.File.
func (f *FS) Open(name string) (fs.File, error) {
	name = filepath.Clean(name)
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{
			Op:   "open",
			Path: name,
			Err:  fs.ErrInvalid,
		}
	}
	if name == "." || name == "" {
		return f.File(), nil
	}
	split := strings.Split(name, "/")
	if split[0] == ".." {
		if f.parent == nil { // root directory
			return nil, &fs.PathError{
				Op:   "open",
				Path: name,
				Err:  fs.ErrNotExist,
			}
		} else {
			return f.parent.Open(strings.Join(split[1:], "/"))
		}
	}
	i, found := slices.BinarySearchFunc(f.d.Entries, split[0], func(e directory.Entry, name string) int {
		return strings.Compare(e.Name, name)
	})
	if !found {
		return nil, &fs.PathError{
			Op:   "open",
			Path: name,
			Err:  fs.ErrNotExist,
		}
	}
	b, err := f.r.Low.BaseFromEntry(f.d.Entries[i])
	if err != nil {
		return nil, err
	}
	if len(split) == 1 {
		return &File{
			b:      b,
			r:      f.r,
			parent: f,
		}, nil
	}
	if !b.IsDir() {
		return nil, &fs.PathError{
			Op:   "open",
			Path: name,
			Err:  fs.ErrNotExist,
		}
	}
	d, err := b.ToDir(&f.r.Low)
	if err != nil {
		return nil, err
	}
	return f.r.FSFromDirectory(d, f).Open(strings.Join(split[1:], "/"))
}

// Returns all DirEntry's for the directory at name.
// If name is not a directory, returns an error.
func (f *FS) ReadDir(name string) ([]fs.DirEntry, error) {
	name = filepath.Clean(name)
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{
			Op:   "readdir",
			Path: name,
			Err:  fs.ErrInvalid,
		}
	}
	if name == "." || name == "" {
		return f.File().ReadDir(-1)
	}
	fil, err := f.Open(name)
	if err != nil {
		return nil, err
	}
	return fil.(*File).ReadDir(-1)
}

// Returns the contents of the file at name.
func (f *FS) ReadFile(name string) (out []byte, err error) {
	name = filepath.Clean(name)
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{
			Op:   "readfile",
			Path: name,
			Err:  fs.ErrInvalid,
		}
	}
	if name == "." || name == "" {
		return nil, fs.ErrInvalid
	}
	fil, err := f.Open(name)
	if err != nil {
		return nil, err
	}
	if !fil.(*File).IsRegular() {
		return nil, fs.ErrInvalid
	}
	return io.ReadAll(fil)
}

// Returns the fs.FileInfo for the file at name.
func (f *FS) Stat(name string) (fs.FileInfo, error) {
	name = filepath.Clean(name)
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{
			Op:   "stat",
			Path: name,
			Err:  fs.ErrInvalid,
		}
	}
	if name == "." || name == "" {
		return f.File().Stat()
	}
	fil, err := f.Open(name)
	if err != nil {
		return nil, err
	}
	return fil.(*File).Stat()
}

// Returns the FS at dir
func (f *FS) Sub(dir string) (fs.FS, error) {
	dir = filepath.Clean(dir)
	if !fs.ValidPath(dir) {
		return nil, &fs.PathError{
			Op:   "dir",
			Path: dir,
			Err:  fs.ErrInvalid,
		}
	}
	if dir == "." || dir == "" {
		return f, nil
	}
	fil, err := f.Open(dir)
	if err != nil {
		return nil, err
	}
	if !fil.(*File).IsDir() {
		return nil, &fs.PathError{
			Op:   "dir",
			Path: dir,
			Err:  fs.ErrInvalid,
		}
	}
	return fil.(*File).FS()
}

// Extract the FS to the given folder. If the file is a folder, the folder's contents will be extracted to the folder.
// Uses default extraction options.
func (f *FS) Extract(folder string) error {
	return f.File().Extract(folder)
}

// Extract the FS to the given folder. If the file is a folder, the folder's contents will be extracted to the folder.
// Allows setting various extraction options via ExtractionOptions.
func (f *FS) ExtractWithOptions(folder string, op *ExtractionOptions) error {
	return f.File().ExtractWithOptions(folder, op)
}

// Returns the FS as a *File
func (f *FS) File() *File {
	return &File{
		b:      f.d.FileBase,
		parent: f.parent,
		r:      f.r,
	}
}

func (f *FS) path() string {
	if f.parent == nil {
		return f.d.Name
	}
	return filepath.Join(f.parent.path(), f.d.Name)
}
