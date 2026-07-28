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
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/sassoftware/go-rpmutils/cpio"
)

// Rpm is an open RPM header and payload
type Rpm struct {
	Header *RpmHeader
	f      io.Reader
}

// RpmHeader holds the signature header and general header of a RPM
//
// Tags are drawn from both header areas, with IDs between _GENERAL_TAG_BASE and
// _SIGHEADER_TAG_BASE coming from the general header and all others coming from
// the signature header.
type RpmHeader struct {
	lead      []byte
	sigHeader *rpmHeader
	genHeader *rpmHeader
	isSource  bool
}

// ReadRpm reads the header from a RPM file and prepares to read payload contents
func ReadRpm(f io.Reader) (*Rpm, error) {
	hdr, err := ReadHeader(f)
	if err != nil {
		return nil, err
	}
	return &Rpm{
		Header: hdr,
		f:      f,
	}, nil
}

// ExpandPayload extracts the payload of a RPM to the specified directory
func (rpm *Rpm) ExpandPayload(dest string) error {
	pld, err := uncompressRpmPayloadReader(rpm.f, rpm.Header)
	if err != nil {
		return err
	}
	if c, ok := pld.(io.Closer); ok {
		defer c.Close()
	}
	return cpio.Extract(pld, dest)
}

// PayloadReader accesses the payload cpio archive within the RPM.
//
// DEPRECATED: Use PayloadReaderExtended instead in order to handle files larger than 4GiB.
func (rpm *Rpm) PayloadReader() (*cpio.Reader, error) {
	pld, err := uncompressRpmPayloadReader(rpm.f, rpm.Header)
	if err != nil {
		return nil, err
	}
	return cpio.NewReader(pld), nil
}

// PayloadReaderExtended accesses payload file contents sequentially
func (rpm *Rpm) PayloadReaderExtended() (PayloadReader, error) {
	pld, err := uncompressRpmPayloadReader(rpm.f, rpm.Header)
	if err != nil {
		return nil, err
	}
	files, err := rpm.Header.GetFiles()
	if err != nil {
		return nil, err
	}
	return newPayloadReader(pld, files), nil
}

// ReadHeader reads the signature and general headers from a RPM.
//
// The stream is positioned for reading the compressed payload following the headers.
func ReadHeader(f io.Reader) (*RpmHeader, error) {
	lead, sigHeader, err := readSignatureHeader(f)
	if err != nil {
		return nil, err
	}

	hash, hashType := getHashAndType(sigHeader)
	genHeader, err := readHeader(f, hash, hashType, sigHeader.isSource, false)
	if err != nil {
		return nil, err
	}

	return &RpmHeader{
		lead:      lead,
		sigHeader: sigHeader,
		genHeader: genHeader,
		isSource:  sigHeader.isSource,
	}, nil
}

func readSignatureHeader(f io.Reader) ([]byte, *rpmHeader, error) {
	// Read signature header
	lead, err := readExact(f, 96)
	if err != nil {
		return nil, nil, fmt.Errorf("error reading RPM lead: %s", err.Error())
	}

	// Check file magic
	magic := binary.BigEndian.Uint32(lead[0:4])
	if magic&0xffffffff != 0xedabeedb {
		return nil, nil, fmt.Errorf("file is not an RPM")
	}

	// Check source flag
	isSource := binary.BigEndian.Uint16(lead[6:8]) == 1

	// Return signature header
	hdr, err := readHeader(f, "", 0, isSource, true)
	return lead, hdr, err
}

// HeaderRange indicates the byte offsets that the RPM header spans
type HeaderRange struct {
	// Start is the byte offset of the signature header
	Start int
	// End is the byte offset of the end of the general header and start of the payload
	End int
}

// GetRange returns the byte offsets that the RPM header spans within the original RPM file
func (hdr *RpmHeader) GetRange() HeaderRange {
	start := 96 + len(hdr.sigHeader.orig)
	end := start + len(hdr.genHeader.orig)
	return HeaderRange{
		Start: start,
		End:   end,
	}
}

// HasTag returns true if the given tag exists in the header
func (hdr *RpmHeader) HasTag(tag int) bool {
	h, t := hdr.getHeader(tag)
	return h.HasTag(t)
}

// Get the value of a tag. Returns whichever type most closely represents how
// the tag was stored, or NoSuchTagError if the tag was not found. If tag is
// OLDFILENAMES, special handling is provided to splice together DIRNAMES and
// BASENAMES if it is not present.
func (hdr *RpmHeader) Get(tag int) (interface{}, error) {
	h, t := hdr.getHeader(tag)
	return h.Get(t)
}

// GetString returns the value of a tag holding a single string
func (hdr *RpmHeader) GetString(tag int) (string, error) {
	vals, err := hdr.GetStrings(tag)
	if err != nil {
		return "", err
	}
	if len(vals) != 1 {
		return "", fmt.Errorf("incorrect number of values")
	}
	return vals[0], nil
}

// GetStrings fetches the given tag holding a string or array of strings. If tag
// is OLDFILENAMES, special handling is provided to splice together DIRNAMES and
// BASENAMES if it is not present.
func (hdr *RpmHeader) GetStrings(tag int) ([]string, error) {
	h, t := hdr.getHeader(tag)
	return h.GetStrings(t)
}

// GetInt gets an integer using the default 'int' type.
//
// DEPRECATED: large int32s and int64s can overflow. Use GetUint32s or GetUint64s instead.
func (hdr *RpmHeader) GetInt(tag int) (int, error) {
	vals, err := hdr.GetInts(tag)
	if err != nil {
		return -1, err
	}
	if len(vals) != 1 {
		return -1, fmt.Errorf("incorrect number of values")
	}
	return vals[0], nil
}

// GetInts gets an integer array using the default 'int' type.
//
// DEPRECATED: large int32s and int64s can overflow. Use GetUint32s or GetUint64s instead.
func (hdr *RpmHeader) GetInts(tag int) ([]int, error) {
	h, t := hdr.getHeader(tag)
	return h.GetInts(t)
}

// GetUint32s gets an int array as a uint32 slice. This can accomodate any int
// type other than INT64. Returns an error in case of overflow.
func (hdr *RpmHeader) GetUint32s(tag int) ([]uint32, error) {
	h, t := hdr.getHeader(tag)
	return h.GetUint32s(t)
}

// GetUint64s gets an int array as a uint64 slice. This can accomodate all int
// types
func (hdr *RpmHeader) GetUint64s(tag int) ([]uint64, error) {
	h, t := hdr.getHeader(tag)
	return h.GetUint64s(t)
}

// GetUint64Fallback gets longTag if it exists, otherwise intTag, and returns
// the value as an array of uint64s. This can accomodate all int types and is
// normally used when a int32 tag was later replaced with a int64 tag.
func (hdr *RpmHeader) GetUint64Fallback(intTag, longTag int) (uint64, error) {
	h, t := hdr.getHeader(longTag)
	vals, err := h.GetUint64s(t)
	if err == nil && len(vals) == 1 {
		return vals[0], nil
	}
	h, t = hdr.getHeader(intTag)
	vals, err = h.GetUint64s(t)
	if err != nil {
		return 0, err
	} else if len(vals) != 1 {
		return 0, errors.New("incorrect number of values")
	}
	return vals[0], nil
}

// GetBytes gets a tag as a byte array.
func (hdr *RpmHeader) GetBytes(tag int) ([]byte, error) {
	h, t := hdr.getHeader(tag)
	return h.GetBytes(t)
}

// getHeader decides whether the conventional tag ID is within the signature
// header range and returns the appropriate sub-header struct and raw tag
// identifier
func (hdr *RpmHeader) getHeader(tag int) (*rpmHeader, int) {
	if tag > _SIGHEADER_TAG_BASE {
		return hdr.sigHeader, tag - _SIGHEADER_TAG_BASE
	}
	if tag < _GENERAL_TAG_BASE {
		return hdr.sigHeader, tag
	}
	return hdr.genHeader, tag
}

// GetNEVRA gets the name, epoch, version, release and arch of the RPM.
func (hdr *RpmHeader) GetNEVRA() (*NEVRA, error) {
	return hdr.genHeader.GetNEVRA()
}

// GetFiles returns an array of FileInfo objects holding file-related attributes
// held in parallel arrays of tags
func (hdr *RpmHeader) GetFiles() ([]FileInfo, error) {
	return hdr.genHeader.GetFiles()
}

// InstalledSize returns the approximate disk space needed to install the package
func (hdr *RpmHeader) InstalledSize() (int64, error) {
	u, err := hdr.GetUint64Fallback(SIZE, LONGSIZE)
	if err != nil {
		return -1, err
	}
	return int64(u), nil
}

// PayloadSize returns the size of the uncompressed payload in bytes
func (hdr *RpmHeader) PayloadSize() (int64, error) {
	u, err := hdr.sigHeader.GetUint64Fallback(SIG_PAYLOADSIZE-_SIGHEADER_TAG_BASE, SIG_LONGARCHIVESIZE)
	if err != nil {
		return -1, err
	} else if len(u) != 1 {
		return -1, errors.New("incorrect number of values")
	}
	return int64(u[0]), err
}
