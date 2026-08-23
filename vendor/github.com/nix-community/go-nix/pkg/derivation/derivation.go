package derivation

import (
	"fmt"

	"github.com/nix-community/go-nix/pkg/storepath"
)

// Derivation describes all data in a .drv, which canonically is expressed in ATerm format.
// Nix requires some stronger properties w.r.t. order of elements, so we can internally use
// maps for some of the fields, and convert to the canonical representation when encoding back
// to ATerm format.
// The field names (and order of fields) also match the json structure
// that the `nix show-derivation /path/to.drv` is using,
// even though this might change in the future.
type Derivation struct {
	// Structured don't have the env name right in the regular spot but in the nested JSON object.
	// This is an internal variable only used for structured attrs derivations, which can currently only be created
	// from an existing drv file.
	name string

	// Outputs are always lexicographically sorted by their name (key in this map)
	Outputs map[string]*Output `json:"outputs"`

	// InputSources are always lexicographically sorted.
	InputSources []string `json:"inputSrcs"`

	// InputDerivations are always lexicographically sorted by their path (key in this map)
	// the []string returns the output names (out, …) of this input derivation that are used.
	InputDerivations map[string][]string `json:"inputDrvs"`

	Platform string `json:"system"`

	Builder string `json:"builder"`

	Arguments []string `json:"args"`

	// Env must be lexicographically sorted by their key.
	Env map[string]string `json:"env"`
}

func (d *Derivation) Validate() error {
	numberOfOutputs := len(d.Outputs)

	if numberOfOutputs == 0 {
		return fmt.Errorf("at least one output must be defined")
	}

	for outputName, output := range d.Outputs {
		if outputName == "" {
			return fmt.Errorf("empty output name")
		}

		// TODO: are there more restrictions on output names?

		// we encountered a fixed-output output
		// In these derivations, there may be only one output,
		// which needs to be called out
		if output.HashAlgorithm != "" {
			if numberOfOutputs != 1 {
				return fmt.Errorf("encountered fixed-output, but there's more than 1 output in total")
			}

			if outputName != "out" {
				return fmt.Errorf("the fixed-output output name must be called 'out'")
			}

			// we confirmed above there's only one output, so we're done with the loop
			break
		}

		err := output.Validate()
		if err != nil {
			return fmt.Errorf("error validating output '%s': %w", outputName, err)
		}
	}

	for inputDerivationPath := range d.InputDerivations {
		err := storepath.Validate(inputDerivationPath)
		if err != nil {
			return err
		}

		outputNames := d.InputDerivations[inputDerivationPath]
		if len(outputNames) == 0 {
			return fmt.Errorf("output names list for '%s' empty", inputDerivationPath)
		}

		for i, o := range outputNames {
			if i > 0 && o < outputNames[i-1] {
				return fmt.Errorf("invalid input derivation output order: %s < %s", o, outputNames[i-1])
			}

			if o == "" {
				return fmt.Errorf("Output name entry for '%s' empty", inputDerivationPath)
			}
		}
	}

	for i, is := range d.InputSources {
		err := storepath.Validate(is)
		if err != nil {
			return fmt.Errorf("error validating input source '%s': %w", is, err)
		}

		if i > 0 && is < d.InputSources[i-1] {
			return fmt.Errorf("invalid input source order: %s < %s", is, d.InputSources[i-1])
		}
	}

	if d.Platform == "" {
		return fmt.Errorf("required attribute 'platform' missing")
	}

	if d.Builder == "" {
		return fmt.Errorf("required attribute 'builder' missing")
	}

	// there has to be an env variable with key `name`.
	hasNameEnv := false

	for k := range d.Env {
		if k == "" {
			return fmt.Errorf("empty environment variable key")
		}

		if k == "name" {
			hasNameEnv = true
		}

		// Structured attrs
		if k == "__json" {
			hasNameEnv = d.name != ""
		}
	}

	if !hasNameEnv {
		return fmt.Errorf("env 'name' not found")
	}

	return nil
}

func (d *Derivation) Name() string {
	if _, ok := d.Env["__json"]; ok {
		return d.name
	}

	name, ok := d.Env["name"]
	if ok {
		return name
	}

	return ""
}
