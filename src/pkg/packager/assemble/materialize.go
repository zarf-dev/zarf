// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package assemble

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zarf-dev/zarf/src/pkg/helpers"
	"github.com/zarf-dev/zarf/src/pkg/logger"
)

// materializePackageSource resolves source symlinks and writes their targets as
// regular files or directories in destination.
func materializePackageSource(ctx context.Context, source, destination string) error {
	return materializePackagePath(ctx, source, destination, make(map[string]struct{}))
}

func materializePackagePath(ctx context.Context, source, destination string, ancestors map[string]struct{}) error {
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
		logger.From(ctx).Warn("materializing symlink in package source", "source", source, "target", target)
		return materializePackagePath(ctx, target, destination, ancestors)
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

		if err := os.MkdirAll(destination, info.Mode().Perm()); err != nil {
			return err
		}
		if err := os.Chmod(destination, info.Mode().Perm()); err != nil {
			return err
		}
		entries, err := os.ReadDir(source)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := materializePackagePath(ctx, filepath.Join(source, entry.Name()), filepath.Join(destination, entry.Name()), ancestors); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("source %q has unsupported file type %s", source, info.Mode().Type())
	}
}
