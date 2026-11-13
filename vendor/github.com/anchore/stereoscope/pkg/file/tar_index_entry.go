package file

import (
	"archive/tar"
	"io"
)

var _ interface {
	io.ReadCloser
	io.ReaderAt
	io.Seeker
} = (*sectionReadCloser)(nil)

type TarIndexEntry struct {
	index        *TarIndex
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

// Open returns a reader over this entry's contents. It supports Read, Seek and ReadAt.
//
// Each call returns an independent reader with its own offset, so readers for the same entry and for
// different entries can be used concurrently. A single reader must not be shared across goroutines:
// Read and Seek mutate its offset, only ReadAt is stateless.
//
// Close on the returned reader is not observable. The index owns the descriptor and releases it in
// TarIndex.Close, after which reads through any reader from this index report os.ErrClosed.
func (t *TarIndexEntry) Open() io.ReadCloser {
	return &sectionReadCloser{sr: io.NewSectionReader(t.index.file, t.seekPosition, t.header.Size)}
}

// sectionReadCloser is a positional view onto the index's shared tar file that satisfies
// io.ReadCloser for callers that close what they open.
//
// The io.SectionReader is held in a field rather than embedded on purpose: embedding promotes
// SectionReader.Outer, which would hand every caller the index's live *os.File along with this
// entry's absolute offset, enough to read the whole layer tar and to close the descriptor out from
// under every other reader.
type sectionReadCloser struct {
	sr *io.SectionReader
}

func (s *sectionReadCloser) Read(p []byte) (int, error) { return s.sr.Read(p) }

func (s *sectionReadCloser) ReadAt(p []byte, off int64) (int, error) { return s.sr.ReadAt(p, off) }

func (s *sectionReadCloser) Seek(offset int64, whence int) (int64, error) {
	return s.sr.Seek(offset, whence)
}

func (s *sectionReadCloser) Close() error { return nil }
