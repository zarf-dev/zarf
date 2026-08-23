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
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"

	"github.com/ulikunitz/xz/lzma"
	"github.com/xi2/xz"
)

// Wrap RPM payload with uncompress reader, assumes that header has
// already been read.
func uncompressRpmPayloadReader(r io.Reader, hdr *RpmHeader) (io.Reader, error) {
	// Check to make sure payload format is a cpio archive. If the tag does
	// not exist, assume archive is cpio.
	if hdr.HasTag(PAYLOADFORMAT) {
		val, err := hdr.GetString(PAYLOADFORMAT)
		if err != nil {
			return nil, err
		}
		if val != "cpio" {
			return nil, fmt.Errorf("Unknown payload format %s", val)
		}
	}

	// Check to see how the payload was compressed. If the tag does not
	// exist, check if it is gzip, if not it is uncompressed.
	var compression string
	if hdr.HasTag(PAYLOADCOMPRESSOR) {
		val, err := hdr.GetString(PAYLOADCOMPRESSOR)
		if err != nil {
			return nil, err
		}
		compression = val
	} else {
		// peek at the start of the payload to see if it's compressed
		b := make([]byte, 2)
		_, err := io.ReadFull(r, b)
		if err != nil {
			return nil, err
		}
		if b[0] == 0x1f && b[1] == 0x8b {
			compression = "gzip"
		} else {
			compression = "uncompressed"
		}
		// splice the peeked bytes back in
		r = io.MultiReader(bytes.NewReader(b), r)
	}

	switch compression {
	case "zstd":
		return newZstdReader(r)
	case "gzip":
		return gzip.NewReader(r)
	case "bzip2":
		return bzip2.NewReader(r), nil
	case "lzma":
		return lzma.NewReader(r)
	case "xz":
		return xz.NewReader(r, 0)
	case "uncompressed":
		// prevent ExpandPayload from closing the underlying file
		return noCloseWrapper{r}, nil
	default:
		return nil, fmt.Errorf("Unknown compression type %s", compression)
	}
}

type noCloseWrapper struct {
	r io.Reader
}

func (w noCloseWrapper) Read(d []byte) (int, error) {
	return w.r.Read(d)
}
