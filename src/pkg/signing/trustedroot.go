// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package signing

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/tuf"
)

// embeddedTrustedRoot is the Sigstore TrustedRoot JSON shipped with the binary.
// Refresh before each release with hack/refresh-trusted-root.sh.
//
//go:embed embedded_trusted_root.json
var embeddedTrustedRoot []byte

// configuredLiveTrustedRoot loads trusted material using the TUF configuration
// contract shared with Cosign without coupling direct verification to Cosign.
func configuredLiveTrustedRoot() (root.TrustedMaterial, error) {
	opts, err := configuredTUFOptions()
	if err != nil {
		return nil, fmt.Errorf("loading configured trusted root: %w", err)
	}
	material, err := root.NewLiveTrustedRoot(opts)
	if err != nil {
		return nil, fmt.Errorf("loading configured trusted root: %w", err)
	}
	return material, nil
}

// configuredTUFOptions resolves TUF_ROOT, TUF_MIRROR, and TUF_ROOT_JSON with
// the same precedence as Cosign's configured trusted root.
func configuredTUFOptions() (*tuf.Options, error) {
	opts := tuf.DefaultOptions()
	if cachePath := os.Getenv("TUF_ROOT"); cachePath != "" {
		opts.CachePath = cachePath
	}

	if mirror := os.Getenv("TUF_MIRROR"); mirror != "" {
		opts.RepositoryBaseURL = mirror
	} else {
		remotePath := filepath.Join(opts.CachePath, "remote.json")
		remoteJSON, err := os.ReadFile(remotePath)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("reading configured TUF remote file %q: %w", remotePath, err)
		}
		if err == nil {
			var remote struct {
				Mirror string `json:"mirror"`
			}
			if err := json.Unmarshal(remoteJSON, &remote); err != nil {
				return nil, fmt.Errorf("decoding configured TUF remote file %q: %w", remotePath, err)
			}
			opts.RepositoryBaseURL = remote.Mirror
		}
	}

	if opts.RepositoryBaseURL == tuf.DefaultMirror {
		return opts, nil
	}

	if rootPath := os.Getenv("TUF_ROOT_JSON"); rootPath != "" {
		rootJSON, err := os.ReadFile(rootPath)
		if err != nil {
			return nil, fmt.Errorf("reading configured TUF root JSON %q: %w", rootPath, err)
		}
		opts.Root = rootJSON
		return opts, nil
	}

	cachedRootPath := filepath.Join(opts.CachePath, tuf.URLToPath(opts.RepositoryBaseURL), "root.json")
	rootJSON, err := os.ReadFile(cachedRootPath)
	if err != nil {
		return nil, fmt.Errorf("reading configured TUF cached root %q: %w", cachedRootPath, err)
	}
	opts.Root = rootJSON
	return opts, nil
}

// writeEmbeddedTrustedRoot stages the embedded TrustedRoot JSON to a tempfile so
// cosign's VerifyBlobCmd (which only accepts file paths) can consume it.
// Caller must invoke cleanup when done; cleanup returns the os.Remove error.
// Will use the os default temporary directory if empty string is supplied
func writeEmbeddedTrustedRoot(tmpDir string) (string, func() error, error) {
	// os.CreateTemp will use os default if tmpdir is an empty string
	if tmpDir != "" {
		if err := os.MkdirAll(tmpDir, 0700); err != nil {
			return "", func() error { return nil }, fmt.Errorf("creating temp directory: %w", err)
		}
	}
	f, err := os.CreateTemp(tmpDir, "zarf-trusted-root-*.json")
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
