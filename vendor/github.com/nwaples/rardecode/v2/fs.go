package rardecode

import (
	"fmt"
	"io"
	"io/fs"
	"path"
	"slices"
	"strings"
	"time"
)

type fileInfo struct {
	h *fileBlockHeader
}

func (f fileInfo) Name() string       { return path.Base(f.h.Name) }
func (f fileInfo) Size() int64        { return f.h.UnPackedSize }
func (f fileInfo) Mode() fs.FileMode  { return f.h.Mode() }
func (f fileInfo) ModTime() time.Time { return f.h.ModificationTime }
func (f fileInfo) IsDir() bool        { return f.h.IsDir }
func (f fileInfo) Sys() any           { return nil }

type dirEntry struct {
	h *fileBlockHeader
}

func (d dirEntry) Name() string               { return path.Base(d.h.Name) }
func (d dirEntry) IsDir() bool                { return d.h.IsDir }
func (d dirEntry) Type() fs.FileMode          { return d.h.Mode().Type() }
func (d dirEntry) Info() (fs.FileInfo, error) { return fileInfo(d), nil }

type dummyDirInfo struct {
	name string
}

func (d dummyDirInfo) Name() string       { return d.name }
func (d dummyDirInfo) Size() int64        { return 0 }
func (d dummyDirInfo) Mode() fs.FileMode  { return 0777 | fs.ModeDir }
func (d dummyDirInfo) ModTime() time.Time { return time.Time{} }
func (d dummyDirInfo) IsDir() bool        { return true }
func (d dummyDirInfo) Sys() any           { return nil }

func newDummyDirInfo(name string) dummyDirInfo {
	return dummyDirInfo{name: path.Base(name)}
}

type dummyDirEntry struct {
	name string
}

func (d dummyDirEntry) Name() string               { return d.name }
func (d dummyDirEntry) IsDir() bool                { return true }
func (d dummyDirEntry) Type() fs.FileMode          { return fs.ModeDir }
func (d dummyDirEntry) Sys() any                   { return nil }
func (d dummyDirEntry) Info() (fs.FileInfo, error) { return dummyDirInfo(d), nil }

func newDummyDirEntry(name string) dummyDirEntry {
	return dummyDirEntry{name: path.Base(name)}
}

type dirFile struct {
	name  string
	info  fs.FileInfo
	files []fs.DirEntry
	index int
}

func (df *dirFile) Read(p []byte) (int, error) { return 0, io.EOF }
func (df *dirFile) ReadByte() (byte, error)    { return 0, io.EOF }
func (df *dirFile) Stat() (fs.FileInfo, error) { return df.info, nil }
func (df *dirFile) Close() error               { return nil }

func (d *dirFile) ReadDir(n int) ([]fs.DirEntry, error) {
	if n <= 0 {
		return d.files, nil
	}
	l := d.files[d.index:]
	d.index += len(l)
	return l, nil
}

type fsNode struct {
	name   string
	blocks *fileBlockList
	files  []*fsNode
}

func (n *fsNode) isDir() bool {
	return n.blocks == nil || n.blocks.isDir()
}

func (n *fsNode) hasFileHash() bool {
	return n.blocks != nil && n.blocks.hasFileHash()
}

func (n *fsNode) firstBlock() *fileBlockHeader {
	if n.blocks == nil {
		return nil
	}
	return n.blocks.firstBlock()
}

func (n *fsNode) fileInfo() fs.FileInfo {
	h := n.firstBlock()
	if h == nil {
		return newDummyDirInfo(n.name)
	}
	return fileInfo{h: h}
}

func (n *fsNode) dirEntry() fs.DirEntry {
	h := n.firstBlock()
	if h == nil {
		return newDummyDirEntry(n.name)
	}
	return dirEntry{h: h}
}

func (n *fsNode) dirEntryList() []fs.DirEntry {
	list := make([]fs.DirEntry, len(n.files))
	for i := range list {
		list[i] = n.files[i].dirEntry()
	}
	slices.SortFunc(list, func(a, b fs.DirEntry) int {
		return strings.Compare(a.Name(), b.Name())
	})
	return list
}

// RarFS implements the fs.FS interface for accessing files in a rar archive.
type RarFS struct {
	vm    *volumeManager
	ftree map[string]*fsNode
}

func (rfs *RarFS) openArchiveFile(blocks *fileBlockList) (fs.File, error) {
	return rfs.vm.openArchiveFile(blocks)
}

// Open opens the named file.
func (rfs *RarFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	node := rfs.ftree[name]
	if node == nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	if node.isDir() {
		return &dirFile{
			name:  name,
			info:  node.fileInfo(),
			files: node.dirEntryList(),
		}, nil
	}
	f, err := rfs.openArchiveFile(node.blocks)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}
	return f, nil
}

// ReadDir reads the named directory and returns a list of directory entries sorted by filename.
func (rfs *RarFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrInvalid}
	}
	node := rfs.ftree[name]
	if node == nil {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
	}
	if !node.isDir() {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrInvalid}
	}
	return node.dirEntryList(), nil
}

// ReadFile reads the named file from the file system fs and returns its contents.
func (rfs *RarFS) ReadFile(name string) ([]byte, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "readfile", Path: name, Err: fs.ErrInvalid}
	}
	node := rfs.ftree[name]
	if node == nil {
		return nil, &fs.PathError{Op: "readfile", Path: name, Err: fs.ErrNotExist}
	}
	if node.isDir() {
		return []byte{}, nil
	}

	f, err := rfs.openArchiveFile(node.blocks)
	if err != nil {
		return nil, &fs.PathError{Op: "readfile", Path: name, Err: err}
	}
	defer f.Close()

	h := node.firstBlock()
	if h.UnKnownSize {
		return io.ReadAll(f)
	}
	buf := make([]byte, h.UnPackedSize)
	_, err = io.ReadFull(f, buf)
	return buf, err
}

/*
func (rfs *RarFS) Check(name string) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "check", Path: name, Err: fs.ErrInvalid}
	}
	node := rfs.ftree[name]
	if node == nil {
		return &fs.PathError{Op: "check", Path: name, Err: fs.ErrNotExist}
	}
	if node.isDir() {
		return &fs.PathError{Op: "check", Path: name, Err: fs.ErrInvalid}
	}
	if !node.hasFileHash() {
		return nil
	}
	f, err := rfs.openArchiveFile(node.blocks)
	if err != nil {
		return &fs.PathError{Op: "check", Path: name, Err: err}
	}
	_, err = io.Copy(io.Discard, f)
	if err != nil {
		return &fs.PathError{Op: "check", Path: name, Err: err}
	}
	return nil
}
*/

// Stat returns a FileInfo describing the named file from the filesystem.
func (rfs *RarFS) Stat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrInvalid}
	}
	node := rfs.ftree[name]
	if node == nil {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
	}
	return node.fileInfo(), nil
}

// Sub returns an FS corresponding to the subtree rooted at fsys's dir.
func (rfs *RarFS) Sub(dir string) (fs.FS, error) {
	if dir == "." {
		return rfs, nil
	}
	if !fs.ValidPath(dir) {
		return nil, &fs.PathError{Op: "sub", Path: dir, Err: fs.ErrInvalid}
	}
	node := rfs.ftree[dir]
	if node == nil {
		return nil, &fs.PathError{Op: "sub", Path: dir, Err: fs.ErrNotExist}
	}
	if !node.isDir() {
		return nil, &fs.PathError{Op: "sub", Path: dir, Err: fs.ErrInvalid}
	}
	newFS := &RarFS{
		ftree: map[string]*fsNode{
			".": {name: ".", files: node.files},
		},
		vm: rfs.vm,
	}
	prefix := dir + "/"
	for k, v := range rfs.ftree {
		if strings.HasPrefix(k, prefix) {
			newFS.ftree[strings.TrimPrefix(k, prefix)] = v
		}
	}
	return newFS, nil
}

func listFileBlocks(name string, opts []Option) (*volumeManager, []*fileBlockList, error) {
	options := getOptions(opts)
	if options.openCheck {
		options.skipCheck = false
	}
	v, err := openVolume(name, options)
	if err != nil {
		return nil, nil, err
	}
	pr := newPackedFileReader(v, options)
	defer v.Close()

	fileBlocks := []*fileBlockList{}
	for {
		blocks, err := pr.nextFile()
		if err != nil {
			if err == io.EOF {
				return v.vm, fileBlocks, nil
			}
			return nil, nil, err
		}
		fileBlocks = append(fileBlocks, blocks)
		if options.openCheck && blocks.hasFileHash() {
			f, err := pr.newArchiveFile(blocks)
			if err != nil {
				return nil, nil, err
			}
			_, err = io.Copy(io.Discard, f)
			if err != nil {
				return nil, nil, err
			}
		}
	}
}

func OpenFS(name string, opts ...Option) (*RarFS, error) {
	vm, fileBlocks, err := listFileBlocks(name, opts)
	if err != nil {
		return nil, err
	}

	rfs := &RarFS{
		ftree: map[string]*fsNode{},
		vm:    vm,
	}
	for _, blocks := range fileBlocks {
		h := blocks.firstBlock()
		fname := strings.TrimPrefix(path.Clean(h.Name), "/")
		if !fs.ValidPath(fname) {
			return nil, fmt.Errorf("rardecode: archived file has invalid path: %s", fname)
		}
		node := rfs.ftree[fname]
		if node != nil {
			if node.blocks == nil || node.firstBlock().Version < h.Version {
				node.blocks = blocks
			}
			continue
		}
		rfs.ftree[fname] = &fsNode{blocks: blocks}
		prev := rfs.ftree[fname]
		// add parent file nodes
		for fname != "." {
			fname = path.Dir(fname)
			node = rfs.ftree[fname]
			if node != nil {
				node.files = append(node.files, prev)
				break
			}
			rfs.ftree[fname] = &fsNode{
				name:  fname,
				files: []*fsNode{prev},
			}
			prev = rfs.ftree[fname]
		}
	}
	return rfs, nil
}
