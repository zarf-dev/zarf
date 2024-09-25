// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package layout contains functions for interacting with Zarf's package layout on disk.
package layout

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/types"
)

// splitFile will split the file into chunks and remove the original file.
func splitFile(srcPath string, chunkSize int) (err error) {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer func(srcFile *os.File) {
		err2 := srcFile.Close()
		err = errors.Join(err, err2)
	}(srcFile)
	fi, err := srcFile.Stat()
	if err != nil {
		return err
	}

	title := fmt.Sprintf("[0/%d] MB bytes written", fi.Size()/1000/1000)
	progressBar := message.NewProgressBar(fi.Size(), title)
	defer func(progressBar *message.ProgressBar) {
		err2 := progressBar.Close()
		err = errors.Join(err, err2)
	}(progressBar)

	hash := sha256.New()
	fileCount := 0
	// TODO(mkcp): The inside of this loop should be wrapped in a closure so we can close the destination file each
	//   iteration as soon as we're done writing.
	for {
		path := fmt.Sprintf("%s.part%03d", srcPath, fileCount+1)
		dstFile, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, helpers.ReadAllWriteUser)
		if err != nil {
			return err
		}
		defer func(dstFile *os.File) {
			err2 := dstFile.Close()
			err = errors.Join(err, err2)
		}(dstFile)

		written, copyErr := io.CopyN(dstFile, srcFile, int64(chunkSize))
		if copyErr != nil && !errors.Is(copyErr, io.EOF) {
			return err
		}
		progressBar.Add(int(written))
		title := fmt.Sprintf("[%d/%d] MB bytes written", progressBar.GetCurrent()/1000/1000, fi.Size()/1000/1000)
		progressBar.Updatef(title)

		_, err = dstFile.Seek(0, io.SeekStart)
		if err != nil {
			return err
		}
		_, err = io.Copy(hash, dstFile)
		if err != nil {
			return err
		}
		err = dstFile.Close()
		if err != nil {
			return err
		}

		// EOF error could be returned on 0 bytes written.
		if written == 0 {
			err = os.Remove(path)
			if err != nil {
				return err
			}
			break
		}

		fileCount++
		if errors.Is(copyErr, io.EOF) {
			break
		}
	}

	// Remove original file
	err = srcFile.Close()
	if err != nil {
		return err
	}
	err = os.Remove(srcPath)
	if err != nil {
		return err
	}

	// Write header file
	data := types.ZarfSplitPackageData{
		Count:     fileCount,
		Bytes:     fi.Size(),
		Sha256Sum: fmt.Sprintf("%x", hash.Sum(nil)),
	}
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("unable to marshal the split package data: %w", err)
	}
	path := fmt.Sprintf("%s.part000", srcPath)
	if err := os.WriteFile(path, b, helpers.ReadAllWriteUser); err != nil {
		return fmt.Errorf("unable to write the file %s: %w", path, err)
	}
	progressBar.Successf("Package split across %d files", fileCount+1)

	return nil
}
