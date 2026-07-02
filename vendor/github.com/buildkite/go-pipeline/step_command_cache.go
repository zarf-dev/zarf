package pipeline

import (
	"encoding/json"
	"fmt"

	"github.com/buildkite/go-pipeline/ordered"
)

var _ interface {
	json.Marshaler
	ordered.Unmarshaler
} = (*Cache)(nil)

var (
	errUnsupportedCacheType = fmt.Errorf("unsupported type for cache")
)

// Cache models the cache settings for a given step
type Cache struct {
	Disabled bool     `yaml:",omitempty"`
	Name     string   `yaml:"name,omitempty"`
	Paths    []string `yaml:"paths,omitempty"`
	Size     string   `yaml:"size,omitempty"`

	RemainingFields map[string]any `yaml:",inline"`
}

// MarshalJSON marshals the step to JSON. Special handling is needed because
// yaml.v3 has "inline" but encoding/json has no concept of it.
func (c *Cache) MarshalJSON() ([]byte, error) {
	if c.Disabled {
		return json.Marshal(false)
	}
	return inlineFriendlyMarshalJSON(c)
}

// UnmarshalOrdered unmarshals from the following types:
// - string: a single path
// - []string: multiple paths
// - ordered.Map: a map containing paths, among potentially other things
func (c *Cache) UnmarshalOrdered(o any) error {
	switch v := o.(type) {
	case bool:
		if !v {
			c.Disabled = true
		}
	case string:
		c.Paths = []string{v}

	case []any:
		s := make([]string, 0, len(v))
		if err := ordered.Unmarshal(v, &s); err != nil {
			return err
		}

		c.Paths = s

	case *ordered.MapSA:
		type wrappedCache Cache
		if err := ordered.Unmarshal(o, (*wrappedCache)(c)); err != nil {
			return err
		}

	default:
		return fmt.Errorf("%w: %T", errUnsupportedCacheType, v)
	}

	return nil
}
