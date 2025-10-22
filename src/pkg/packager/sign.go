// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"oras.land/oras-go/v2/registry"
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
	// Retries is the number of retries to attempt when publishing to OCI
	Retries int
	RemoteOptions
	// CachePath is the path to the cache directory
	CachePath string
}

// SignExistingPackage signs an existing Zarf package with the provided signing key.
// It loads the package, optionally verifies any existing signature, removes the old signature,
// signs the zarf.yaml file, and either archives to a directory or publishes to an OCI registry.
//
// The outputDest parameter can be:
//   - A local directory path (e.g., "./output" or "/tmp/signed")
//   - An OCI registry URL (e.g., "oci://ghcr.io/my-org/packages")
func SignExistingPackage(
	ctx context.Context,
	packageSource string,
	outputDest string,
	opts SignOptions,
) (string, error) {
	l := logger.From(ctx)

	// Validate required options
	// Note: This should be removed when broader signing strategies are available
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
	l.Info("signing package with provided key")

	// Create a password function for encrypted keys
	passFunc := cosign.PassFunc(func(_ bool) ([]byte, error) {
		return []byte(opts.SigningKeyPassword), nil
	})

	// Build cosign sign options from packager sign options
	signOpts := utils.DefaultSignBlobOptions()
	signOpts.KeyRef = opts.SigningKeyPath
	signOpts.PassFunc = passFunc

	err = pkgLayout.SignPackage(ctx, signOpts)
	if err != nil {
		return "", fmt.Errorf("failed to sign package: %w", err)
	}

	// Check if output destination is an OCI registry
	if helpers.IsOCIURL(outputDest) {
		l.Info("publishing signed package to OCI registry", "destination", outputDest)

		// Parse the OCI reference
		trimmed := strings.TrimPrefix(outputDest, helpers.OCIURLPrefix)
		parts := strings.Split(trimmed, "/")
		dstRef := registry.Reference{
			Registry:   parts[0],
			Repository: strings.Join(parts[1:], "/"),
		}

		// Validate the registry reference
		if err := dstRef.ValidateRegistry(); err != nil {
			return "", fmt.Errorf("invalid OCI registry URL: %w", err)
		}

		// Publish the signed package to OCI
		publishOpts := PublishPackageOptions{
			OCIConcurrency:     opts.OCIConcurrency,
			SigningKeyPath:     "", // Already signed, don't re-sign
			SigningKeyPassword: "",
			Retries:            opts.Retries,
			RemoteOptions:      opts.RemoteOptions,
		}

		pubRef, err := PublishPackage(ctx, pkgLayout, dstRef, publishOpts)
		if err != nil {
			return "", fmt.Errorf("failed to publish signed package to OCI: %w", err)
		}

		refString := pubRef.String()
		l.Info("package signed and published successfully", "reference", refString)
		return refString, nil
	}

	// Archive to local directory (includes the new signature)
	l.Info("archiving signed package to local directory", "directory", outputDest)
	signedPath, err := pkgLayout.Archive(ctx, outputDest, 0)
	if err != nil {
		return "", fmt.Errorf("failed to archive signed package: %w", err)
	}

	l.Info("package signed successfully", "path", signedPath)
	return signedPath, nil
}
