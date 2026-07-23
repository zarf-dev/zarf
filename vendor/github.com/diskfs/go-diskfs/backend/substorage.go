package backend

import (
	"io"
	"io/fs"
	"os"
)

type SubStorage struct {
	underlying Storage
	offset     int64
	size       int64
	path       string
}

func Sub(u Storage, offset, size int64) Storage {
	return SubStorage{
		underlying: u,
		offset:     offset,
		size:       size,
		path:       u.Path(),
	}
}

func (s SubStorage) Stat() (fs.FileInfo, error) {
	return s.underlying.Stat()
}

func (s SubStorage) Read(bytes []byte) (int, error) {
	return s.underlying.Read(bytes)
}

func (s SubStorage) Close() error {
	return s.underlying.Close()
}

func (s SubStorage) ReadAt(p []byte, off int64) (n int, err error) {
	return s.underlying.ReadAt(p, s.offset+off)
}

func (s SubStorage) Seek(offset int64, whence int) (int64, error) {
	var (
		pos int64
		err error
	)

	switch whence {
	case io.SeekStart:
		pos, err = s.underlying.Seek(offset+s.offset, io.SeekStart)
	case io.SeekCurrent:
		pos, err = s.underlying.Seek(offset, io.SeekCurrent)
	case io.SeekEnd:
		pos, err = s.underlying.Seek(s.offset+s.size+offset, io.SeekStart)
	default:
		return -1, ErrNotSuitable
	}

	if err != nil {
		return -1, err
	}

	return pos - s.offset, nil
}

func (s SubStorage) Sys() (*os.File, error) {
	return s.underlying.Sys()
}

func (s SubStorage) Writable() (WritableFile, error) {
	uw, err := s.underlying.Writable()
	if err != nil {
		return nil, err
	}
	return subWritable{
		underlying: uw,
		offset:     s.offset,
		size:       s.size,
	}, nil
}

func (s SubStorage) Path() string {
	return s.path
}

type subWritable struct {
	underlying WritableFile
	offset     int64
	size       int64
}

func (sw subWritable) Stat() (fs.FileInfo, error) {
	return sw.underlying.Stat()
}

func (sw subWritable) Read(b []byte) (int, error) {
	return sw.underlying.Read(b)
}

func (sw subWritable) Close() error {
	return sw.underlying.Close()
}

func (sw subWritable) ReadAt(p []byte, off int64) (n int, err error) {
	return sw.underlying.ReadAt(p, sw.offset+off)
}

func (sw subWritable) Seek(offset int64, whence int) (int64, error) {
	var (
		pos int64
		err error
	)

	switch whence {
	case io.SeekStart:
		pos, err = sw.underlying.Seek(offset+sw.offset, io.SeekStart)
	case io.SeekCurrent:
		pos, err = sw.underlying.Seek(offset, io.SeekCurrent)
	case io.SeekEnd:
		pos, err = sw.underlying.Seek(sw.offset+sw.size+offset, io.SeekStart)
	default:
		return -1, ErrNotSuitable
	}

	if err != nil {
		return -1, err
	}

	return pos - sw.offset, nil
}

func (sw subWritable) WriteAt(p []byte, off int64) (n int, err error) {
	return sw.underlying.WriteAt(p, sw.offset+off)
}
