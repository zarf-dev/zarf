// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic helper functions.
package utils

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/message"
)

func parseChecksum(src string) (string, string, error) {
	atSymbolCount := strings.Count(src, "@")
	var checksum string
	if atSymbolCount > 0 {
		parsed, err := url.Parse(src)
		if err != nil {
			return src, checksum, fmt.Errorf("unable to parse the URL: %s", src)
		}
		if atSymbolCount == 1 && parsed.User != nil {
			return src, checksum, nil
		}

		index := strings.LastIndex(src, "@")
		checksum = src[index+1:]
		src = src[:index]
	}
	return src, checksum, nil
}

// DownloadToFile downloads a given URL to the target filepath (including the cosign key if necessary).
func DownloadToFile(ctx context.Context, src string, dst string, cosignKeyPath string) (err error) {
	message.Debugf("Downloading %s to %s", src, dst)
	// check if the parsed URL has a checksum
	// if so, remove it and use the checksum to validate the file
	src, checksum, err := parseChecksum(src)
	if err != nil {
		return err
	}

	err = helpers.CreateDirectory(filepath.Dir(dst), helpers.ReadWriteExecuteUser)
	if err != nil {
		return fmt.Errorf(lang.ErrCreatingDir, filepath.Dir(dst), err.Error())
	}

	// Create the file
	file, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf(lang.ErrWritingFile, dst, err.Error())
	}
	defer file.Close()

	parsed, err := url.Parse(src)
	if err != nil {
		return fmt.Errorf("unable to parse the URL: %s", src)
	}
	// If the source url starts with the sget protocol use that, otherwise do a typical GET call
	if parsed.Scheme == helpers.SGETURLScheme {
		err = Sget(ctx, src, cosignKeyPath, file)
		if err != nil {
			return fmt.Errorf("unable to download file with sget: %s: %w", src, err)
		}
		if err != nil {
			return err
		}
	} else {
		err = httpGetFile(src, file)
		if err != nil {
			return err
		}
	}

	// If the file has a checksum, validate it
	if len(checksum) > 0 {
		received, err := helpers.GetSHA256OfFile(dst)
		if err != nil {
			return err
		}
		if received != checksum {
			return fmt.Errorf("shasum mismatch for file %s: expected %s, got %s ", dst, checksum, received)
		}
	}

	return nil
}

func httpGetFile(url string, destinationFile *os.File) error {
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("unable to download the file %s", url)
	}
	defer resp.Body.Close()

	// Check server response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad HTTP status: %s", resp.Status)
	}

	// Writer the body to file
	title := fmt.Sprintf("Downloading %s", filepath.Base(url))
	progressBar := message.NewProgressBar(resp.ContentLength, title)

	if _, err = io.Copy(destinationFile, io.TeeReader(resp.Body, progressBar)); err != nil {
		progressBar.Failf("Unable to save the file %s: %s", destinationFile.Name(), err.Error())
		return err
	}

	title = fmt.Sprintf("Downloaded %s", url)
	progressBar.Successf("%s", title)
	return nil
}
