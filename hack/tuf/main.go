// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package main provides a tool to fetch the latest Sigstore trusted root via TUF
// This tool should be run periodically (e.g., before releases) to update the
// embedded trusted root with proper supply chain verification.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/tuf"
)

const (
	outputPath = "src/pkg/utils/data/trusted_root.json"
)

func main() {
	fmt.Println("Fetching latest Sigstore trusted root via TUF...")
	fmt.Println("This uses The Update Framework (TUF) to cryptographically verify the trusted root.")
	fmt.Println()

	// Use default TUF options (fetches from tuf-repo-cdn.sigstore.dev)
	// This provides cryptographic verification of the trusted root
	opts := tuf.DefaultOptions()

	trustedRoot, err := root.FetchTrustedRootWithOptions(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching trusted root: %v\n", err)
		os.Exit(1)
	}

	// Get JSON representation
	rootJSON, err := trustedRoot.MarshalJSON()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling trusted root: %v\n", err)
		os.Exit(1)
	}

	// Ensure output directory exists
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating directory %s: %v\n", dir, err)
		os.Exit(1)
	}

	// Write to file for embedding
	if err := os.WriteFile(outputPath, rootJSON, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing trusted root: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Trusted root successfully written to %s\n", outputPath)
	fmt.Printf("  Size: %d bytes\n", len(rootJSON))
	fmt.Println()
	fmt.Println("This file will be embedded in the Zarf binary at build time.")
	fmt.Println("Commit this file to ensure reproducible builds with verified trust roots.")
	fmt.Println()
	fmt.Println("To update periodically:")
	fmt.Println("  go run hack/tuf/main.go")
	fmt.Println("  git add src/pkg/utils/data/trusted_root.json")
	fmt.Println("  git commit -m 'chore: update embedded Sigstore trusted root'")
}
