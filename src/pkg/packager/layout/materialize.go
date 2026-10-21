// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zarf-dev/zarf/src/pkg/helpers"
	"github.com/zarf-dev/zarf/src/pkg/logger"
)

// MaterializeSourceToLayout resolves symbolic links from a local package source
// and writes the resulting regular files and directories to destination in
// package staging.
//
// Use it for every local path selected by a package definition before that
// content enters package staging. Callers must provide a destination already
// validated as being within staging. Do not use it for generated, downloaded,
// or archive-extracted content.
func MaterializeSourceToLayout(ctx context.Context, source, destination string) error {
	return materializePath(ctx, source, destination, make(map[string]struct{}))
}

// resolveMaterializationDestination resolves existing destination parents
// without creating output that a source walk could consume.
func resolveMaterializationDestination(destination string) (string, error) {
	existing, err := filepath.Abs(destination)
	if err != nil {
		return "", err
	}

	var missing string
	for {
		resolved, err := filepath.EvalSymlinks(existing)
		if err == nil {
			return filepath.Join(resolved, missing), nil
		}
		parent := filepath.Dir(existing)
		if !os.IsNotExist(err) || parent == existing {
			return "", err
		}
		missing = filepath.Join(filepath.Base(existing), missing)
		existing = parent
	}
}

func materializePath(ctx context.Context, source, destination string, ancestors map[string]struct{}) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}

	if info.Mode()&os.ModeSymlink != 0 {
		target, err := filepath.EvalSymlinks(source)
		if err != nil {
			return fmt.Errorf("resolving symlink %q: %w", source, err)
		}
		target, err = filepath.Abs(target)
		if err != nil {
			return fmt.Errorf("getting absolute symlink target %q: %w", target, err)
		}
		logger.From(ctx).Warn("materializing package source symlink", "source", source, "target", target, "destination", destination)
		return materializePath(ctx, target, destination, ancestors)
	}

	resolvedSource, err := filepath.EvalSymlinks(source)
	if err != nil {
		return fmt.Errorf("resolving source %q: %w", source, err)
	}
	resolvedSource, err = filepath.Abs(resolvedSource)
	if err != nil {
		return fmt.Errorf("getting absolute source %q: %w", resolvedSource, err)
	}

	switch {
	case info.Mode().IsRegular():
		return helpers.CreatePathAndCopy(source, destination)
	case info.IsDir():
		if _, found := ancestors[resolvedSource]; found {
			return fmt.Errorf("symlink cycle includes %q", source)
		}
		ancestors[resolvedSource] = struct{}{}
		defer delete(ancestors, resolvedSource)

		resolvedDestination, err := resolveMaterializationDestination(destination)
		if err != nil {
			return fmt.Errorf("resolving destination %q: %w", destination, err)
		}
		if rel, err := filepath.Rel(resolvedSource, resolvedDestination); err == nil && filepath.IsLocal(rel) {
			return fmt.Errorf("cannot materialize source %q into destination %q: destination is within source directory", source, destination)
		}

		if err := os.MkdirAll(destination, helpers.ReadWriteExecuteUser); err != nil {
			return err
		}
		if err := os.Chmod(destination, helpers.ReadWriteExecuteUser); err != nil {
			return err
		}
		entries, err := os.ReadDir(source)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := materializePath(ctx, filepath.Join(source, entry.Name()), filepath.Join(destination, entry.Name()), ancestors); err != nil {
				return err
			}
		}
		if err := os.Chmod(destination, info.Mode().Perm()); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("source %q has unsupported file type %s", source, info.Mode().Type())
	}
}
