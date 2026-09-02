package file

import (
	"fmt"
	"io"
	"os"
)

type TarIndexVisitor func(TarIndexEntry) error

// TarIndex is a tar reader capable of O(1) fetching of entry contents after the first read.
type TarIndex struct {
	indexByName map[string][]TarIndexEntry
}

// NewTarIndex creates a new TarIndex that is already indexed.
func NewTarIndex(tarFilePath string, onIndex TarIndexVisitor) (*TarIndex, error) {
	t := &TarIndex{
		indexByName: make(map[string][]TarIndexEntry),
	}
	tarFileHandle, err := os.Open(tarFilePath)
	if err != nil {
		return nil, err
	}
	defer tarFileHandle.Close()

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
			path:         tarFileHandle.Name(),
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

	return t, IterateTar(tarFileHandle, visitor)
}

// EntriesByName fetches all TarFileEntries for the given tar header name.
func (t *TarIndex) EntriesByName(name string) ([]TarFileEntry, error) {
	if indexes, exists := t.indexByName[name]; exists {
		entries := make([]TarFileEntry, len(indexes))
		for i, index := range indexes {
			entries[i] = index.ToTarFileEntry()
		}
		return entries, nil
	}
	return nil, nil
}
