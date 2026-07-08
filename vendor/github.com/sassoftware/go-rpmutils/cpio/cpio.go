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

package cpio

import (
	"errors"
	"fmt"
	"io"
)

// TRAILER is the filename found on the last entry of a cpio archive
const TRAILER = "TRAILER!!!"

// ErrStrippedHeader indicates that a RPM-style archive was read without calling SetFileSizes()
var ErrStrippedHeader = errors.New("invalid cpio header: rpm-style stripped cpio requires supplemental size info")

// CpioEntry points to a single file within a cpio stream
type CpioEntry struct {
	Header  *Cpio_newc_header
	payload *file_stream
}

// CpioStream reads file metadata and contents from a cpio archive
type CpioStream struct {
	stream  *countingReader
	nextPos int64
	sizes   []int64
}

type countingReader struct {
	stream io.Reader
	curPos int64
}

// NewCpioStream starts reading files from a cpio archive
func NewCpioStream(stream io.Reader) *CpioStream {
	return &CpioStream{
		stream: &countingReader{
			stream: stream,
			curPos: 0,
		},
		nextPos: 0,
	}
}

// SetFileSizes provides supplemental file size info so that RPMs with files > 4GiB can be read
func (cs *CpioStream) SetFileSizes(sizes []int64) {
	cs.sizes = sizes
}

// ReadNextEntry returns the metadata of the next file in the archive.
//
// The final file in the archive can be detected by checking for a Filename of TRAILER.
func (cs *CpioStream) ReadNextEntry() (*CpioEntry, error) {
	if cs.nextPos != cs.stream.curPos {
		_, err := cs.stream.Seek(cs.nextPos-cs.stream.curPos, 1)
		if err != nil {
			return nil, err
		}
	}

	// Read header
	hdr, err := readHeader(cs.stream)
	if err != nil {
		return nil, err
	} else if hdr.stripped {
		return cs.readStrippedEntry(hdr)
	}

	// Read filename
	buf := make([]byte, hdr.namesize)
	if _, err = io.ReadFull(cs.stream, buf); err != nil {
		return nil, err
	}

	filename := string(buf[:len(buf)-1])

	offset := pad(newcHeaderLength+int(hdr.namesize)) - newcHeaderLength - int(hdr.namesize)

	if offset > 0 {
		_, err := cs.stream.Seek(int64(offset), 1)
		if err != nil {
			return nil, err
		}
	}

	// Find the next entry
	cs.nextPos = pad64(cs.stream.curPos + int64(hdr.filesize))

	// Find the payload
	payload, err := newFileStream(cs.stream, int64(hdr.filesize))
	if err != nil {
		return nil, err
	}

	// Create then entry
	hdr.filename = filename
	entry := CpioEntry{
		Header:  hdr,
		payload: payload,
	}

	return &entry, nil
}

func (cs *CpioStream) readStrippedEntry(hdr *Cpio_newc_header) (*CpioEntry, error) {
	// magic has already been read
	if cs.sizes == nil {
		return nil, ErrStrippedHeader
	} else if hdr.index >= len(cs.sizes) {
		return nil, fmt.Errorf("stripped cpio refers to invalid file index %d", hdr.index)
	}
	size := cs.sizes[hdr.index]
	cs.nextPos = pad64(cs.stream.curPos + size)
	payload, err := newFileStream(cs.stream, size)
	if err != nil {
		return nil, err
	}
	return &CpioEntry{Header: hdr, payload: payload}, nil
}

func (cr *countingReader) Read(p []byte) (n int, err error) {
	n, err = cr.stream.Read(p)
	cr.curPos += int64(n)
	return
}

func (cr *countingReader) Seek(offset int64, whence int) (int64, error) {
	if whence != 1 {
		return 0, fmt.Errorf("only seeking from current location supported")
	}
	if offset == 0 {
		return cr.curPos, nil
	}
	b := make([]byte, offset)
	n, err := io.ReadFull(cr, b)
	if err != nil && err != io.EOF {
		return 0, err
	}
	return int64(n), nil
}

func pad(num int) int {
	return num + 3 - (num+3)%4
}

func pad64(num int64) int64 {
	return num + 3 - (num+3)%4
}
