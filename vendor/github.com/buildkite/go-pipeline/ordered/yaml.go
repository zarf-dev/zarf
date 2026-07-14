package ordered

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// DecodeYAML recursively unmarshals n into a generic type (any, []any, or
// *Map[string, any]) depending on the kind of n. Where yaml.v3 typically infer
// map[string]any for unmarshaling mappings into any, DecodeYAML chooses
// *Map[string, any] instead.
func DecodeYAML(n *yaml.Node) (any, error) {
	return decodeYAML(make(map[*yaml.Node]bool), n)
}

// decode recursively unmarshals n into a generic type (any, []any, or
// *Map[string, any]) depending on the kind of n.
func decodeYAML(seen map[*yaml.Node]bool, n *yaml.Node) (any, error) {
	// nil decodes to nil.
	if n == nil {
		return nil, nil
	}

	// If n has been seen already while processing the parents of n, it's an
	// infinite recursion.
	// Simple example:
	// ---
	// a: &a  // seen is empty on encoding a
	//   b: *a   // seen contains a while encoding b
	if seen[n] {
		return nil, fmt.Errorf("line %d, col %d: infinite recursion", n.Line, n.Column)
	}
	seen[n] = true

	// n needs to be "un-seen" when this layer of recursion is done:
	defer delete(seen, n)
	// Why? seen is a map, which is used by reference, so it will be shared
	// between calls to decode, which is recursive. And unlike a merge, the
	// same alias can be validly used for different subtrees:
	// ---
	// a: &a
	//   b: c
	// d:
	//   da: *a
	//   db: *a
	// ...
	// (d contains two copies of a).
	// So *a needs to be "unseen" between encoding "da" and "db".

	switch n.Kind {
	case yaml.ScalarNode:
		// If we need to parse more kinds of scalar, e.g. !!bool NO, or base-60
		// integers, this is where we would swap out n.Decode.
		var v any
		if err := n.Decode(&v); err != nil {
			return nil, err
		}
		return v, nil

	case yaml.SequenceNode:
		v := make([]any, 0, len(n.Content))
		for _, c := range n.Content {
			cv, err := decodeYAML(seen, c)
			if err != nil {
				return nil, err
			}
			v = append(v, cv)
		}
		return v, nil

	case yaml.MappingNode:
		m := NewMap[string, any](len(n.Content) / 2)
		// Why not call m.UnmarshalYAML(n) ?
		// Because we can't pass `seen` through that.
		err := rangeYAMLMap(n, func(key string, val *yaml.Node) error {
			v, err := decodeYAML(seen, val)
			if err != nil {
				return err
			}
			m.Set(key, v)
			return nil
		})
		if err != nil {
			return nil, err
		}
		return m, nil

	case yaml.AliasNode:
		// This is one of the two ways this can blow up recursively.
		// The other (map merges) is handled by rangeMap.
		return decodeYAML(seen, n.Alias)

	case yaml.DocumentNode:
		switch len(n.Content) {
		case 0:
			return nil, nil
		case 1:
			return decodeYAML(seen, n.Content[0])
		default:
			return nil, fmt.Errorf("line %d, col %d: document contains more than 1 content item (%d)", n.Line, n.Column, len(n.Content))
		}

	default:
		return nil, fmt.Errorf("line %d, col %d: unsupported kind %x", n.Line, n.Column, n.Kind)
	}
}

// rangeYAMLMap calls f with each key/value pair in a mapping node.
// It only supports scalar keys, and converts them to canonical string values.
// Non-scalar and non-stringable keys result in an error.
// Because mapping nodes can contain merges from other mapping nodes,
// potentially via sequence nodes and aliases, this function also accepts
// sequences and aliases (that must themselves recursively only contain
// mappings, sequences, and aliases...).
func rangeYAMLMap(n *yaml.Node, f func(key string, val *yaml.Node) error) error {
	return rangeYAMLMapImpl(make(map[*yaml.Node]bool), n, f)
}

// rangeYAMLMapImpl implements rangeYAMLMap. It tracks mapping nodes already
// merged, to prevent infinite merge loops and avoid unnecessarily merging the
// same mapping repeatedly.
func rangeYAMLMapImpl(merged map[*yaml.Node]bool, n *yaml.Node, f func(key string, val *yaml.Node) error) error {
	// Go-like semantics: no entries in "nil".
	if n == nil {
		return nil
	}

	// If this node has already been merged into the top-level map being ranged,
	// we don't need to merge it again.
	if merged[n] {
		return nil
	}
	merged[n] = true

	switch n.Kind {
	case yaml.MappingNode:
		// gopkg.in/yaml.v3 parses mapping node contents as a flat list:
		// key, value, key, value...
		if len(n.Content)%2 != 0 {
			return fmt.Errorf("line %d, col %d: mapping node has odd content length %d", n.Line, n.Column, len(n.Content))
		}

		// Keys at an outer level take precedence over keys being merged:
		// "its key/value pairs is inserted into the current mapping, unless the
		// key already exists in it." https://yaml.org/type/merge.html
		// But we care about key ordering!
		// This necessitates two passes:
		// 1. Obtain the keys in this map
		// 2. Range over the map again, recursing into merges.
		// While merging, ignore keys in the outer level.
		// Merges may produce new keys to ignore in subsequent merges:
		// "Keys in mapping nodes earlier in the sequence override keys
		// specified in later mapping nodes."

		// 1. A pass to get the keys at this level.
		keys := make(map[string]bool)
		for i := 0; i < len(n.Content); i += 2 {
			k := n.Content[i]

			// Ignore merges in this pass.
			if k.Tag == "!!merge" {
				continue
			}

			// Canonicalise the key into a string and store it.
			ck, err := canonicalMapKey(k)
			if err != nil {
				return err
			}
			keys[ck] = true
		}

		// Ignore existing keys when merging. Record new keys to ignore in
		// subsequent merges.
		skipKeys := func(k string, v *yaml.Node) error {
			if keys[k] {
				return nil
			}
			keys[k] = true
			return f(k, v)
		}

		// 2. Range over each pair, recursing into merges.
		for i := 0; i < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]

			// Is this pair a merge? (`<<: *foo`)
			if k.Tag == "!!merge" {
				// Recursively range over the contents of the value, which
				// could be an alias to a mapping node, or a sequence of aliases
				// to mapping nodes, which could themselves contain merges...
				if err := rangeYAMLMapImpl(merged, v, skipKeys); err != nil {
					return err
				}
				continue
			}

			// Canonicalise the key into a string (again).
			ck, err := canonicalMapKey(k)
			if err != nil {
				return err
			}

			// Yield the canonical key and the value.
			if err := f(ck, v); err != nil {
				return err
			}
		}

	case yaml.SequenceNode:
		// Range over each element e in the sequence.
		for _, e := range n.Content {
			if err := rangeYAMLMapImpl(merged, e, f); err != nil {
				return err
			}
		}

	case yaml.AliasNode:
		// Follow the alias and range over that.
		if err := rangeYAMLMapImpl(merged, n.Alias, f); err != nil {
			return err
		}

	default:
		// TODO: Use %v once yaml.Kind has a String method
		return fmt.Errorf("line %d, col %d: cannot range over node kind %x", n.Line, n.Column, n.Kind)
	}
	return nil
}

// canonicalMapKey converts a scalar value into a string suitable for use as
// a map key. YAML expects different representations of the same value, e.g.
// 0xb and 11, to be equivalent, and therefore a duplicate key. JSON requires
// all keys to be strings.
func canonicalMapKey(n *yaml.Node) (string, error) {
	switch n.Kind {
	case yaml.AliasNode:
		return canonicalMapKey(n.Alias)

	case yaml.ScalarNode:
		var x any
		if err := n.Decode(&x); err != nil {
			return "", err
		}
		if x == nil || n.Tag == "!!null" {
			// Nulls are not valid JSON keys.
			return "", fmt.Errorf("line %d, col %d: null not supported as a map key", n.Line, n.Column)
		}
		switch n.Tag {
		case "!!bool":
			// Canonicalise to true or false.
			return fmt.Sprintf("%t", x), nil
		case "!!int":
			// Canonicalise to decimal.
			return fmt.Sprintf("%d", x), nil
		case "!!float":
			// Canonicalise to scientific notation.
			// Don't handle Inf or NaN specially, as they will be quoted.
			return fmt.Sprintf("%e", x), nil
		default:
			// Assume the value is already a suitable key.
			return n.Value, nil
		}

	default:
		// TODO: Use %v once yaml.Kind has a String method
		return "", fmt.Errorf("line %d, col %d: cannot use node kind %x as a map key", n.Line, n.Column, n.Kind)
	}
}
