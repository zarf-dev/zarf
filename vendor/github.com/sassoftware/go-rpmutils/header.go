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
	"crypto"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
)

const introMagic = 0x8eade801

type entry struct {
	dataType, count int32
	contents        []byte
}

type rpmHeader struct {
	entries  map[int]entry
	isSource bool
	orig     []byte
}

type headerIntro struct {
	Magic, Reserved, Entries, Size uint32
}

type headerTag struct {
	Tag, DataType, Offset, Count int32
}

var typeAlign = map[int32]int{
	RPM_INT16_TYPE: 2,
	RPM_INT32_TYPE: 4,
	RPM_INT64_TYPE: 8,
}

var typeSizes = map[int32]int{
	RPM_NULL_TYPE:  0,
	RPM_CHAR_TYPE:  1,
	RPM_INT8_TYPE:  1,
	RPM_INT16_TYPE: 2,
	RPM_INT32_TYPE: 4,
	RPM_INT64_TYPE: 8,
	RPM_BIN_TYPE:   1,
}

func readExact(f io.Reader, n int) ([]byte, error) {
	buf := make([]byte, n)
	_, err := io.ReadFull(f, buf)
	return buf, err
}

func readHeader(f io.Reader, hash string, hashType crypto.Hash, isSource bool, sigBlock bool) (*rpmHeader, error) {
	// save original header
	var origBuf bytes.Buffer
	f = io.TeeReader(f, &origBuf)
	// verify intro
	var intro headerIntro
	if err := binary.Read(f, binary.BigEndian, &intro); err != nil {
		return nil, fmt.Errorf("error reading RPM header: %s", err.Error())
	}
	if intro.Magic != introMagic {
		return nil, fmt.Errorf("bad magic for header")
	}
	// read entries
	entryTable, err := readExact(f, int(intro.Entries*16))
	if err != nil {
		return nil, fmt.Errorf("error reading RPM header table: %s", err.Error())
	}
	// read data
	size := intro.Size
	if sigBlock {
		// signature block is padded to 8 byte alignment
		size = (size + 7) / 8 * 8
	}
	data, err := readExact(f, int(size))
	if err != nil {
		return nil, fmt.Errorf("error reading RPM header data: %s", err.Error())
	}
	// Check hash if it was specified
	if len(hash) > 1 {
		h := hashType.New()
		h.Write(origBuf.Bytes())
		calculated := hex.EncodeToString(h.Sum(nil))
		if calculated != hash {
			return nil, fmt.Errorf("%s mismatch in signature header", hashType)
		}
	}
	// parse entries
	ents := make(map[int]entry)
	buf := bytes.NewReader(entryTable)
	for i := 0; i < int(intro.Entries); i++ {
		var tag headerTag
		if err := binary.Read(buf, binary.BigEndian, &tag); err != nil {
			return nil, err
		}
		typeSize, ok := typeSizes[tag.DataType]
		var end int
		if ok {
			end = int(tag.Offset) + typeSize*int(tag.Count)
		} else {
			// String types are null-terminated
			end = int(tag.Offset)
			for i := 0; i < int(tag.Count); i++ {
				next := bytes.IndexByte(data[end:], 0)
				if next < 0 {
					return nil, fmt.Errorf("tag %d is truncated", tag.Tag)
				}
				end += next + 1
			}
		}
		ents[int(tag.Tag)] = entry{
			dataType: tag.DataType,
			count:    tag.Count,
			contents: data[tag.Offset:end],
		}
	}

	return &rpmHeader{
		entries:  ents,
		isSource: isSource,
		orig:     origBuf.Bytes(),
	}, nil
}

// HasTag returns true if the given tag exists in the header
func (hdr *rpmHeader) HasTag(tag int) bool {
	_, ok := hdr.entries[tag]
	return ok
}

// Get the value of a tag. Returns whichever type most closely represents how
// the tag was stored, or NoSuchTagError if the tag was not found. If tag is
// OLDFILENAMES, special handling is provided to splice together DIRNAMES and
// BASENAMES if it is not present.
func (hdr *rpmHeader) Get(tag int) (interface{}, error) {
	ent, ok := hdr.entries[tag]
	if !ok && tag == OLDFILENAMES {
		return hdr.GetStrings(tag)
	}
	if !ok {
		return nil, NewNoSuchTagError(tag)
	}
	switch ent.dataType {
	case RPM_STRING_TYPE, RPM_STRING_ARRAY_TYPE, RPM_I18NSTRING_TYPE:
		return hdr.GetStrings(tag)
	case RPM_INT8_TYPE, RPM_INT16_TYPE, RPM_INT32_TYPE, RPM_INT64_TYPE, RPM_CHAR_TYPE:
		out, _, err := hdr.getInts(tag)
		return out, err
	case RPM_BIN_TYPE:
		return hdr.GetBytes(tag)
	default:
		return nil, fmt.Errorf("unsupported data type")
	}
}

// GetStrings fetches the given tag holding a string or array of strings. If tag
// is OLDFILENAMES, special handling is provided to splice together DIRNAMES and
// BASENAMES if it is not present.
func (hdr *rpmHeader) GetStrings(tag int) ([]string, error) {
	ent, ok := hdr.entries[tag]
	if tag == OLDFILENAMES && !ok {
		dirs, err := hdr.GetStrings(DIRNAMES)
		if err != nil {
			return nil, err
		}
		dirIdxs, err := hdr.GetUint32s(DIRINDEXES)
		if err != nil {
			return nil, err
		}
		baseNames, err := hdr.GetStrings(BASENAMES)
		if err != nil {
			return nil, err
		}
		paths := make([]string, 0, len(baseNames))
		for i, base := range baseNames {
			paths = append(paths, path.Join(dirs[dirIdxs[i]], base))
		}
		return paths, nil
	}

	if !ok {
		return nil, NewNoSuchTagError(tag)
	}
	if ent.dataType != RPM_STRING_TYPE && ent.dataType != RPM_STRING_ARRAY_TYPE && ent.dataType != RPM_I18NSTRING_TYPE {
		return nil, fmt.Errorf("unsupported datatype for string: %d, tag: %d", ent.dataType, tag)
	}
	strs := strings.Split(string(ent.contents), "\x00")
	return strs[:ent.count], nil
}

// get an int array using whatever the appropriate sized type is
func (hdr *rpmHeader) getInts(tag int) (buf interface{}, n int, err error) {
	ent, ok := hdr.entries[tag]
	if !ok {
		return nil, 0, NewNoSuchTagError(tag)
	}
	n = len(ent.contents)
	switch ent.dataType {
	case RPM_INT8_TYPE, RPM_CHAR_TYPE:
		buf = make([]uint8, n)
	case RPM_INT16_TYPE:
		n >>= 1
		buf = make([]uint16, n)
	case RPM_INT32_TYPE:
		n >>= 2
		buf = make([]uint32, n)
	case RPM_INT64_TYPE:
		n >>= 3
		buf = make([]uint64, n)
	default:
		return nil, 0, fmt.Errorf("tag %d isn't an int type", tag)
	}
	if err := binary.Read(bytes.NewReader(ent.contents), binary.BigEndian, buf); err != nil {
		return nil, 0, err
	}
	return
}

// GetInts gets an integer array using the default 'int' type.
//
// DEPRECATED: large int32s and int64s can overflow. Use GetUint32s or GetUint64s instead.
func (hdr *rpmHeader) GetInts(tag int) ([]int, error) {
	buf, n, err := hdr.getInts(tag)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	switch bvals := buf.(type) {
	case []uint8:
		for i, v := range bvals {
			out[i] = int(v)
		}
	case []uint16:
		for i, v := range bvals {
			out[i] = int(v)
		}
	case []uint32:
		for i, v := range bvals {
			if v > (1<<31)-1 {
				return nil, fmt.Errorf("value %d out of range for int32 array in tag %d", i, tag)
			}
			out[i] = int(v)
		}
	default:
		return nil, fmt.Errorf("tag %d is too big for int type", tag)
	}
	return out, nil
}

// GetUint32s gets an int array as a uint32 slice. This can accomodate any int
// type other than INT64. Returns an error in case of overflow.
func (hdr *rpmHeader) GetUint32s(tag int) ([]uint32, error) {
	buf, n, err := hdr.getInts(tag)
	if err != nil {
		return nil, err
	}
	if out, ok := buf.([]uint32); ok {
		return out, nil
	}
	out := make([]uint32, n)
	switch bvals := buf.(type) {
	case []uint8:
		for i, v := range bvals {
			out[i] = uint32(v)
		}
	case []uint16:
		for i, v := range bvals {
			out[i] = uint32(v)
		}
	default:
		return nil, fmt.Errorf("tag %d is too big for int type", tag)
	}
	return out, nil
}

// GetUint64s gets an int array as a uint64 slice. This can accomodate all int
// types.
func (hdr *rpmHeader) GetUint64s(tag int) ([]uint64, error) {
	buf, n, err := hdr.getInts(tag)
	if err != nil {
		return nil, err
	}
	if out, ok := buf.([]uint64); ok {
		return out, nil
	}
	out := make([]uint64, n)
	switch bvals := buf.(type) {
	case []uint8:
		for i, v := range bvals {
			out[i] = uint64(v)
		}
	case []uint16:
		for i, v := range bvals {
			out[i] = uint64(v)
		}
	case []uint32:
		for i, v := range bvals {
			out[i] = uint64(v)
		}
	}
	return out, nil
}

// GetUint64Fallback gets longTag if it exists, otherwise intTag, and returns
// the value as an array of uint64s. This can accomodate all int types and is
// normally used when a int32 tag was later replaced with a int64 tag.
func (hdr *rpmHeader) GetUint64Fallback(intTag, longTag int) ([]uint64, error) {
	if _, ok := hdr.entries[longTag]; ok {
		return hdr.GetUint64s(longTag)
	}
	return hdr.GetUint64s(intTag)
}

// GetBytes gets a tag as a byte array.
func (hdr *rpmHeader) GetBytes(tag int) ([]byte, error) {
	ent, ok := hdr.entries[tag]
	if !ok {
		return nil, NewNoSuchTagError(tag)
	}
	if ent.dataType != RPM_BIN_TYPE {
		return nil, fmt.Errorf("unsupported datatype for bytes: %d, tag: %d", ent.dataType, tag)
	}
	return ent.contents, nil
}

// GetNEVRA gets the name, epoch, version, release and arch of the RPM.
func (hdr *rpmHeader) GetNEVRA() (*NEVRA, error) {
	name, err := hdr.GetStrings(NAME)
	if err != nil {
		return nil, err
	}
	epoch, err := hdr.GetUint64s(EPOCH)
	if _, absent := err.(NoSuchTagError); !absent && err != nil {
		return nil, err
	} else if len(epoch) == 0 {
		// no epoch is treated as 0
		epoch = []uint64{0}
	}
	version, err := hdr.GetStrings(VERSION)
	if err != nil {
		return nil, err
	}
	release, err := hdr.GetStrings(RELEASE)
	if err != nil {
		return nil, err
	}
	arch, err := hdr.GetStrings(ARCH)
	if err != nil {
		return nil, err
	}
	return &NEVRA{
		Name:    name[0],
		Epoch:   strconv.FormatUint(epoch[0], 10),
		Version: version[0],
		Release: release[0],
		Arch:    arch[0],
	}, nil
}

// GetFiles returns an array of FileInfo objects holding file-related attributes
// held in parallel arrays of tags
//
// If the RPM has no files, GetFiles returns an empty list.
func (hdr *rpmHeader) GetFiles() ([]FileInfo, error) {
	paths, err := hdr.GetStrings(OLDFILENAMES)
	if err != nil {
		if !errors.As(err, &NoSuchTagError{}) {
			return nil, err
		}
		// RPM has no files
		return nil, nil
	}
	fileSizes, err := hdr.GetUint64Fallback(FILESIZES, LONGFILESIZES)
	if err != nil {
		return nil, err
	}
	fileUserName, err := hdr.GetStrings(FILEUSERNAME)
	if err != nil {
		return nil, err
	}
	fileGroupName, err := hdr.GetStrings(FILEGROUPNAME)
	if err != nil {
		return nil, err
	}
	fileFlags, err := hdr.GetUint32s(FILEFLAGS)
	if err != nil {
		return nil, err
	}
	fileMtimes, err := hdr.GetUint32s(FILEMTIMES)
	if err != nil {
		return nil, err
	}
	fileDigests, err := hdr.GetStrings(FILEDIGESTS)
	if err != nil {
		return nil, err
	}
	fileModes, err := hdr.GetUint32s(FILEMODES)
	if err != nil {
		return nil, err
	}
	linkTos, err := hdr.GetStrings(FILELINKTOS)
	if err != nil {
		return nil, err
	}
	devices, err := hdr.GetUint32s(FILEDEVICES)
	if err != nil {
		devices = make([]uint32, len(paths))
	}
	inodes, err := hdr.GetUint32s(FILEINODES)
	if err != nil {
		inodes = make([]uint32, len(paths))
	}

	files := make([]FileInfo, len(paths))
	for i := 0; i < len(paths); i++ {
		files[i] = &fileInfo{
			name:      paths[i],
			size:      fileSizes[i],
			userName:  fileUserName[i],
			groupName: fileGroupName[i],
			flags:     fileFlags[i],
			mtime:     fileMtimes[i],
			digest:    fileDigests[i],
			mode:      fileModes[i],
			linkName:  linkTos[i],
			device:    devices[i],
			inode:     inodes[i],
		}
	}

	return files, nil
}
