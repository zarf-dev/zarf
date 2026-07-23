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
	"encoding/binary"
	"errors"
	"io"
	"sort"
)

func writeTag(entries io.Writer, blobs *bytes.Buffer, tag int, e entry) error {
	align, ok := typeAlign[e.dataType]
	if ok {
		n := blobs.Len() % align
		if n != 0 {
			if _, err := blobs.Write(make([]byte, align-n)); err != nil {
				return err
			}
		}
	}
	ht := headerTag{
		Tag:      int32(tag),
		DataType: int32(e.dataType),
		Offset:   int32(blobs.Len()),
		Count:    int32(e.count),
	}
	if err := binary.Write(entries, binary.BigEndian, &ht); err != nil {
		return err
	}
	if _, err := blobs.Write(e.contents); err != nil {
		return err
	}
	return nil
}

func writeRegion(entries io.Writer, blobs *bytes.Buffer, regionTag int, tagCount int) error {
	// The data for a region tag is also in the format of a tag, and its offset
	// points backwards to the first tag that's part of the region. This one
	// covers the whole header.
	regionValue := headerTag{
		Tag:      int32(regionTag),
		DataType: RPM_BIN_TYPE,
		Offset:   int32(-16 * (1 + tagCount)),
		Count:    16,
	}
	regionBuf := bytes.NewBuffer(make([]byte, 0, 16))
	if err := binary.Write(regionBuf, binary.BigEndian, &regionValue); err != nil {
		return err
	}
	regionEntry := entry{dataType: RPM_BIN_TYPE, count: 16, contents: regionBuf.Bytes()}
	return writeTag(entries, blobs, regionTag, regionEntry)
}

// WriteTo writes the header out, adding a region tag encompassing all the existing tags
func (hdr *rpmHeader) WriteTo(outfile io.Writer, regionTag int) error {
	if regionTag != 0 && regionTag >= RPMTAG_HEADERREGIONS {
		return errors.New("invalid region tag")
	}
	// sort tags
	var keys []int
	for k := range hdr.entries {
		if k < RPMTAG_HEADERREGIONS {
			// discard existing regions
			continue
		}
		keys = append(keys, k)
	}
	sort.Ints(keys)
	entries := bytes.NewBuffer(make([]byte, 0, 16*len(keys)))
	blobs := bytes.NewBuffer(make([]byte, 0, len(hdr.orig)))
	for _, k := range keys {
		if k == regionTag {
			continue
		}
		if err := writeTag(entries, blobs, k, hdr.entries[k]); err != nil {
			return err
		}
	}
	intro := headerIntro{
		Magic:    introMagic,
		Reserved: 0,
		Entries:  uint32(len(keys) + 1),
		Size:     uint32(blobs.Len() + 16),
	}
	if err := binary.Write(outfile, binary.BigEndian, &intro); err != nil {
		return err
	}
	if err := writeRegion(outfile, blobs, regionTag, len(keys)); err != nil {
		return err
	}
	totalSize := 96 + blobs.Len() + entries.Len()
	if _, err := io.Copy(outfile, entries); err != nil {
		return err
	}
	if _, err := io.Copy(outfile, blobs); err != nil {
		return err
	}
	if regionTag == RPMTAG_HEADERSIGNATURES {
		alignment := totalSize % 8
		if alignment != 0 {
			if _, err := outfile.Write(make([]byte, 8-alignment)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (hdr *rpmHeader) size(regionTag int) (uint64, error) {
	var sink byteCountSink
	if err := hdr.WriteTo(&sink, regionTag); err != nil {
		return 0, err
	}
	return uint64(sink), nil
}

type byteCountSink uint64

func (sink *byteCountSink) Write(data []byte) (int, error) {
	*sink += byteCountSink(len(data))
	return len(data), nil
}

// OriginalSignatureHeaderSize returns the size of the lead and signature header
// area as originally read from the file.
func (hdr *RpmHeader) OriginalSignatureHeaderSize() int {
	return len(hdr.sigHeader.orig) + 96
}

// DumpSignatureHeader dumps the lead and signature header, optionally adding or
// changing padding to make it the same size as when it was originally read.
// Otherwise padding is removed to make it as small as possible.
//
// A RPM can be signed by removing the first OriginalSignatureHeaderSize() bytes
// of the file and replacing it with the result of DumpSignatureHeader().
func (hdr *RpmHeader) DumpSignatureHeader(sameSize bool) ([]byte, error) {
	if len(hdr.lead) != 96 {
		return nil, errors.New("invalid or missing RPM lead")
	}
	sigh := hdr.sigHeader
	regionTag := RPMTAG_HEADERSIGNATURES
	delete(sigh.entries, SIG_RESERVEDSPACE-_SIGHEADER_TAG_BASE)
	if sameSize {
		needed, err := sigh.size(regionTag)
		if err != nil {
			return nil, err
		}
		available := uint64(len(sigh.orig))
		if needed+16 <= available {
			// Fill unused space with a RESERVEDSPACE tag
			padding := make([]byte, available-needed-16)
			sigh.entries[SIG_RESERVEDSPACE-_SIGHEADER_TAG_BASE] = entry{
				dataType: RPM_BIN_TYPE,
				count:    int32(len(padding)),
				contents: padding,
			}
		}
	}
	buf := new(bytes.Buffer)
	buf.Write(hdr.lead)
	if err := sigh.WriteTo(buf, regionTag); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
