package file

import (
	"fmt"
	"io"
	"os"
	"sync"
)

type TarIndexVisitor func(TarIndexEntry) error

// TarIndex is a tar reader capable of O(1) fetching of entry contents.
//
// The tar file stays open for the life of the index and every entry reads through that one
// descriptor with positional reads (pread), so opening an entry costs no syscalls. Call Close to
// release the descriptor; an index holds one open until then.
type TarIndex struct {
	// file is written once at construction and never again: Open reads it from any goroutine, so
	// writing it here would race them. Closing it is safe by contrast, because *os.File refcounts its
	// own descriptor and fails in-flight reads with os.ErrClosed rather than reusing the fd.
	file        *os.File
	closeOnce   sync.Once
	indexByName map[string][]TarIndexEntry
}

// NewTarIndex creates a new TarIndex that is already indexed. Call Close when the index is no
// longer needed to release the tar file.
func NewTarIndex(tarFilePath string, onIndex TarIndexVisitor) (*TarIndex, error) {
	tarFileHandle, err := os.Open(tarFilePath)
	if err != nil {
		return nil, err
	}
	t := &TarIndex{
		file:        tarFileHandle,
		indexByName: make(map[string][]TarIndexEntry),
	}

	visitor := func(entry TarFileEntry) error {
		// keep track of the current location (just after reading the tar header) as this is the file content for the
		// current entry being processed.
		entrySeekPosition, err := tarFileHandle.Seek(0, io.SeekCurrent)
		if err != nil {
			return fmt.Errorf("unable to read current position in tar: %v", err)
		}

		// keep track of the header position for this entry; the current tarFileHandle position is where the entry
		// body payload starts (after the header has been read).
		indexEntry := TarIndexEntry{
			index:        t,
			sequence:     entry.Sequence,
			header:       entry.Header,
			seekPosition: entrySeekPosition,
		}
		t.indexByName[entry.Header.Name] = append(t.indexByName[entry.Header.Name], indexEntry)

		// run though the visitors
		if onIndex != nil {
			if err := onIndex(indexEntry); err != nil {
				return fmt.Errorf("failed visitor on tar indexEntry: %w", err)
			}
		}

		return nil
	}

	if err := IterateTar(tarFileHandle, visitor); err != nil {
		_ = tarFileHandle.Close()
		return nil, err
	}
	return t, nil
}

// Close releases the tar file. Reads through entry readers, whether opened before or after this call,
// will then fail with os.ErrClosed. Zero-size entries are the exception: they never touch the
// descriptor, so they keep returning a clean io.EOF.
func (t *TarIndex) Close() error {
	if t == nil || t.file == nil {
		return nil
	}
	var err error
	t.closeOnce.Do(func() { err = t.file.Close() })
	return err
}

// EntriesByName fetches all TarFileEntries for the given tar header name.
func (t *TarIndex) EntriesByName(name string) ([]TarFileEntry, error) {
	if t == nil {
		return nil, nil
	}
	if indexes, exists := t.indexByName[name]; exists {
		entries := make([]TarFileEntry, len(indexes))
		for i, index := range indexes {
			entries[i] = index.ToTarFileEntry()
		}
		return entries, nil
	}
	return nil, nil
}
