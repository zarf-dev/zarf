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

	// Read the first 512 bytes of the file
	data := make([]byte, 512)
	n, err := f.Read(data)
	if err != nil && err != io.EOF {
		return false, err
	}

	// Use http.DetectContentType to determine the MIME type of the file
	mimeType := http.DetectContentType(data[:n])

	// Check if the MIME type indicates that the file is text
	hasText := strings.HasPrefix(mimeType, "text/")
	hasJSON := strings.Contains(mimeType, "json")
	hasXML := strings.Contains(mimeType, "xml")

	return hasText || hasJSON || hasXML, nil
}
