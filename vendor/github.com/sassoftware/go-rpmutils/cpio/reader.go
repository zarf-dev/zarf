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

import "io"

// Reader accesses a cpio archive stream using a simple interface similar to the archive/tar package
type Reader struct {
	stream *CpioStream
	cur    *CpioEntry
}

// NewReader starts reading a cpio archive stream
func NewReader(stream io.Reader) *Reader {
	return NewReaderWithSizes(stream, nil)
}

// NewReaderWithSizes starts reading a stripped cpio archive from a RPM payload using the provided file sizes
func NewReaderWithSizes(stream io.Reader, sizes []int64) *Reader {
	cstream := NewCpioStream(stream)
	cstream.SetFileSizes(sizes)
	return &Reader{
		stream: cstream,
		cur:    nil,
	}
}

// Next returns the metadata of the next file in the archive, which can then be
// read with Read().
//
// Returns io.EOF upon encountering the archive trailer.
func (r *Reader) Next() (*Cpio_newc_header, error) {
	ent, err := r.stream.ReadNextEntry()
	if err != nil {
		return nil, err
	} else if ent.Header.filename == TRAILER {
		return nil, io.EOF
	}
	r.cur = ent
	return r.cur.Header, nil
}

// Read bytes from the file returned by the preceding call to Next().
//
// Returns io.EOF when the current file has been read in its entirety.
func (r *Reader) Read(p []byte) (n int, err error) {
	return r.cur.payload.Read(p)
}
