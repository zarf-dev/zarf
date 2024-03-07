// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helpers provides generic helper functions with no external imports
package helpers

import (
	"io"
	"net/http"
	"os"
	"strings"
)

// Truncate truncates provided text to the requested length
func Truncate(text string, length int, invert bool) string {
	// Remove newlines and replace with semicolons
	textEscaped := strings.ReplaceAll(text, "\n", "; ")
	// Truncate the text if it is longer than length so it isn't too long.
	if len(textEscaped) > length {
		if invert {
			start := len(textEscaped) - length + 3
			textEscaped = "..." + textEscaped[start:]
		} else {
			end := length - 3
			textEscaped = textEscaped[:end] + "..."
		}
	}
	return textEscaped
}

// IsTextFile returns true if the given file is a text file.
func IsTextFile(path string) (bool, error) {
	// Open the file
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close() // Make sure to close the file when we're done

	// Get file stat
	stat, err := f.Stat()
	if err != nil {
		return false, err
	}

	// Clip offset to minimum of 0
	lastOffset := max(0, stat.Size()-512)

	// Take two passes checking front and back of the file
	offsetPasses := []int64{0, lastOffset}
	isTextCheck := []bool{false, false}
	for idx, offset := range offsetPasses {
		// Create 512 byte buffer
		data := make([]byte, 512)

		n, err := f.ReadAt(data, offset)
		if err != nil && err != io.EOF {
			return false, err
		}

		// Use http.DetectContentType to determine the MIME type of the file
		mimeType := http.DetectContentType(data[:n])

		// Check if the MIME type indicates that the file is text
		hasText := strings.HasPrefix(mimeType, "text/")
		hasJSON := strings.Contains(mimeType, "json")
		hasXML := strings.Contains(mimeType, "xml")

		// Save result
		isTextCheck[idx] = hasText || hasJSON || hasXML
	}

	// Returns true if both front and back show they are text
	return isTextCheck[0] && isTextCheck[1], nil
}
