// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package value supports values files and validation
package value

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/zarf-dev/zarf/src/pkg/logger"
)

// Values provides a map of keys to values for use in templating and Helm overrides.
type Values map[string]any

// ParseFilesOptions provides optional configuration for ParseFiles
type ParseFilesOptions struct {
	// TODO: Add schema check. Maybe here in parsing, or later in the process like templating?
	// Schema Schema
	// REVIEW: Should we guard against?
	// FileSizeLimit
	// MaximumYAMLDepth
	// Timeout
}

// ParseFiles parses the given files in order, overwriting previous values with later values, and returns a merged
// Values map.
func ParseFiles(ctx context.Context, paths []string, _ ParseFilesOptions) (_ Values, err error) {
	m := make(Values)
	start := time.Now()
	defer func() {
		logger.From(ctx).Debug("values parsing complete",
			"duration", time.Since(start),
			"files", len(paths))
	}()

	if ctx == nil {
		return Values{}, errors.New("context cannot be nil")
	}
	// No files given
	if len(paths) <= 0 {
		return Values{}, nil
	}
	// Validate file extensions
	for _, path := range paths {
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil, &InvalidFileExtError{FilePath: path, Ext: ext}
		}
	}
	logger.From(ctx).Debug("parsing values files", "paths", paths)
	for _, path := range paths {
		// Allow for cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		// Ensure file exists
		// REVIEW: Do we care about empty files? Here? Small UX tradeoff whether or not to fail on empty files
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		vals, err := parseFile(ctx, path)
		if err != nil {
			return nil, err
		}
		deepMerge(m, vals)
	}
	return m, nil
}

func parseFile(ctx context.Context, path string) (Values, error) {
	m := make(Values)

	// Handle files
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func(f *os.File) {
		if closeErr := f.Close(); closeErr != nil {
			// Log close errors, don't fail on them for read operations
			logger.From(ctx).Warn("failed to close file", "path", path, "error", closeErr)
		}
	}(f)

	// Decode and merge values
	if err = yaml.NewDecoder(f).DecodeContext(ctx, &m); err != nil {
		if errors.Is(err, io.EOF) {
			return m, nil // Empty file is ok
		}
		return nil, &YAMLDecodeError{
			FilePath: path,
			Err:      fmt.Errorf("%s", yaml.FormatError(err, true, true)),
		}
	}
	return m, nil
}

// CheckSchema_Stub is intended to take a JSON schema and validate it against the values file(s).
// TODO: implement public
// TODO: Some open design questions:
// - Do we take a json or byte array, a map[string]any, or a specific json.schema type?
// - Do we want to return a list of errors, some specific schema fail datatype, or some other type?
// - Surely there's libraries for this which have their own opinionated inputs for the schema and return types
func checkSchemaStub(_ Values, _ string) []error {
	return nil
}

// deepMergeValues merges two Values maps recursively via mutation, overwriting keys in dst with keys from src
func deepMerge(dst, src Values) {
	for key, srcVal := range src {
		if dstVal, exists := dst[key]; exists {
			// Both have the key, merge
			srcMap, srcIsMap := srcVal.(map[string]any)
			dstMap, dstIsMap := dstVal.(map[string]any)
			if srcIsMap && dstIsMap {
				// Both are maps, recur
				deepMerge(dstMap, srcMap)
			} else {
				// Not both maps, src overwrites dst
				dst[key] = srcVal
			}
		} else {
			// Key only in src
			dst[key] = srcVal
		}
	}
}

// InvalidFileExtError represents an error when a file has an invalid extension
type InvalidFileExtError struct {
	FilePath string
	Ext      string
}

func (e *InvalidFileExtError) Error() string {
	return fmt.Sprintf("invalid file extension for values file %s: %s", e.FilePath, e.Ext)
}

// YAMLDecodeError represents an error when YAML parsing fails
type YAMLDecodeError struct {
	FilePath string
	Err      error
}

func (e *YAMLDecodeError) Error() string {
	return fmt.Sprintf("failed to decode YAML from values file %s: %v", e.FilePath, e.Err)
}

func (e *YAMLDecodeError) Unwrap() error {
	return e.Err
}
