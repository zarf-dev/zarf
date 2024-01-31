// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helpers provides generic helper functions with no external imports
package helpers

import (
	"fmt"
	"os"
)

// WriteFile writes the given data to the given path.
func WriteFile(path string, data []byte) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("unable to create the file at %s to write the contents: %w", path, err)
	}

	_, err = f.Write(data)
	if err != nil {
		_ = f.Close()
		return fmt.Errorf("unable to write the file at %s contents:%w", path, err)
	}

	err = f.Close()
	if err != nil {
		return fmt.Errorf("error saving file %s: %w", path, err)
	}

	return nil
}
