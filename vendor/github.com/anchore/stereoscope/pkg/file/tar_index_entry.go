package file

import (
	"archive/tar"
	"io"
)

type TarIndexEntry struct {
	path         string
	sequence     int64
	header       tar.Header
	seekPosition int64
}

func (t *TarIndexEntry) ToTarFileEntry() TarFileEntry {
	return TarFileEntry{
		Sequence: t.sequence,
		Header:   t.header,
		Reader:   t.Open(),
	}
}

func (t *TarIndexEntry) Open() io.ReadCloser {
	return newLazyBoundedReadCloser(t.path, t.seekPosition, t.header.Size)
}
