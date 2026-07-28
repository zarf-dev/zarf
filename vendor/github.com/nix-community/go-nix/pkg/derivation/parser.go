package derivation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

var (
	derivationPrefix  = []byte("Derive") //nolint:gochecknoglobals
	errArrayNotClosed = fmt.Errorf("array not closed")
)

//nolint:gochecknoglobals
var stringUnescaper = strings.NewReplacer(
	"\\\\", "\\",
	"\\n", "\n",
	"\\r", "\r",
	"\\t", "\t",
	"\\\"", "\"",
)

// ReadDerivation parses a Derivation in ATerm format and returns the Derivation struct,
// or an error in case any parsing error occurs, or some of the fields would be illegal.
func ReadDerivation(reader io.Reader) (*Derivation, error) {
	bytes, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	drv, err := parseDerivation(bytes)
	if err != nil {
		return nil, err
	}

	return drv, drv.Validate()
}

// parseDerivation provides a derivation parser that works without any memory allocations.
// It does so by walking the byte slice recursively and calling a callback for every array item found
// with the array item sub-sliced from the passed slice.
// During parsing, it checks for some invalid inputs (e.g. maps in the wrong order) that won't be
// recognizable in the returned struct.
// Other checks are handled by Derivation.Validate(),
// which is called by ReadDerivation() after parseDerivation().
func parseDerivation(derivationBytes []byte) (*Derivation, error) {
	if len(derivationBytes) < 8 {
		return nil, fmt.Errorf("input too short to be a valid derivation")
	}

	if !bytes.Equal(derivationBytes[:6], derivationPrefix) {
		return nil, fmt.Errorf("missing derivation prefix")
	}

	drv := &Derivation{}

	// https://github.com/golang/go/issues/37711
	drv.InputSources = []string{}
	drv.Arguments = []string{}

	err := arrayEach(derivationBytes[6:], func(value []byte, index int) error {
		var err error

		switch index {
		case 0: // Outputs
			drv.Outputs = make(map[string]*Output)
			// Outputs are always lexicographically sorted by their name.
			// keep track of the previous path read (if any), so we detect
			// invalid encodings.
			prevOutputName := ""
			err = arrayEach(value, func(value []byte, _ int) error {
				output := &Output{}
				outputName := ""

				// Get every output field
				err := arrayEach(value, func(value []byte, index int) error {
					var err error

					switch index {
					case 0:
						outputName, err = unquoteSlice(value)
						if err != nil {
							return err
						}

						if outputName <= prevOutputName {
							return fmt.Errorf("invalid output order, %s <= %s", outputName, prevOutputName)
						}
					case 1:
						output.Path, err = unquoteSlice(value)
						if err != nil {
							return err
						}
					case 2:
						output.HashAlgorithm, err = unquoteSlice(value)
						if err != nil {
							return err
						}
					case 3:
						output.Hash, err = unquoteSlice(value)
						if err != nil {
							return err
						}
					default:
						return fmt.Errorf("unhandled output index: %d", index)
					}

					return nil
				})
				if err != nil {
					return err
				}

				if outputName == "" {
					return fmt.Errorf("output name for %s may not be empty", output.Path)
				}

				drv.Outputs[outputName] = output
				prevOutputName = outputName

				return nil
			})

		case 1: // InputDerivations
			drv.InputDerivations = make(map[string][]string)
			// InputDerivations are always lexicographically sorted by their path
			prevInputDrvPath := ""
			err = arrayEach(value, func(value []byte, _ int) error {
				inputDrvPath := ""
				inputDrvNames := []string{}

				err := arrayEach(value, func(value []byte, index int) error {
					var err error

					switch index {
					case 0:
						inputDrvPath, err = unquoteSlice(value)
						if err != nil {
							return err
						}

						if inputDrvPath <= prevInputDrvPath {
							return fmt.Errorf("invalid input derivation order: %s <= %s", inputDrvPath, prevInputDrvPath)
						}

					case 1:
						err := arrayEach(value, func(value []byte, _ int) error {
							unquoted, err := unquoteSlice(value)
							if err != nil {
								return err
							}

							inputDrvNames = append(inputDrvNames, unquoted)

							return nil
						})
						if err != nil {
							return err
						}

					default:
						return fmt.Errorf("unhandled input derivation index: %d", index)
					}

					return nil
				})
				if err != nil {
					return err
				}

				drv.InputDerivations[inputDrvPath] = inputDrvNames
				prevInputDrvPath = inputDrvPath

				return nil
			})

		case 2: // InputSources
			err = arrayEach(value, func(value []byte, _ int) error {
				unquoted, err := unquoteSlice(value)
				if err != nil {
					return err
				}

				drv.InputSources = append(drv.InputSources, unquoted)

				return nil
			})

		case 3: // Platform
			drv.Platform, err = unquoteSlice(value)

		case 4: // Builder
			drv.Builder, err = unquoteSlice(value)

		case 5: // Arguments
			err = arrayEach(value, func(value []byte, _ int) error {
				unquoted, err := unquoteSlice(value)
				if err != nil {
					return err
				}

				drv.Arguments = append(drv.Arguments, unquoted)

				return nil
			})

		case 6: // Env
			drv.Env = make(map[string]string)
			prevEnvKey := ""
			err = arrayEach(value, func(value []byte, _ int) error {
				envValue := ""
				envKey := ""

				// For every field
				err := arrayEach(value, func(value []byte, index int) error {
					var err error

					switch index {
					case 0:
						envKey, err = unquoteSlice(value)
						if err != nil {
							return err
						}

						if envKey <= prevEnvKey {
							return fmt.Errorf("invalid env var order: %s <= %s", envKey, prevEnvKey)
						}
					case 1:
						envValue, err = unquote(value)
						if err != nil {
							return err
						}
					default:
						return fmt.Errorf("unhandled env var index: %d", index)
					}

					return nil
				})
				if err != nil {
					return err
				}

				drv.Env[envKey] = envValue
				prevEnvKey = envKey

				return err
			})

		default:
			return fmt.Errorf("unhandled derivation index: %d", index)
		}

		return err
	})
	if err != nil {
		return nil, err
	}

	// Handle structured attrs which doesn' have the name variable in the env directly
	// but in the nested JSON object.
	if structuredJSON, ok := drv.Env["__json"]; ok {
		attrs := &struct {
			Name string
		}{}

		err := json.Unmarshal([]byte(structuredJSON), &attrs)
		if err != nil {
			return nil, err
		}

		drv.name = attrs.Name
	}

	return drv, nil
}

// arrayEach - Call callback method for every array item found in byte slice.
func arrayEach(value []byte, callback func(value []byte, index int) error) error {
	if len(value) < 2 { // Empty array
		return fmt.Errorf("array too short")
	} else if len(value) == 2 {
		return nil
	}

	switch value[0] {
	case '(':
		if value[len(value)-1] != ')' {
			return errArrayNotClosed
		}

	case '[':
		if value[len(value)-1] != ']' {
			return errArrayNotClosed
		}

	default:
		return fmt.Errorf("invalid array opening character: %q", value[0])
	}

	count := 0 // Open paren count
	start := 1 // Start of next value
	idx := 0   // Array index

	escaped := false
	inString := false

	for i, c := range value {
		if escaped { // If value is escaped skip this iteration
			escaped = false

			continue
		} else if c == '\\' { // Set escaped state
			escaped = true

			continue
		}

		if c == '"' {
			inString = !inString

			continue
		} else if inString {
			continue
		}

		if (count == 1 && c == ',') || i == len(value)-1 {
			err := callback(value[start:i], idx)
			if err != nil {
				return err
			}

			idx++ // Array index

			start = i + 1 // Offset to next value
		}

		switch c {
		case '[':
			count++

			continue
		case ']':
			count--

			continue
		case '(':
			count++

			continue
		case ')':
			count--

			continue
		}
	}

	return nil
}

// Unquote a byte slice.
func unquote(b []byte) (string, error) {
	s, err := unquoteSlice(b)
	if err != nil {
		return "", err
	}

	// If the value doesn't contain escaped sequences avoid some extra allocations
	if !bytes.ContainsRune(b, '\\') {
		return s, nil
	}

	return stringUnescaper.Replace(s), nil
}

// Unquote a byte slice by simply removing the first and last characters.
// This should only be used in places where an escaped character is invalid.
func unquoteSlice(b []byte) (string, error) {
	if len(b) < 2 {
		return "", fmt.Errorf("invalid quoted string length: %d", len(b))
	}

	if b[0] != '"' || b[len(b)-1] != '"' {
		return "", fmt.Errorf("string not quoted: %s", string(b))
	}

	b = b[1 : len(b)-1]

	return unsafeString(b), nil
}
