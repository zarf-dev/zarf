// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package signing

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
)

// embeddedTrustedRoot is the Sigstore TrustedRoot JSON shipped with the binary.
// Refresh before each release with hack/refresh-trusted-root.sh.
//
//go:embed embedded_trusted_root.json
var embeddedTrustedRoot []byte

// writeEmbeddedTrustedRoot stages the embedded TrustedRoot JSON to a tempfile so
// cosign's VerifyBlobCmd (which only accepts file paths) can consume it.
// Caller must invoke cleanup when done; cleanup returns the os.Remove error.
func writeEmbeddedTrustedRoot() (string, func() error, error) {
	f, err := os.CreateTemp("", "zarf-trusted-root-*.json")
	if err != nil {
		return "", func() error { return nil }, fmt.Errorf("creating tempfile: %w", err)
	}
	cleanup := func() error { return os.Remove(f.Name()) }

	if _, writeErr := f.Write(embeddedTrustedRoot); writeErr != nil {
		closeErr := f.Close()
		removeErr := cleanup()
		return "", func() error { return nil },
			fmt.Errorf("writing embedded trusted root: %w", errors.Join(writeErr, closeErr, removeErr))
	}
	if closeErr := f.Close(); closeErr != nil {
		removeErr := cleanup()
		return "", func() error { return nil },
			fmt.Errorf("closing embedded trusted root tempfile: %w", errors.Join(closeErr, removeErr))
	}
	return f.Name(), cleanup, nil
}
