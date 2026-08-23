// Copyright 2018 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package osx

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"syscall"
)

type MmapFile struct {
	f *os.File
	b []byte
}

func OpenMmapFile(path string) (*MmapFile, error) {
	return OpenMmapFileWithSize(path, 0)
}

func OpenMmapFileWithSize(path string, size int) (*MmapFile, error) {
	p := filepath.Clean(path)
	p = InlineTilde(p)

	f, err := os.Open(p)
	if err != nil {
		return nil, fmt.Errorf("try lock file: %w", err)
	}
	if size <= 0 {
		info, err := f.Stat()
		if err != nil {
			Close(f)
			return nil, fmt.Errorf("stat: %w", err)
		}
		size = int(info.Size())
	}

	b, err := mmap(f, size)
	if err != nil {
		Close(f)
		return nil, fmt.Errorf("mmap, size %d: %w", size, err)
	}

	return &MmapFile{f: f, b: b}, nil
}

func (f *MmapFile) Close() error {
	err0 := munmap(f.b)
	err1 := f.f.Close()

	if err0 != nil {
		return err0
	}
	return err1
}

func (f *MmapFile) Bytes() []byte {
	return f.b
}

func (f *MmapFile) Len() int64 {
	return int64(len(f.b))
}

var ErrPageFault = errors.New("page fault occurred while reading from memory map")

func (f *MmapFile) ReadAt(p []byte, off int64) (_ int, err error) {
	if off < 0 {
		return 0, syscall.EINVAL
	}
	if off > f.Len() {
		return 0, io.EOF
	}

	old := debug.SetPanicOnFault(true)
	defer func() {
		debug.SetPanicOnFault(old)
		if recover() != nil {
			err = ErrPageFault
		}
	}()

	n := copy(p, f.b[off:])
	if n < len(p) {
		err = io.EOF
	}
	return n, err
}
