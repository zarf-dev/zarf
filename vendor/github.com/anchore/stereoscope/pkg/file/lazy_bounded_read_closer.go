package file

import (
	"errors"
	"io"
	"os"

	"github.com/anchore/stereoscope/internal/log"
)

var _ interface {
	io.ReadCloser
	io.ReaderAt
	io.Seeker
} = (*lazyBoundedReadCloser)(nil)

// lazyBoundedReadCloser is a "lazy" read closer, allocating a file descriptor for the given path only upon the first Read() call.
// Only part of the file is allowed to be read, starting at a given position.
type lazyBoundedReadCloser struct {
	// path is the path to be opened
	path string
	// file is the active file handle for the given path
	file *os.File
	// reader is the LimitedReader that wraps the open file
	reader   *io.SectionReader
	start    int64
	size     int64
	isEOF    bool
	isClosed bool
}

// NewDeferredPartialReadCloser creates a new NewDeferredPartialReadCloser for the given path.
func newLazyBoundedReadCloser(path string, start, size int64) *lazyBoundedReadCloser {
	return &lazyBoundedReadCloser{
		path:  path,
		start: start,
		size:  size,
	}
}

// Read implements the io.Reader interface for the previously loaded path, opening the file upon the first invocation.
func (d *lazyBoundedReadCloser) Read(b []byte) (int, error) {
	if err := d.openFile(); err != nil {
		return 0, err
	}

	n, err := d.reader.Read(b)
	if err != nil && errors.Is(err, io.EOF) {
		d.isEOF = true
		d.reader = nil // IMPORTANT: this needs to be unset so opneFile continues to work when appropriate
		// we've reached the end of the file, release of the file descriptor. continue to return EOF
		if closeErr := d.file.Close(); closeErr != nil {
			log.Tracef("unable to close: %v: %v", d.path, closeErr)
		}
	}
	return n, err
}

// Close implements the io.Closer interface for the previously loaded path / opened file.
func (d *lazyBoundedReadCloser) Close() error {
	d.isClosed = true

	if d.file == nil {
		return nil
	}

	err := d.file.Close()
	if err != nil && errors.Is(err, os.ErrClosed) {
		// ignore the fact that this file has already been closed
		err = nil
	}
	d.file = nil
	d.reader = nil
	return err
}

func (d *lazyBoundedReadCloser) Seek(offset int64, whence int) (int64, error) {
	// let Read determine further EOF state
	d.isEOF = false

	if err := d.openFile(); err != nil {
		return 0, err
	}

	return d.reader.Seek(offset, whence)
}

func (d *lazyBoundedReadCloser) ReadAt(b []byte, off int64) (n int, err error) {
	// let Read determine further EOF state
	d.isEOF = false

	if err := d.openFile(); err != nil {
		return 0, err
	}

	n, err = d.reader.ReadAt(b, off)
	if err != nil && errors.Is(err, io.EOF) {
		d.isEOF = true
		d.reader = nil // IMPORTANT: this needs to be unset so opneFile continues to work when appropriate
		// we've reached the end of the file, release of the file descriptor. continue to return EOF
		if closeErr := d.file.Close(); closeErr != nil {
			log.Tracef("unable to close: %v: %v", d.path, closeErr)
		}
	}
	return n, err
}

func (d *lazyBoundedReadCloser) openFile() error {
	if d.isClosed {
		return os.ErrClosed
	}
	if d.isEOF {
		return io.EOF
	}
	if d.reader != nil {
		return nil
	}

	file, err := os.Open(d.path)
	if err != nil {
		return err
	}

	d.file = file
	d.reader = io.NewSectionReader(d.file, d.start, d.size)
	return nil
}
