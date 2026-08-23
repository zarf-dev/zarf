package filesystem

import (
	"io"
	"io/fs"
)

// File a reference to a single file on disk
type File interface {
	io.ReadWriteSeeker
	io.Closer
	fs.File
	// io.ReaderAt
	// io.WriterAt
}
