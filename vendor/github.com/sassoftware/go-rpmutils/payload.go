/*
 * Copyright (c) SAS Institute, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package rpmutils

import (
	"errors"
	"fmt"
	"io"

	"github.com/sassoftware/go-rpmutils/cpio"
)

// TODO version 2:
// - Make PayloadReader and FileInfo regular structs
// - Promote IsLink to a method of FileInfo
// - Add Close() that must be called to clean up decompressors

// PayloadReader is used to sequentially access the file contents of a RPM payload
type PayloadReader interface {
	Next() (FileInfo, error)
	Read([]byte) (int, error)
	IsLink() bool
}

type payloadReader struct {
	stream  io.Reader
	cr      *cpio.Reader
	files   []*fileInfo
	fileMap map[string]int
	isLink  []bool
	index   int
}

func newPayloadReader(r io.Reader, files []FileInfo) *payloadReader {
	pr := &payloadReader{
		stream:  r,
		files:   make([]*fileInfo, len(files)),
		fileMap: make(map[string]int, len(files)),
		isLink:  make([]bool, len(files)),
	}
	fileSizes := make([]int64, len(files))
	lastInodes := make(map[uint64]int)
	for i, info := range files {
		fileSt := info.(*fileInfo)
		pr.files[i] = fileSt
		pr.fileMap[fileSt.name] = i
		switch fileSt.fileType() {
		case cpio.S_ISREG:
			fileSizes[i] = fileSt.Size()

			// all but the last file in a link group will have no contents. flag
			// them so we don't try to read the nonexistent payload.
			ino := fileSt.inode64()
			if lastInode, ok := lastInodes[ino]; ok && ino != 0 {
				pr.isLink[lastInode] = true
				fileSizes[lastInode] = 0
			}
			lastInodes[ino] = i
		case cpio.S_ISLNK:
			fileSizes[i] = int64(len(fileSt.linkName))
		}
	}
	pr.cr = cpio.NewReaderWithSizes(r, fileSizes)
	return pr
}

// Next returns the info of the next file in the payload. After calling Next(),
// Read() can be used to read the contents of the file. Returns io.EOF when all
// files have been consumed.
func (pr *payloadReader) Next() (FileInfo, error) {
	hdr, err := pr.cr.Next()
	if err != nil {
		// close decompressor on EOF, zstd in particular leaks goroutines otherwise
		if c, ok := pr.stream.(io.Closer); ok {
			c.Close()
		}
		return nil, err
	}
	var index int
	if hdr.IsStripped() {
		index = hdr.Index()
	} else {
		var ok bool
		name := hdr.Filename()
		if len(name) > 1 && name[0] == '.' && name[1] == '/' {
			name = name[1:]
		}
		index, ok = pr.fileMap[name]
		if !ok {
			return nil, fmt.Errorf("invalid file \"%s\" in payload", name)
		}
	}
	if index >= len(pr.files) {
		return nil, errors.New("invalid file index")
	}
	pr.index = index
	return pr.files[index], nil
}

// Read bytes from the file returned by the preceding call to Next()
func (pr *payloadReader) Read(d []byte) (int, error) {
	return pr.cr.Read(d)
}

// IsLink returns true if the current file is a hard-link with no contents. A
// subsequent file with the same FileInfo.Inode and for which IsLink() returns
// false will have the contents.
func (pr *payloadReader) IsLink() bool {
	return pr.isLink[pr.index]
}
