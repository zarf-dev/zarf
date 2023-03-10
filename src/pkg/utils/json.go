// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic helper functions.
package utils

import (
	"encoding/json"
	"os"
)

// WriteJSON writes a given json struct to a json file on disk.
func WriteJSON(path string, v any) error {
	// Remove any file that might already exist
	_ = os.Remove(path)

	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	// Create the index.json file and save the data to it
	return os.WriteFile(path, data, 0644)
}
