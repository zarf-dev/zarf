// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic helper functions.
package utils

import (
	"context"
	"crypto"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/message"
)

// IsURL is a helper function to check if a URL is valid.
func IsURL(source string) bool {
	parsedURL, err := url.Parse(source)
	return err == nil && parsedURL.Scheme != "" && parsedURL.Host != ""
}

// IsOCIURL returns true if the given URL is an OCI URL.
func IsOCIURL(source string) bool {
	parsedURL, err := url.Parse(source)
	return err == nil && parsedURL.Scheme == "oci"
}

// DoHostnamesMatch returns a boolean indicating if the hostname of two different URLs are the same.
func DoHostnamesMatch(url1 string, url2 string) (bool, error) {
	parsedURL1, err := url.Parse(url1)
	if err != nil {
		message.Debugf("unable to parse the url (%s)", url1)

		return false, err
	}
	parsedURL2, err := url.Parse(url2)
	if err != nil {
		message.Debugf("unable to parse the url (%s)", url2)

		return false, err
	}

	return parsedURL1.Hostname() == parsedURL2.Hostname(), nil
}

// Fetch fetches the response body from a given URL.
func Fetch(url string) io.ReadCloser {
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		message.Fatal(err, "Unable to download the file")
	}

	// Check server response
	if resp.StatusCode != http.StatusOK {
		message.Fatalf(nil, "Bad HTTP status: %s", resp.Status)
	}

	return resp.Body
}

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
func DownloadToFile(src string, dst string, cosignKeyPath string) (err error) {
	message.Debugf("Downloading %s to %s", src, dst)
	// check if the parsed URL has a checksum
	// if so, remove it and use the checksum to validate the file
	src, checksum, err := parseChecksum(src)
	if err != nil {
		return err
	}

	// Create the file
	file, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf(lang.ErrWritingFile, dst, err.Error())
	}

	parsed, err := url.Parse(src)
	if err != nil {
		return fmt.Errorf("unable to parse the URL: %s", src)
	}
	// If the source url starts with the sget protocol use that, otherwise do a typical GET call
	if parsed.Scheme == "sget" {
		err = sgetFile(src, file, cosignKeyPath)
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
		received, err := GetCryptoHash(dst, crypto.SHA256)
		if err != nil {
			return err
		}
		if received != checksum {
			return fmt.Errorf("shasum mismatch for file %s: expected %s, got %s ", dst, checksum, received)
		}
	}
	return file.Close()
}

// GetAvailablePort retrieves an available port on the host machine. This delegates the port selection to the golang net
// library by starting a server and then checking the port that the server is using.
func GetAvailablePort() (int, error) {
	message.Debug("tunnel.GetAvailablePort()")
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer func(l net.Listener) {
		// ignore this error because it won't help us to tell the user
		_ = l.Close()
	}(l)

	_, p, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		return 0, err
	}
	port, err := strconv.Atoi(p)
	if err != nil {
		return 0, err
	}
	return port, err
}

// IsValidHostName returns a boolean indicating if the hostname of the host machine is valid according to https://www.ietf.org/rfc/rfc1123.txt.
func IsValidHostName() bool {
	// Quick & dirty character validation instead of a complete RFC validation since the OS is already allowing it
	hostname, err := os.Hostname()

	if err != nil {
		return false
	}

	return validHostname(hostname)
}

func validHostname(hostname string) bool {
	// Explanation: https://regex101.com/r/zUGqjP/1/
	rfcDomain := regexp.MustCompile(`^[a-zA-Z0-9\-.]+$`)
	// Explanation: https://regex101.com/r/vPGnzR/1/
	localhost := regexp.MustCompile(`\.?localhost$`)
	isValid := rfcDomain.MatchString(hostname)
	if isValid {
		isValid = !localhost.MatchString(hostname)
	}
	return isValid
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
	title := fmt.Sprintf("Downloading %s", path.Base(url))
	progressBar := message.NewProgressBar(resp.ContentLength, title)

	if _, err = io.Copy(destinationFile, io.TeeReader(resp.Body, progressBar)); err != nil {
		progressBar.Errorf(err, "Unable to save the file %s", destinationFile.Name())
		return err
	}

	title = fmt.Sprintf("Downloaded %s", url)
	progressBar.Successf("%s", title)
	return nil
}

func sgetFile(src string, destinationFile *os.File, cosignKeyPath string) error {
	// Remove the custom protocol header from the url
	parsed, err := url.Parse(src)
	if err != nil {
		return fmt.Errorf("unable to parse the URL: %s", src)
	}
	parsed.Scheme = ""
	err = Sget(context.TODO(), parsed.String(), cosignKeyPath, destinationFile)
	if err != nil {
		return fmt.Errorf("unable to download file with sget: %s", parsed.String())
	}
	return nil
}
