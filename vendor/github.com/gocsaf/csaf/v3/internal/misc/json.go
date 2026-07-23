// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSES/Apache-2.0.txt for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2025 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2025 Intevation GmbH <https://intevation.de>

package misc

import (
	"encoding/json"
	"fmt"
	"io"
)

// StrictJSONParse creates a JSON decoder that decodes an interface
// while not allowing trailing data
func StrictJSONParse(jsonData io.Reader, target any) error {
	decoder := json.NewDecoder(jsonData)
	// Don't allow unknown fields
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("JSON decoding error: %w", err)
	}

	// Check for any trailing data after the main JSON structure
	if _, err := decoder.Token(); err != io.EOF {
		if err != nil {
			return fmt.Errorf("error reading trailing data: %w", err)
		}
		return fmt.Errorf("unexpected trailing data after JSON object")
	}

	return nil
}
