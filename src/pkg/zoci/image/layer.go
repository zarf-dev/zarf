// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package image

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	digest "github.com/opencontainers/go-digest"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/utils"
)

type ImageVolumeCompression string

// These are the 3 valid tar formats
const (
	// ImageVolumeCompressionGzip is the gzip compression format.
	ImageVolumeCompressionGzip ImageVolumeCompression = "gzip"
	// ImageVolumeCompressionZstd is the zstd compression format.
	ImageVolumeCompressionZstd ImageVolumeCompression = "zstd"
	// ImageVolumeCompressionUncompressed is the uncompressed compression format.
	ImageVolumeCompressionUncompressed ImageVolumeCompression = "uncompressed"
)

var (
	ErrSetFolder = errors.New("must set the Folder var")
)

type ImageVolumeLayer struct {
	Folder string
	size   int64
	digest digest.Digest
}

func (ivl *ImageVolumeLayer) Size() int64 {
	return ivl.size
}

func (ivl *ImageVolumeLayer) Digest() digest.Digest {
	return ivl.digest
}

func (ivl *ImageVolumeLayer) Clean() error {
	return os.RemoveAll(ivl.Folder)
}

func (ivl *ImageVolumeLayer) AddFile(file string, info os.FileInfo) error {
	if ivl.Folder == "" {
		return ErrSetFolder
	}
	tmpDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return err
	}

	tempTar := filepath.Join(tmpDir, fmt.Sprintf("%s.tar", filepath.Base(file)))
	out, err := os.Create(tempTar)
	if err != nil {
		return err
	}
	defer out.Close()

	digester := digest.Canonical.Digester()
	tw := tar.NewWriter(io.MultiWriter(out, digester.Hash()))

	fileName, err := filepath.Rel(ivl.Folder, file)
	if err != nil {
		return err
	}

	hdr, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	hdr.Name = fileName
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}

	src, err := os.Open(file)
	if err != nil {
		return err
	}
	defer src.Close()

	if _, err := io.Copy(tw, src); err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}

	fi, err := out.Stat()
	if err != nil {
		return err
	}

	ivl.digest = digester.Digest()
	ivl.size = fi.Size()

	return nil
}
