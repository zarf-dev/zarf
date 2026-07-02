package pipeline

import (
	"encoding/json"
	"fmt"

	"github.com/buildkite/go-pipeline/ordered"
	"gopkg.in/yaml.v3"
)

var _ interface {
	json.Unmarshaler
	ordered.Unmarshaler
} = (*Plugins)(nil)

// Plugins is a sequence of plugins. It is useful for unmarshaling.
type Plugins []*Plugin

// UnmarshalOrdered unmarshals Plugins from either
//   - []any - originally a sequence of "one-item mappings" (normal form), or
//   - *ordered.MapSA - a mapping (where order is important...non-normal form).
//
// "plugins" is supposed to be a sequence of one-item maps, since order matters.
// But some people (even us) write plugins into one big mapping and rely on
// order preservation.
func (p *Plugins) UnmarshalOrdered(o any) error {
	// Whether processing one big map, or a sequence of small maps, the central
	// part remains the same.
	// Parse each "key: value" as "name: config", then append in order.
	unmarshalMap := func(m *ordered.MapSA) error {
		return m.Range(func(k string, v any) error {
			// ToMapRecursive demolishes any ordering within the plugin config.
			// This is needed because the backend likes to reorder the keys,
			// and for signing we need the JSON form to be stable.
			plugin := &Plugin{
				Source: k,
				Config: ordered.ToMapRecursive(v),
			}
			*p = append(*p, plugin)
			return nil
		})
	}

	switch o := o.(type) {
	case nil:
		// Someone wrote `plugins: null` (possibly us).
		*p = nil
		return nil

	case []any:
		for _, c := range o {
			switch ct := c.(type) {
			case *ordered.MapSA:
				// Typical case:
				//
				// plugins:
				//   - plugin#1.0.0:
				//       config: config, etc
				if err := unmarshalMap(ct); err != nil {
					return err
				}

			case string:
				// Less typical, but supported:
				//
				// plugins:
				//   - plugin#1.0.0
				// (no config, only plugin)
				*p = append(*p, &Plugin{
					Source: ct,
					Config: nil,
				})

			default:
				return fmt.Errorf("unmarshaling plugins: plugin type %T, want *ordered.Map[string, any] or string", c)
			}
		}

	case *ordered.MapSA:
		// Legacy form:
		//
		// plugins:
		//   plugin#1.0.0:
		//     config: config, etc
		//   otherplugin#2.0.0:
		//     etc
		if err := unmarshalMap(o); err != nil {
			return err
		}

	default:
		return fmt.Errorf("unmarshaling plugins: got %T, want []any or *ordered.Map[string, any]", o)

	}
	return nil
}

// UnmarshalJSON is used mainly to normalise the BUILDKITE_PLUGINS env var.
func (p *Plugins) UnmarshalJSON(b []byte) error {
	// JSON is just a specific kind of YAML.
	var n yaml.Node
	if err := yaml.Unmarshal(b, &n); err != nil {
		return err
	}
	return ordered.Unmarshal(&n, &p)
}
