package squashfs

import (
	"errors"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/sylabs/squashfs/internal/routinemanager"
	squashfslow "github.com/sylabs/squashfs/low"
	"github.com/sylabs/squashfs/low/data"
	"github.com/sylabs/squashfs/low/inode"
)

// File represents a file inside a squashfs archive.
type File struct {
	full     *data.FullReader
	rdr      *data.Reader
	parent   *FS
	r        *Reader
	b        squashfslow.FileBase
	dirsRead int
}

// Creates a new *File from the given *squashfs.Base
func (r *Reader) FileFromBase(b squashfslow.FileBase, parent *FS) *File {
	return &File{
		b:      b,
		parent: parent,
		r:      r,
	}
}

func (f *File) FS() (*FS, error) {
	if !f.IsDir() {
		return nil, errors.New("not a directory")
	}
	d, err := f.b.ToDir(&f.r.Low)
	if err != nil {
		return nil, err
	}
	return &FS{d: d, parent: f.parent, r: f.r}, nil
}

// Closes the underlying readers.
// Further calls to Read and WriteTo will re-create the readers.
// Never returns an error.
func (f *File) Close() error {
	if f.rdr != nil {
		return f.rdr.Close()
	}
	f.rdr = nil
	f.full = nil
	return nil
}

// Returns the file the symlink points to.
// If the file isn't a symlink, or points to a file outside the archive, returns nil.
func (f *File) GetSymlinkFile() fs.File {
	if !f.IsSymlink() {
		return nil
	}
	if filepath.IsAbs(f.SymlinkPath()) {
		return nil
	}
	fil, err := f.parent.Open(f.SymlinkPath())
	if err != nil {
		return nil
	}
	return fil
}

// Returns whether the file is a directory.
func (f *File) IsDir() bool {
	return f.b.IsDir()
}

// Returns whether the file is a regular file.
func (f *File) IsRegular() bool {
	return f.b.IsRegular()
}

// Returns whether the file is a symlink.
func (f *File) IsSymlink() bool {
	return f.b.Inode.Type == inode.Sym || f.b.Inode.Type == inode.ESym
}

func (f *File) Mode() fs.FileMode {
	return f.b.Inode.Mode()
}

// Read reads the data from the file. Only works if file is a normal file.
func (f *File) Read(b []byte) (int, error) {
	if !f.IsRegular() {
		return 0, errors.New("file is not a regular file")
	}
	if f.rdr == nil {
		err := f.initializeReaders()
		if err != nil {
			return 0, err
		}
	}
	return f.rdr.Read(b)
}

// ReadDir returns n fs.DirEntry's that's contained in the File (if it's a directory).
// If n <= 0 all fs.DirEntry's are returned.
func (f *File) ReadDir(n int) ([]fs.DirEntry, error) {
	if !f.IsDir() {
		return nil, errors.New("file is not a directory")
	}
	d, err := f.b.ToDir(&f.r.Low)
	if err != nil {
		return nil, err
	}
	start, end := 0, len(d.Entries)
	if n > 0 {
		start, end = f.dirsRead, f.dirsRead+n
		if end > len(d.Entries) {
			end = len(d.Entries)
			err = io.EOF
		}
	}
	var out []fs.DirEntry
	var fi fileInfo
	for _, e := range d.Entries[start:end] {
		fi, err = f.r.newFileInfo(e)
		if err != nil {
			f.dirsRead += len(out)
			return out, err
		}
		out = append(out, fs.FileInfoToDirEntry(fi))
	}
	f.dirsRead += len(out)
	return out, err
}

// Returns the file's fs.FileInfo
func (f *File) Stat() (fs.FileInfo, error) {
	return newFileInfo(f.b.Name, &f.b.Inode), nil
}

// SymlinkPath returns the symlink's target path. Is the File isn't a symlink, returns an empty string.
func (f *File) SymlinkPath() string {
	switch f.b.Inode.Type {
	case inode.Sym:
		return string(f.b.Inode.Data.(inode.Symlink).Target)
	case inode.ESym:
		return string(f.b.Inode.Data.(inode.ESymlink).Target)
	}
	return ""
}

// Writes all data from the file to the given writer in a multi-threaded manner.
// The underlying reader is separate
func (f *File) WriteTo(w io.Writer) (int64, error) {
	if !f.IsRegular() {
		return 0, errors.New("file is not a regular file")
	}
	if f.full == nil {
		err := f.initializeReaders()
		if err != nil {
			return 0, err
		}
	}
	return f.full.WriteTo(w)
}

func (f *File) initializeReaders() error {
	var err error
	f.rdr, f.full, err = f.b.GetRegFileReaders(&f.r.Low)
	return err
}

func (f *File) deviceDevices() (maj uint32, min uint32) {
	var dev uint32
	switch f.b.Inode.Type {
	case inode.Char, inode.Block:
		dev = f.b.Inode.Data.(inode.Device).Dev
	case inode.EChar, inode.EBlock:
		dev = f.b.Inode.Data.(inode.EDevice).Dev
	}
	return dev >> 8, dev & 0x000FF
}

func (f *File) path() string {
	if f.parent == nil {
		return f.b.Name
	}
	return filepath.Join(f.parent.path(), f.b.Name)
}

// Extract the file to the given folder. If the file is a folder, the folder's contents will be extracted to the folder.
// Uses default extraction options.
func (f *File) Extract(folder string) error {
	return f.ExtractWithOptions(folder, DefaultOptions())
}

// Extract the file to the given folder. If the file is a folder, the folder's contents will be extracted to the folder.
// Allows setting various extraction options via ExtractionOptions.
func (f *File) ExtractWithOptions(path string, op *ExtractionOptions) error {
	if op.manager == nil {
		op.manager = routinemanager.NewManager(op.SimultaneousFiles)
		if op.LogOutput != nil {
			log.SetOutput(op.LogOutput)
		}
		err := os.MkdirAll(path, 0777)
		if err != nil {
			if op.Verbose {
				log.Println("Failed to create initial directory", path)
			}
			return err
		}
	}
	switch f.b.Inode.Type {
	case inode.Dir, inode.EDir:
		d, err := f.b.ToDir(&f.r.Low)
		if err != nil {
			if op.Verbose {
				log.Println("Failed to create squashfs.Directory for", path)
			}
			return errors.Join(errors.New("failed to create squashfs.Directory: "+path), err)
		}
		errChan := make(chan error, len(d.Entries))
		for i := range d.Entries {
			b, err := f.r.Low.BaseFromEntry(d.Entries[i])
			if err != nil {
				if op.Verbose {
					log.Println("Failed to get squashfs.Base from entry for", path)
				}
				return errors.Join(errors.New("failed to get base from entry: "+path), err)
			}
			go func(b squashfslow.FileBase, path string) {
				i := op.manager.Lock()
				if b.IsDir() {
					extDir := filepath.Join(path, b.Name)
					err = os.Mkdir(extDir, 0777)
					op.manager.Unlock(i)
					if err != nil {
						if op.Verbose {
							log.Println("Failed to create directory", path)
						}
						errChan <- errors.Join(errors.New("failed to create directory: "+path), err)
						return
					}
					err = f.r.FileFromBase(b, f.r.FSFromDirectory(d, f.parent)).ExtractWithOptions(extDir, op)
					if err != nil {
						if op.Verbose {
							log.Println("Failed to extract directory", path)
						}
						errChan <- errors.Join(errors.New("failed to extract directory: "+path), err)
						return
					}
					errChan <- nil
				} else {
					fil := f.r.FileFromBase(b, f.r.FSFromDirectory(d, f.parent))
					err = fil.ExtractWithOptions(path, op)
					op.manager.Unlock(i)
					fil.Close()
					errChan <- err
				}
			}(b, path)
		}
		var errCache []error
		for range d.Entries {
			err := <-errChan
			if err != nil {
				errCache = append(errCache, err)
			}
		}
		if len(errCache) > 0 {
			return errors.Join(errors.New("failed to extract folder: "+path), errors.Join(errCache...))
		}
	case inode.Fil, inode.EFil:
		path = filepath.Join(path, f.b.Name)
		outFil, err := os.Create(path)
		if err != nil {
			if op.Verbose {
				log.Println("Failed to create file", path)
			}
			return errors.Join(errors.New("failed to create file: "+path), err)
		}
		defer outFil.Close()
		full, err := f.b.GetFullReader(&f.r.Low)
		if err != nil {
			if op.Verbose {
				log.Println("Failed to create full reader for", path)
			}
			return errors.Join(errors.New("failed to create full reader: "+path), err)
		}
		full.SetGoroutineLimit(op.ExtractionRoutines)
		_, err = full.WriteTo(outFil)
		if err != nil {
			if op.Verbose {
				log.Println("Failed to write file", path)
			}
			return errors.Join(errors.New("failed to write file: "+path), err)
		}
	case inode.Sym, inode.ESym:
		symPath := f.SymlinkPath()
		if op.DereferenceSymlink {
			filTmp := f.GetSymlinkFile()
			if filTmp == nil {
				if op.Verbose {
					log.Println("Failed to get symlink's file:", f.path())
				}
				return errors.New("failed to get symlink's file")
			}
			fil := filTmp.(*File)
			fil.b.Name = f.b.Name
			err := fil.ExtractWithOptions(path, op)
			if err != nil {
				if op.Verbose {
					log.Println("Failed to extract symlink's file:", filepath.Join(path, f.b.Name))
				}
				return errors.Join(errors.New("failed to extract symlink's file: "+path), err)
			}
		} else {
			if op.UnbreakSymlink {
				filTmp := f.GetSymlinkFile()
				if filTmp == nil {
					if op.Verbose {
						log.Println("Failed to get symlink's file:", f.path())
					}
					return errors.New("failed to get symlink's file")
				}
				extractLoc := filepath.Join(path, filepath.Dir(symPath))
				fil := filTmp.(*File)
				err := fil.ExtractWithOptions(extractLoc, op)
				if err != nil {
					if op.Verbose {
						log.Println("Error while extracting", fil.path(), "to make sure symlink at", f.path(), "is unbroken")
					}
					return errors.Join(errors.New("failed to extract symlink's file: "+extractLoc), err)
				}
			}
			path = filepath.Join(path, f.b.Name)
			err := os.Symlink(f.SymlinkPath(), path)
			if err != nil {
				if op.Verbose {
					log.Println("Failed to create symlink:", path)
				}
				return errors.Join(errors.New("failed to create symlink: "+path), err)
			}
		}
	case inode.Char, inode.EChar, inode.Block, inode.EBlock, inode.Fifo, inode.EFifo:
		if runtime.GOOS == "windows" {
			if op.Verbose {
				log.Println(f.path(), "ignored. A device link and can't be created on Windows.")
			}
			return nil
		}
		_, err := exec.LookPath("mknod")
		if err != nil {
			if op.Verbose {
				log.Println("mknot command not found, cannot create device link for", f.path())
			}
			return errors.Join(errors.New("mknot command not found"), err)
		}
		path = filepath.Join(path, f.b.Name)
		var typ string
		switch f.b.Inode.Type {
		case inode.Char, inode.EChar:
			typ = "c"
		case inode.Block, inode.EBlock:
			typ = "b"
		default: //Fifo IPC
			if runtime.GOOS == "darwin" {
				if op.Verbose {
					log.Println(f.path(), "ignored. A Fifo file and can't be created on Darwin.")
				}
				return nil
			}
			typ = "p"
		}
		cmd := exec.Command("mknod", path, typ)
		if typ != "p" {
			maj, min := f.deviceDevices()
			cmd.Args = append(cmd.Args, strconv.Itoa(int(maj)), strconv.Itoa(int(min)))
		}
		if op.Verbose {
			cmd.Stdout = op.LogOutput
			cmd.Stderr = op.LogOutput
		}
		err = cmd.Run()
		if err != nil {
			if op.Verbose {
				log.Println("Error while running mknod for", path)
			}
			return errors.Join(errors.New("error while running mknod for "+path), err)
		}
	case inode.Sock, inode.ESock:
		if op.Verbose {
			log.Println(f.path(), "ignored since it's a socket file.")
		}
		return nil
	default:
		return errors.New("Unsupported file type. Inode type: " + strconv.Itoa(int(f.b.Inode.Type)))
	}
	if op.Verbose {
		log.Println(f.path(), "extracted to", path)
	}
	if op.IgnorePerm {
		return nil
	}
	uid, err := f.b.Uid(&f.r.Low)
	if err != nil {
		if op.Verbose {
			log.Println("Failed to get uid for", path)
			log.Println(err)
		}
		return nil
	}
	gid, err := f.b.Gid(&f.r.Low)
	if err != nil {
		if op.Verbose {
			log.Println("Failed to get gid for", path)
			log.Println(err)
		}
		return nil
	}
	os.Chmod(path, f.Mode())
	os.Chown(path, int(uid), int(gid))
	return nil
}
