package pipeline

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/buildkite/go-pipeline/ordered"
	"gopkg.in/yaml.v3"
)

var _ interface {
	json.Marshaler
	json.Unmarshaler
	ordered.Unmarshaler
} = (*CommandStep)(nil)

// Signature models a signature (on a step, etc).
type Signature struct {
	Algorithm    string   `json:"algorithm" yaml:"algorithm"`
	SignedFields []string `json:"signed_fields" yaml:"signed_fields"`
	Value        string   `json:"value" yaml:"value"`
}

// CommandStep models a command step.
//
// Standard caveats apply - see the package comment.
type CommandStep struct {
	// Fields common to various step types
	Key   string `yaml:"key,omitempty" aliases:"id,identifier"`
	Label string `yaml:"label,omitempty" aliases:"name"`

	// Fields that are meaningful specifically for command steps
	Command   string            `yaml:"command"`
	Plugins   Plugins           `yaml:"plugins,omitempty"`
	Secrets   Secrets           `yaml:"secrets,omitempty"`
	Env       map[string]string `yaml:"env,omitempty"`
	Signature *Signature        `yaml:"signature,omitempty"`
	Matrix    *Matrix           `yaml:"matrix,omitempty"`
	Cache     *Cache            `yaml:"cache,omitempty"`

	// RemainingFields stores any other top-level mapping items so they at least
	// survive an unmarshal-marshal round-trip.
	RemainingFields map[string]any `yaml:",inline"`
}

// MarshalJSON marshals the step to JSON. Special handling is needed because
// yaml.v3 has "inline" but encoding/json has no concept of it.
func (c *CommandStep) MarshalJSON() ([]byte, error) {
	return inlineFriendlyMarshalJSON(c)
}

// UnmarshalJSON is used when unmarshalling an individual step directly, e.g.
// from the Agent API Accept Job.
func (c *CommandStep) UnmarshalJSON(b []byte) error {
	// JSON is just a specific kind of YAML.
	var n yaml.Node
	if err := yaml.Unmarshal(b, &n); err != nil {
		return err
	}
	return ordered.Unmarshal(&n, &c)
}

// UnmarshalOrdered unmarshals a command step from an ordered map.
func (c *CommandStep) UnmarshalOrdered(src any) error {
	type wrappedCommand CommandStep
	// Unmarshal into this secret type, then process special fields specially.
	fullCommand := new(struct {
		Commands []string `yaml:"commands" aliases:"command"`

		// Use inline trickery to capture the rest of the struct.
		Rem *wrappedCommand `yaml:",inline"`
	})
	fullCommand.Rem = (*wrappedCommand)(c)
	if err := ordered.Unmarshal(src, fullCommand); err != nil {
		return fmt.Errorf("unmarshalling CommandStep: %w", err)
	}

	// Normalise cmds into one single command string.
	// This makes signing easier later on - it's easier to hash one
	// string consistently than it is to pick apart multiple strings
	// in a consistent way in order to hash all of them
	// consistently.
	c.Command = strings.Join(fullCommand.Commands, "\n")
	return nil
}

// InterpolateMatrixPermutation validates and then interpolates the choice of
// matrix values into the step. This should only be used in order to validate
// a job that's about to be run, and not used before pipeline upload.
func (c *CommandStep) InterpolateMatrixPermutation(mp MatrixPermutation) error {
	if err := c.Matrix.validatePermutation(mp); err != nil {
		return err
	}
	if len(mp) == 0 {
		return nil
	}
	return c.interpolate(newMatrixInterpolator(mp))
}

func (c *CommandStep) interpolate(tf stringTransformer) error {
	// Fields that are interpolated with env vars and matrix tokens:
	// command, plugins, secrets
	if err := interpolateString(tf, &c.Command); err != nil {
		return fmt.Errorf("interpolating command: %w", err)
	}
	if err := interpolateString(tf, &c.Label); err != nil {
		return fmt.Errorf("interpolating label: %w", err)
	}
	if err := interpolateSlice(tf, c.Plugins); err != nil {
		return fmt.Errorf("interpolating plugins: %w", err)
	}
	if err := interpolateSlice(tf, c.Secrets); err != nil {
		return fmt.Errorf("interpolating secrets: %w", err)
	}

	switch tf.(type) {
	case envInterpolator:
		// Env interpolation applies to nearly everything:
		// key, depends_on, env (keys and values), matrix
		if err := interpolateString(tf, &c.Key); err != nil {
			return fmt.Errorf("interpolating key: %w", err)
		}
		if err := interpolateMap(tf, c.Env); err != nil {
			return fmt.Errorf("interpolating env: %w", err)
		}
		if err := c.Matrix.interpolate(tf); err != nil {
			return fmt.Errorf("interpolating matrix: %w", err)
		}

	case matrixInterpolator:
		// Matrix interpolation applies only to some things, but particularly
		// only affects env values (not env keys).
		if err := interpolateMapValues(tf, c.Env); err != nil {
			return fmt.Errorf("interpolating env values: %w", err)
		}
	}

	// NB: Do not interpolate Signature.

	if err := interpolateMap(tf, c.RemainingFields); err != nil {
		return fmt.Errorf("interpolating remaining fields: %w", err)
	}

	return nil
}

// MergeSecretsFromPipeline merges pipeline-level secrets with this step's secrets.
// Step-level secrets take precedence over pipeline-level secrets for deduplication.
func (c *CommandStep) MergeSecretsFromPipeline(pipelineSecrets Secrets) {
	c.Secrets = pipelineSecrets.MergeWith(c.Secrets)
}

func (CommandStep) stepTag() {}
