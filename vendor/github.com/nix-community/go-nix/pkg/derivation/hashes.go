package derivation

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	chash "hash"

	"github.com/nix-community/go-nix/pkg/nixhash"
	"github.com/nix-community/go-nix/pkg/storepath"
)

//nolint:gochecknoglobals
var colon = []byte{':'}

// getMaskedATermHash returns the hex-representation of
// In case the Derivation is not just a fixed-output derivation,
// calculating the output hashes includes all inputs derivations.
//
// This is done by hashing a special ATerm variant.
// In this variant, all output paths, and environment variables
// named like output names are set to an empty string,
// aka "not calculated yet".
//
// Input derivation are replaced with a hex-replacement string,
// which is calculated by CalculateDrvReplacement,
// but passed in as a map here (we don't want to always recurse, but precompute).
func (d *Derivation) getMaskedATermHash(inputDrvReplacements map[string]string) (string, error) {
	h := sha256.New()

	err := d.writeDerivation(h, true, inputDrvReplacements)
	if err != nil {
		return "", fmt.Errorf("error writing masked ATerm: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func hashStrings(h chash.Hash, strings ...string) []byte {
	h.Write(unsafeBytes(strings[0]))

	for _, s := range strings[1:] {
		h.Write(colon)
		h.Write(unsafeBytes(s))
	}

	return h.Sum(nil)
}

// CalculateOutputPaths calculates the output paths of all outputs
// It consumes a list of input derivation path replacements.
func (d *Derivation) CalculateOutputPaths(inputDrvReplacements map[string]string) (map[string]string, error) {
	derivationName := d.Name()

	if derivationName == "" {
		// asserted by Validate
		panic("env 'name' not found")
	}

	outputPaths := make(map[string]string, len(d.Outputs))

	h := sha256.New()

	for outputName, o := range d.Outputs {
		// calculate the part of an output path that comes after the hash
		var outputPathName string
		if outputName == "out" {
			outputPathName = derivationName
		} else {
			outputPathName = derivationName + "-" + outputName
		}

		var storeHash []byte

		if o.HashAlgorithm != "" {
			// This code is _weird_ but it is what Nix is doing. See:
			// https://github.com/NixOS/nix/blob/1385b2007804c8a0370f2a6555045a00e34b07c7/src/libstore/store-api.cc#L178-L196
			if o.HashAlgorithm == "r:sha256" {
				storeHash = hashStrings(
					h,
					"source",
					"sha256",
					o.Hash,
					storepath.StoreDir,
					derivationName,
				)
			} else {
				fixedHex := hex.EncodeToString(hashStrings(h, "fixed", "out", o.HashAlgorithm, o.Hash, ""))

				h.Reset()

				storeHash = hashStrings(
					h,
					"output",
					"out",
					"sha256",
					fixedHex,
					storepath.StoreDir,
					derivationName,
				)
			}
		} else {
			maskedATermHash, err := d.getMaskedATermHash(inputDrvReplacements)
			if err != nil {
				return nil, fmt.Errorf("failed to calculate masked ATerm hash: %w", err)
			}

			storeHash = hashStrings(
				h,
				"output",
				outputName,
				"sha256",
				maskedATermHash,
				storepath.StoreDir,
				outputPathName,
			)
		}

		calculatedPath := storepath.StorePath{
			Name:   outputPathName,
			Digest: nixhash.CompressHash(storeHash, 20),
		}

		outputPaths[outputName] = calculatedPath.Absolute()

		h.Reset()
	}

	return outputPaths, nil
}

// CalculateDrvReplacement calculates the hex-replacement string for a derivation.
// When calculating output paths with Derivation.CalculateOutputPaths(),
// for a non-fixed-output derivation, a map of replacements (each calculated by this function)
// needs to be passed in.
//
// To calculate replacement strings of non-fixed-output derivations,
// *their* input derivation replacements also need to be known - so
// the calculation would be recursive.
//
// We solve this having calculateDrvReplacement accept a map of
// /its/ replacements, instead of recursing.
func (d *Derivation) CalculateDrvReplacement(inputDrvReplacements map[string]string) (string, error) {
	// Check if we're a fixed output
	if len(d.Outputs) == 1 {
		// Is it fixed output?
		if o, ok := d.Outputs["out"]; ok && o.HashAlgorithm != "" {
			return hex.EncodeToString(hashStrings(
				sha256.New(),
				"fixed",
				"out",
				o.HashAlgorithm,
				o.Hash,
				o.Path,
			)), nil
		}
	}

	h := sha256.New()

	err := d.writeDerivation(h, false, inputDrvReplacements)
	if err != nil {
		return "", fmt.Errorf("error hashing ATerm: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
