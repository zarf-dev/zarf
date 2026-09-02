package image

import (
	"fmt"
	"io"

	"github.com/anchore/stereoscope/pkg/file"
	"github.com/anchore/stereoscope/pkg/filetree"
)

// fetchReaderByPath is a common helper function for resolving the file contents for a path from the file
// catalog relative to the given tree.
func fetchReaderByPath(ft filetree.Reader, fileCatalog FileCatalogReader, path file.Path) (io.ReadCloser, error) {
	exists, refVia, err := ft.File(path, filetree.FollowBasenameLinks)
	if err != nil {
		return nil, err
	}
	if !exists && refVia == nil || refVia.Reference == nil {
		return nil, fmt.Errorf("could not find file path in Tree: %s", path)
	}

	reader, err := fileCatalog.Open(*refVia.Reference)
	if err != nil {
		return nil, err
	}
	return reader, nil
}
