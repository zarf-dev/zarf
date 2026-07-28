package derivation

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

//nolint:gochecknoglobals
var stringEscaper = strings.NewReplacer(
	"\\", "\\\\",
	"\n", "\\n",
	"\r", "\\r",
	"\t", "\\t",
	"\"", "\\\"",
)

//nolint:gochecknoglobals
var (
	comma        = []byte{','}
	parenOpen    = []byte{'('}
	parenClose   = []byte{')'}
	bracketOpen  = []byte{'['}
	bracketClose = []byte{']'}
	quoteC       = []byte{'"'}
)

// Adds quotation marks around a string while escaping it.
func escapeString(s string) string {
	s = stringEscaper.Replace(s)

	return "\"" + s + "\""
}

// Like escapeString but returns the underlying byte slice.
func escapeStringB(s string) []byte {
	return unsafeBytes(escapeString(s))
}

// Write a list of elements staring with `opening` character and ending with a `closing` character.
func writeArrayElems(writer io.Writer, quote bool, open []byte, closing []byte, elems ...string) error {
	var err error

	if _, err = writer.Write(open); err != nil {
		return err
	}

	for i, elem := range elems {
		if i > 0 {
			if _, err = writer.Write(comma); err != nil {
				return err
			}
		}

		if quote {
			if _, err = writer.Write(quoteC); err != nil {
				return err
			}
		}

		if _, err = writer.Write(unsafeBytes(elem)); err != nil {
			return err
		}

		if quote {
			if _, err = writer.Write(quoteC); err != nil {
				return err
			}
		}
	}

	if _, err = writer.Write(closing); err != nil {
		return err
	}

	return nil
}

// WriteDerivation writes the ATerm representation of the derivation to the passed writer.
func (d *Derivation) WriteDerivation(writer io.Writer) error {
	return d.writeDerivation(writer, false, nil)
}

// writeDerivation writes the ATerm representation of the derivation to the passed writer.
// Optionally, the following transformations can be made while writing out the ATerm:
//
//   - stripOutput will replace output hashes in `Outputs` (`Output[$outputName]`),
//     and `env[$outputName]` with empty strings
//
//   - inputDrvReplacements (map[$drvPath]$replacement) can be provided.
//     If set, it must contain all derivation path in d.InputDerivations[*]
//     These will be replaced with their replacement value.
//     As this will change map keys, and map keys need to be serialized alphabetically sorted,
//     this will shuffle the order of values.
//
// This replacement/stripping is only used when calculating output hashes.
// Set to false / nil in normal mode.
func (d *Derivation) writeDerivation(
	writer io.Writer,
	stripOutputs bool,
	inputDrvReplacements map[string]string,
) error {
	// To order outputs by their output name (which is the key of the map), we
	// get the keys, sort them, then add each one by one.
	outputNames := make([]string, len(d.Outputs))
	{
		i := 0

		for k := range d.Outputs {
			outputNames[i] = k
			i++
		}

		sort.Strings(outputNames)
	}

	// If inputDrvReplacements are provided, populate a new map
	// if they are not, provide an alias to the existing one
	var inputDerivations map[string][]string
	if len(inputDrvReplacements) == 0 {
		inputDerivations = d.InputDerivations
	} else {
		inputDerivations = make(map[string][]string, len(d.InputDerivations))
		// walk over d.InputDerivations.
		// Check if there's a match in inputDrvReplacements, and if so, replace
		// it with that.
		// If there's no match, this means we were called wrongly
		for drvPath, outputNames := range d.InputDerivations {
			replacement, ok := inputDrvReplacements[drvPath]
			if !ok {
				return fmt.Errorf("unable to find replacement for %s, but replacement requested", replacement)
			}

			inputDerivations[replacement] = outputNames
		}
	}

	// input derivations are sorted by their path, which is the key of the map.
	// get the list of keys, sort them, then add each one by one.
	inputDerivationPaths := make([]string, len(inputDerivations))
	{
		i := 0

		for inputDerivationPath := range inputDerivations {
			inputDerivationPaths[i] = inputDerivationPath
			i++
		}

		sort.Strings(inputDerivationPaths)
	}

	// environment variables need to be sorted by their key.
	// extract the list of keys, sort them, then add each one by one
	envKeys := make([]string, len(d.Env))
	{
		i := 0

		for k := range d.Env {
			envKeys[i] = k
			i++
		}

		sort.Strings(envKeys)
	}

	// Derivation prefix (Derive)
	if _, err := writer.Write(derivationPrefix); err != nil {
		return err
	}

	// Open Derive call
	if _, err := writer.Write(parenOpen); err != nil {
		return err
	}

	// Outputs
	{
		if _, err := writer.Write(bracketOpen); err != nil {
			return err
		}

		for i, outputName := range outputNames {
			if i > 0 {
				_, err := writer.Write(comma)
				if err != nil {
					return err
				}
			}

			o := d.Outputs[outputName]

			encPath := o.Path
			if stripOutputs {
				encPath = ""
			}

			if err := writeArrayElems(
				writer,
				true,
				parenOpen,
				parenClose,
				outputName,
				encPath,
				o.HashAlgorithm,
				o.Hash,
			); err != nil {
				return err
			}
		}

		if _, err := writer.Write(bracketClose); err != nil {
			return err
		}
	}

	// Input derivations
	{
		if _, err := writer.Write(comma); err != nil {
			return err
		}

		if _, err := writer.Write(bracketOpen); err != nil {
			return err
		}

		{
			for i, inputDerivationPath := range inputDerivationPaths {
				if i > 0 {
					if _, err := writer.Write(comma); err != nil {
						return err
					}
				}

				if _, err := writer.Write(parenOpen); err != nil {
					return err
				}

				if _, err := writer.Write(quoteC); err != nil {
					return err
				}

				if _, err := writer.Write(unsafeBytes(inputDerivationPath)); err != nil {
					return err
				}

				if _, err := writer.Write(quoteC); err != nil {
					return err
				}

				if _, err := writer.Write(comma); err != nil {
					return err
				}

				if err := writeArrayElems(
					writer,
					true,
					bracketOpen,
					bracketClose,
					inputDerivations[inputDerivationPath]...,
				); err != nil {
					return err
				}

				if _, err := writer.Write(parenClose); err != nil {
					return err
				}
			}
		}

		if _, err := writer.Write(bracketClose); err != nil {
			return err
		}
	}

	// Input sources
	{
		if _, err := writer.Write(comma); err != nil {
			return err
		}

		if err := writeArrayElems(writer, true, bracketOpen, bracketClose, d.InputSources...); err != nil {
			return err
		}
	}

	// Platform
	{
		if _, err := writer.Write(comma); err != nil {
			return err
		}

		if _, err := writer.Write(escapeStringB(d.Platform)); err != nil {
			return err
		}
	}

	// Builder
	{
		if _, err := writer.Write(comma); err != nil {
			return err
		}

		if _, err := writer.Write(escapeStringB(d.Builder)); err != nil {
			return err
		}
	}

	// Arguments
	{
		if _, err := writer.Write(comma); err != nil {
			return err
		}

		if err := writeArrayElems(writer, true, bracketOpen, bracketClose, d.Arguments...); err != nil {
			return err
		}
	}

	// Env
	{
		if _, err := writer.Write(comma); err != nil {
			return err
		}

		if _, err := writer.Write(bracketOpen); err != nil {
			return err
		}

		for i, key := range envKeys {
			if i > 0 {
				if _, err := writer.Write(comma); err != nil {
					return err
				}
			}

			value := d.Env[key]
			// when stripOutputs is set, we need to strip all env keys
			// that are named like an output.
			if stripOutputs {
				if _, ok := d.Outputs[key]; ok {
					value = ""
				}
			}

			if err := writeArrayElems(writer, false, parenOpen, parenClose, "\""+key+"\"", escapeString(value)); err != nil {
				return err
			}
		}

		if _, err := writer.Write(bracketClose); err != nil {
			return err
		}
	}

	// Close Derive call
	if _, err := writer.Write(parenClose); err != nil {
		return err
	}

	return nil
}
