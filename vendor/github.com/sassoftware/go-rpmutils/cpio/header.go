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
	"fmt"
	"io"
	"strconv"
)

// reference http://people.freebsd.org/~kientzle/libarchive/man/cpio.5.txt

const (
	newcHeaderLength = 110
	newcMagic        = "070701"
	strippedMagic    = "07070X"
)

// Cpio_newc_header is the raw header of a newc-style cpio archive
type Cpio_newc_header struct {
	magic     string
	ino       int
	mode      int
	uid       int
	gid       int
	nlink     int
	mtime     int
	filesize  int
	devmajor  int
	devminor  int
	rdevmajor int
	rdevminor int
	namesize  int
	check     int

	stripped bool
	filename string
	index    int
	size64   int64
}

type binaryReader struct {
	r   io.Reader
	buf [8]byte
}

func (br *binaryReader) Read16(buf *int) error {
	bb := br.buf[:8]
	if _, err := io.ReadFull(br.r, bb); err != nil {
		return err
	}
	i, err := strconv.ParseInt(string(bb), 16, 0)
	if err != nil {
		return err
	}
	*buf = int(i)
	return nil
}

func readHeader(r io.Reader) (*Cpio_newc_header, error) {
	hdr := Cpio_newc_header{}
	br := binaryReader{r: r}

	magic := make([]byte, 6)
	if _, err := io.ReadFull(r, magic); err != nil {
		return nil, err
	}
	if string(magic) == strippedMagic {
		return readStrippedHeader(br)
	} else if string(magic) != newcMagic {
		return nil, fmt.Errorf("bad magic")
	}
	hdr.magic = newcMagic

	if err := br.Read16(&hdr.ino); err != nil {
		return nil, err
	}
	if err := br.Read16(&hdr.mode); err != nil {
		return nil, err
	}
	if err := br.Read16(&hdr.uid); err != nil {
		return nil, err
	}
	if err := br.Read16(&hdr.gid); err != nil {
		return nil, err
	}
	if err := br.Read16(&hdr.nlink); err != nil {
		return nil, err
	}
	if err := br.Read16(&hdr.mtime); err != nil {
		return nil, err
	}
	if err := br.Read16(&hdr.filesize); err != nil {
		return nil, err
	}
	hdr.size64 = int64(hdr.filesize)
	if err := br.Read16(&hdr.devmajor); err != nil {
		return nil, err
	}
	if err := br.Read16(&hdr.devminor); err != nil {
		return nil, err
	}
	if err := br.Read16(&hdr.rdevmajor); err != nil {
		return nil, err
	}
	if err := br.Read16(&hdr.rdevminor); err != nil {
		return nil, err
	}
	if err := br.Read16(&hdr.namesize); err != nil {
		return nil, err
	}
	if err := br.Read16(&hdr.check); err != nil {
		return nil, err
	}
	return &hdr, nil
}

func readStrippedHeader(br binaryReader) (*Cpio_newc_header, error) {
	hdr := &Cpio_newc_header{
		magic:    strippedMagic,
		stripped: true,
	}
	if err := br.Read16(&hdr.index); err != nil {
		return nil, err
	}
	return hdr, nil
}

// Magic returns the magic number preceding the file entry
func (hdr *Cpio_newc_header) Magic() string { return hdr.magic }

// Ino returns the inode number of the file
func (hdr *Cpio_newc_header) Ino() int { return hdr.ino }

// Mode returns the file's permissions and file type
func (hdr *Cpio_newc_header) Mode() int { return hdr.mode }

// Uid returns the file's owner user ID
func (hdr *Cpio_newc_header) Uid() int { return hdr.uid }

// Gid returns the file's owner group ID
func (hdr *Cpio_newc_header) Gid() int { return hdr.gid }

// Nlink returns the number of hardlinks to the file
func (hdr *Cpio_newc_header) Nlink() int { return hdr.nlink }

// Mtime returns the file's modification time in seconds since the UNIX epoch
func (hdr *Cpio_newc_header) Mtime() int { return hdr.mtime }

// Filesize returns the size of the file in bytes
func (hdr *Cpio_newc_header) Filesize() int { return hdr.filesize }

// Devmajor returns the major device number of a character or block device
func (hdr *Cpio_newc_header) Devmajor() int { return hdr.devmajor }

// Devminor returns the minor device number of a character or block device
func (hdr *Cpio_newc_header) Devminor() int { return hdr.devminor }

// Rdevmajor returns the major device number of a character or block device
func (hdr *Cpio_newc_header) Rdevmajor() int { return hdr.rdevmajor }

// Rdevminor returns the minor device number of a character or block device
func (hdr *Cpio_newc_header) Rdevminor() int { return hdr.rdevminor }

// Namesize returns the length of the filename
func (hdr *Cpio_newc_header) Namesize() int { return hdr.namesize }

// Check returns the checksum of the entry, if present
func (hdr *Cpio_newc_header) Check() int { return hdr.check }

// Filename returns the name of the file entry
func (hdr *Cpio_newc_header) Filename() string { return hdr.filename }

// stripped header functions

// IsStripped returns true if the file header is missing info that must come
// from the preceding RPM header
func (hdr *Cpio_newc_header) IsStripped() bool { return hdr.stripped }

// Index returns the position in the RPM header file info array corresponding to
// this file
func (hdr *Cpio_newc_header) Index() int { return hdr.index }

// Filesize64 contains the file's size as a 64-bit integer, coming from either
// SetFileSizes() if used or from the regular cpio file entry
func (hdr *Cpio_newc_header) Filesize64() int64 { return hdr.size64 }
