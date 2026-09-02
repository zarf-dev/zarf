package pipeline

import (
	"errors"
	"io"
	"iter"
	"strings"

	"github.com/buildkite/go-pipeline/ordered"
	"gopkg.in/yaml.v3"
)

// Options are functional options for creating a new Env.
type Options func(*Pipeline)

// Parse parses a pipeline. It does not apply interpolation.
// Note that it only parses the first YAML document in the input.
// To parse all YAML documents in the input reader as pipelines, use ParseAll.
//
// Warnings are passed through the err return:
//
//	p, err := Parse(src)
//	if w := warning.As(err); w != nil {
//		// Here are some warnings that should be shown
//		log.Printf("*Warning* - pipeline is not fully parsed:\n%v", w)
//	} else if err != nil {
//	    // Parse could not understand src at all
//	    return err
//	}
//	// Use p
func Parse(src io.Reader) (*Pipeline, error) {
	// First get yaml.v3 to give us a raw document (*yaml.Node).
	n := new(yaml.Node)
	if err := yaml.NewDecoder(src).Decode(n); err != nil {
		return nil, formatYAMLError(err)
	}

	// Instead of unmarshalling into structs, which is easy-ish to use but
	// doesn't work with some non YAML 1.2 features (merges), decode the
	// *yaml.Node into *ordered.Map, []any, or any (recursively).
	// This resolves aliases and merges and gives a more convenient form to work
	// with when handling different structural representations of the same
	// configuration. Then decode _that_ into a pipeline.
	p := new(Pipeline)

	return p, ordered.Unmarshal(n, p)
}

// ParseAll parses all YAML documents in the input stream as pipelines.
// It does not stop yielding on decoding or unmarshaling errors.
// It does not apply interpolation.
//
// Warnings are passed through the err return:
//
//	for p, err := range ParseAll(src) {
//		if w := warning.As(err); w != nil {
//			// Here are some warnings that should be shown
//			log.Printf("*Warning* - pipeline is not fully parsed:\n%v", w)
//		} else if err != nil {
//			// Parse could not understand src at all
//			return err
//		}
//		// Use p
//	}
func ParseAll(src io.Reader) iter.Seq2[*Pipeline, error] {
	dec := yaml.NewDecoder(src)
	return func(yield func(*Pipeline, error) bool) {
		for {
			// The loop body here is essentially similar to Parse (above).
			n := new(yaml.Node)
			if err := dec.Decode(n); err != nil {
				if err == io.EOF {
					return // end of stream
				}
				if !yield(nil, formatYAMLError(err)) {
					return
				}
				continue
			}

			p := new(Pipeline)
			if !yield(p, ordered.Unmarshal(n, p)) {
				return
			}
		}
	}
}

func formatYAMLError(err error) error {
	return errors.New(strings.TrimPrefix(err.Error(), "yaml: "))
}
