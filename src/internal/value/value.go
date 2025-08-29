package value

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/goccy/go-yaml"
	"github.com/zarf-dev/zarf/src/pkg/logger"
)

type Values map[string]any

// ParseFiles parses the given files in order, overwriting previous values with later values, and returns a merged
// Values map
// TODO: Add schema check. Maybe here in parsing, or later in the process like templating?
func ParseFiles(ctx context.Context, paths []string) (_ Values, err error) {
	l := logger.From(ctx)
	m := make(Values)

	// No files given
	if len(paths) <= 0 {
		return map[string]any{}, nil
	}

	// Ensure files exist
	l.Debug("parsing values files", "paths", paths)
	for _, path := range paths {
		// REVIEW: Do we care about empty files? Here? Small UX tradeoff whether or not to fail on empty files
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		vals, err := parseFile(ctx, path)
		if err != nil {
			return nil, err
		}
		newM := deepMergeValues(m, vals)
		m = newM
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
			err = fmt.Errorf("%w:%w", closeErr, err)
		}
	}(f)

	// Decode and merge values
	err = yaml.NewDecoder(f).DecodeContext(ctx, &m)
	if err != nil {
		// an empty file is fine
		if errors.Is(err, io.EOF) {
			return m, nil
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
func checkSchema_Stub(values Values, jsonSchema string) []error {
	return nil
}

// deepMergeValues merges two Values maps recursively, overwriting keys in dst with keys from src
func deepMergeValues(dst, src Values) Values {
	for key, srcVal := range src {
		if dstVal, exists := dst[key]; exists {
			// Both have the key, merge
			srcMap, srcIsMap := srcVal.(map[string]any)
			dstMap, dstIsMap := dstVal.(map[string]any)
			if srcIsMap && dstIsMap {
				// Both are maps, recur
				deepMergeValues(dstMap, srcMap)
			} else {
				// Not both maps, src overwrites dst
				dst[key] = srcVal
			}
		} else {
			// Key only in src
			dst[key] = srcVal
		}
	}
	return dst
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
