// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package utils

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/sigstore/sigstore-go/pkg/root"
)

// EmbeddedTrustedRoot contains the Sigstore public TUF trusted root
// embedded at build time. This enables offline verification without
// requiring network access to tuf-repo-cdn.sigstore.dev.
//
// This file is fetched via TUF (The Update Framework) which provides:
//   - Cryptographic verification of the trusted root
//   - Protection against rollback attacks
//   - Secure key rotation
//
// To update this embedded file (e.g., before releases):
//
//	go run hack/tuf/main.go
//
// The file will be written to src/pkg/utils/data/trusted_root.json
// and should be committed to the repository.
//
//go:embed data/trusted_root.json
var EmbeddedTrustedRoot []byte

// GetTrustedRootMaterial returns TrustedMaterial for Cosign verification.
// Priority order:
//  1. If customPath is provided, load and use that trusted root
//  2. Otherwise, use the embedded trusted root (for air-gap compatibility)
//
// This enables:
//   - Manual override via custom path (for private Sigstore deployments)
//   - Offline operation via embedded root (air-gapped environments)
//   - No network calls during verification (TUF updates happen at build time)
func GetTrustedRootMaterial(customPath string) (root.TrustedMaterial, error) {
	// Priority 1: Use custom path if provided
	if customPath != "" {
		trustedRoot, err := root.NewTrustedRootFromPath(customPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load custom trusted root from %s: %w", customPath, err)
		}
		return trustedRoot, nil
	}

	// Priority 2: Use embedded trusted root (offline fallback)
	if len(EmbeddedTrustedRoot) == 0 {
		return nil, fmt.Errorf("no trusted root available: embedded root is empty")
	}

	trustedRoot, err := root.NewTrustedRootFromJSON(EmbeddedTrustedRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to parse embedded trusted root: %w", err)
	}

	return trustedRoot, nil
}

// GetTrustedRootPath returns a path to a trusted root JSON file.
// This is useful when the verification API requires a file path instead of
// the TrustedMaterial object.
//
// Priority order:
//  1. If customPath is provided and exists, use it
//  2. Otherwise, write embedded root to temp file
//
// Returns: (path string, cleanup func(), error)
// The cleanup function should be called to remove any temporary files.
func GetTrustedRootPath(customPath string) (string, func(), error) {
	cleanup := func() {} // No-op cleanup by default

	// Priority 1: Use custom path if provided
	if customPath != "" {
		if _, err := os.Stat(customPath); err == nil {
			return customPath, cleanup, nil
		}
		return "", cleanup, fmt.Errorf("custom trusted root not found: %s", customPath)
	}

	// Priority 2: Use embedded trusted root (write to temp file)
	if len(EmbeddedTrustedRoot) == 0 {
		return "", cleanup, fmt.Errorf("no trusted root available: embedded root is empty")
	}

	// Write embedded root to temp file
	tmpFile, err := os.CreateTemp("", "zarf-trusted-root-*.json")
	if err != nil {
		return "", cleanup, fmt.Errorf("failed to create temp file for embedded trusted root: %w", err)
	}

	if _, err := tmpFile.Write(EmbeddedTrustedRoot); err != nil {
		_ = tmpFile.Close()           //nolint:errcheck
		_ = os.Remove(tmpFile.Name()) //nolint:errcheck
		return "", cleanup, fmt.Errorf("failed to write embedded trusted root: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpFile.Name()) //nolint:errcheck
		return "", cleanup, fmt.Errorf("failed to close temp trusted root file: %w", err)
	}

	// Setup cleanup function to remove temp file
	cleanup = func() {
		_ = os.Remove(tmpFile.Name()) //nolint:errcheck
	}

	return tmpFile.Name(), cleanup, nil
}
