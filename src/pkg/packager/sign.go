// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
)

// SignOptions are the options used when signing an existing package.
type SignOptions struct {
	// SigningKeyPath is the path to the private key or KMS URL for signing
	SigningKeyPath string
	// SigningKeyPassword is the password for encrypted private keys
	SigningKeyPassword string
	// PublicKeyPath is the path to the public key for verifying existing signatures (optional)
	PublicKeyPath string
	// SkipSignatureValidation skips verification of existing signatures when loading the package
	SkipSignatureValidation bool
	// Overwrite allows overwriting an existing signature
	Overwrite bool
	// OCIConcurrency is the number of concurrent OCI operations
	OCIConcurrency int
	RemoteOptions
	// CachePath is the path to the cache directory
	CachePath string
}

// SignExistingPackage signs an existing Zarf package with the provided signing key.
// It loads the package, optionally verifies any existing signature, removes the old signature,
// signs the zarf.yaml file, and archives the signed package to the output directory.
func SignExistingPackage(
	ctx context.Context,
	packageSource string,
	outputDir string,
	opts SignOptions,
) (string, error) {
	l := logger.From(ctx)

	// Validate required options
	if opts.SigningKeyPath == "" {
		return "", errors.New("signing key path is required")
	}

	// Load the package
	loadOpts := LoadOptions{
		PublicKeyPath:           opts.PublicKeyPath,
		SkipSignatureValidation: opts.SkipSignatureValidation,
		Filter:                  filters.Empty(),
		Architecture:            config.GetArch(),
		OCIConcurrency:          opts.OCIConcurrency,
		RemoteOptions:           opts.RemoteOptions,
		CachePath:               opts.CachePath,
	}

	l.Info("loading package", "source", packageSource)
	pkgLayout, err := LoadPackage(ctx, packageSource, loadOpts)
	if err != nil {
		return "", fmt.Errorf("unable to load package: %w", err)
	}
	defer func() {
		if cleanupErr := pkgLayout.Cleanup(); cleanupErr != nil {
			l.Warn("failed to cleanup package layout", "error", cleanupErr)
		}
	}()

	// Check for existing signature
	sigPath := filepath.Join(pkgLayout.DirPath(), layout.Signature)
	_, err = os.Stat(sigPath)
	sigExists := err == nil

	if sigExists && !opts.Overwrite {
		return "", errors.New("package is already signed, use --overwrite to re-sign")
	}

	if sigExists {
		l.Info("removing existing signature")
		if err := os.Remove(sigPath); err != nil {
			return "", fmt.Errorf("failed to remove old signature: %w", err)
		}
	}

	// Sign the package
	// This creates a new zarf.yaml.sig without modifying any checksums
	// The signature file is intentionally excluded from checksums.txt
	l.Info("signing package with provided key")
	err = pkgLayout.SignPackage(opts.SigningKeyPath, opts.SigningKeyPassword)
	if err != nil {
		return "", fmt.Errorf("failed to sign package: %w", err)
	}

	// Archive to output directory (includes the new signature)
	l.Info("archiving signed package")
	signedPath, err := pkgLayout.Archive(ctx, outputDir, 0)
	if err != nil {
		return "", fmt.Errorf("failed to archive signed package: %w", err)
	}

	l.Info("package signed successfully", "path", signedPath)
	return signedPath, nil
}
