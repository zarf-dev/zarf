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

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/goccy/go-yaml"
	"github.com/xeipuuv/gojsonschema"
	"github.com/zarf-dev/zarf/src/pkg/logger"
)

// Values provides a map of keys to values for use in templating and Helm overrides.
// NOTE(mkcp): Values is NOT thread-safe. If you need concurrent access:
//   - Use external synchronization (sync.RWMutex)
//   - Clone before passing to goroutines
type Values map[string]any

// Path starts with a . and represents a specific key in a nested hierarchy of keys. For example, .resources.limits.cpu
// resolves the value for "cpu" within the keyspace of Values.
type Path string

// Validate inspects the string stored at Path and ensures it's valid.
func (p Path) Validate() error {
	if p == "" || !strings.HasPrefix(string(p), ".") {
		return fmt.Errorf("invalid path format: %s", p)
	}
	return nil
}

// ParseFilesOptions provides optional configuration for ParseFiles
type ParseFilesOptions struct {
	// TODO(mkcp): Add schema check. Maybe here in parsing, or later in the process like templating?
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
			var vals Values
			if helpers.IsURL(path) {
				return nil, fmt.Errorf("remote values files not yet supported, url=%s", path)
			}
			_, err := helpers.IsTextFile(path)
			if err != nil {
				return nil, err
			}
			vals, err = parseLocalFile(ctx, path)
			if err != nil {
				return nil, err
			}
			// Done, merge new values into existing
			m.DeepMerge(vals)
		}
	}
	return m, nil
}

func parseLocalFile(ctx context.Context, path string) (Values, error) {
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

// DeepMerge merges one or more Values maps recursively into the receiver via mutation.
// Later maps in the variadic arguments take precedence over earlier ones.
func (v Values) DeepMerge(sources ...Values) {
	if v == nil {
		return
	}
	for _, src := range sources {
		if src == nil {
			continue
		}
		for key, srcVal := range src {
			if dstVal, exists := v[key]; exists {
				// Both have the key, merge
				srcMap, srcIsMap := srcVal.(map[string]any)
				dstMap, dstIsMap := dstVal.(map[string]any)
				if srcIsMap && dstIsMap {
					// Both are maps, recur
					Values(dstMap).DeepMerge(srcMap)
				} else {
					// Not both maps, src overwrites dst
					v[key] = srcVal
				}
			} else {
				// Key only in src
				v[key] = srcVal
			}
		}
	}
}

// Extract retrieves a value from a nested Values map using dot notation path.
// Path format: ".key.subkey.value" where each dot represents a map level.
func (v Values) Extract(path Path) (any, error) {
	if err := path.Validate(); err != nil {
		return nil, err
	}

	// Fetch everything if given the root path "."
	if path == "." {
		return v, nil
	}

	// Split path into parts (remove leading dot first)
	parts := strings.Split(string(path)[1:], ".")

	// Traverse the nested map structure
	current := v
	for i, key := range parts {
		value, exists := current[key]
		if !exists {
			return nil, fmt.Errorf("key %q not found in path %s", key, path)
		}

		// If this is the final key, return the value
		if i == len(parts)-1 {
			return value, nil
		}

		// Otherwise, value must be a nested map to continue
		nextMap, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("cannot traverse path %s: key %q contains %T, expected map",
				path, key, value)
		}
		current = nextMap
	}
	return nil, fmt.Errorf("internal error: empty path components")
}

// Set takes a Values, a Path to a new or existing key, and any value and stores the newVal at the path.
// Special case: path "." merges the newVal's map contents directly into v (at the root).
func (v Values) Set(path Path, newVal any) error {
	if err := path.Validate(); err != nil {
		return err
	}

	// Handle root path "." - merge the value directly into the map
	if path == "." {
		if valueMap, ok := newVal.(map[string]any); ok {
			// If newVal is a map, merge its contents into v
			for k, val := range valueMap {
				v[k] = val
			}
			return nil
		}
		return fmt.Errorf("cannot merge non-map value at root path")
	}

	// Split path into parts (remove leading dot first)
	parts := strings.Split(string(path)[1:], ".")

	// Navigate to the nested location and set the value
	current := v
	for i, part := range parts {
		if i == len(parts)-1 {
			// Set the value at the last key in the path
			current[part] = newVal
		} else {
			if _, exists := current[part]; !exists {
				// If the part does not exist, create a new map for it
				current[part] = make(map[string]any)
			}

			nextMap, ok := current[part].(map[string]any)
			if !ok {
				return fmt.Errorf("conflict at %q, expected map but got %T", strings.Join(parts[:i+1], "."), current[part])
			}
			current = nextMap
		}
	}
	return nil
}

// Validate validates the Values against a JSON schema file at schemaPath.
func (v Values) Validate(ctx context.Context, schemaPath string) error {
	l := logger.From(ctx)
	start := time.Now()
	defer func() {
		l.Debug("schema validation complete",
			"duration", time.Since(start),
			"schemaPath", schemaPath)
	}()

	// Load the schema from file
	// Convert to absolute path and ensure forward slashes for URI
	absPath, err := filepath.Abs(schemaPath)
	if err != nil {
		return &SchemaValidationError{
			SchemaPath: schemaPath,
			Err:        fmt.Errorf("failed to resolve absolute path: %w", err),
		}
	}
	// Convert backslashes to forward slashes for file URI
	absPath = filepath.ToSlash(absPath)
	schemaLoader := gojsonschema.NewReferenceLoader("file:///" + absPath)

	// Convert Values to a document for validation
	documentLoader := gojsonschema.NewGoLoader(v)

	// Validate
	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return &SchemaValidationError{
			SchemaPath: schemaPath,
			Err:        fmt.Errorf("failed to load or parse schema: %w", err),
		}
	}

	// Check if validation passed
	if !result.Valid() {
		return &SchemaValidationError{
			SchemaPath: schemaPath,
			Errors:     result.Errors(),
		}
	}

	return nil
}

// SchemaValidationError represents an error when JSON schema validation fails
type SchemaValidationError struct {
	SchemaPath string
	Errors     []gojsonschema.ResultError
	Err        error
}

func (e *SchemaValidationError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("schema validation failed for %s: %v", e.SchemaPath, e.Err)
	}
	if len(e.Errors) > 0 {
		var errMsgs []string
		for _, err := range e.Errors {
			errMsgs = append(errMsgs, err.String())
		}
		return fmt.Sprintf("schema validation failed for %s:\n%s", e.SchemaPath, strings.Join(errMsgs, "\n"))
	}
	return fmt.Sprintf("schema validation failed for %s", e.SchemaPath)
}

func (e *SchemaValidationError) Unwrap() error {
	return e.Err
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
