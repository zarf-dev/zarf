package pipeline

import (
	"encoding/json"
	"net/url"
	"path"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	_ interface {
		json.Marshaler
		yaml.Marshaler
		selfInterpolater
	} = (*Plugin)(nil)
)

// Plugin models plugin configuration.
//
// Standard caveats apply - see the package comment.
type Plugin struct {
	Source string
	Config any
}

// MarshalJSON returns the plugin in "one-key object" form. Plugin sources are
// marshalled into "full" form. Plugins originally specified as a single string
// (no config, only source) are canonicalised into "one-key object" with config
// null.
func (p *Plugin) MarshalJSON() ([]byte, error) {
	// NB: MarshalYAML (as seen below) never returns an error.
	o, _ := p.MarshalYAML()
	return json.Marshal(o)
}

// MarshalYAML returns the plugin in "one-item map" form. Plugin sources
// are marshalled into "full" form.  Plugins originally specified as a single
// string (no config, only source) are canonicalised into "one-item map" with
// config nil. Configs that are zero-length maps are canonicalised to nil.
func (p *Plugin) MarshalYAML() (any, error) {
	cfg := p.Config
	switch x := cfg.(type) {
	case map[string]any:
		if len(x) == 0 {
			cfg = nil
		}

	case []any:
		// Should be invalid, but a different part of the process should be
		// responsible for checking and complaining.
		if len(x) == 0 {
			cfg = nil
		}
	}
	return map[string]any{
		p.FullSource(): cfg,
	}, nil
}

// FullSource attempts to canonicalise Source. If it fails, it returns Source
// unaltered. Otherwise, it resolves sources in a manner described at
// https://buildkite.com/docs/plugins/using#plugin-sources.
func (p *Plugin) FullSource() string {
	if p.Source == "" {
		return ""
	}

	// Looks like an absolute or relative file path.
	if strings.HasPrefix(p.Source, "/") || strings.HasPrefix(p.Source, ".") || strings.HasPrefix(p.Source, `\`) {
		return p.Source
	}

	u, err := url.Parse(p.Source)
	if err != nil {
		return p.Source
	}

	// They wrote something like ssh://..., https://..., or C:\...
	// in which case they _mean it_.
	if u.Scheme != "" || u.Opaque != "" {
		return p.Source
	}

	// thing      => thing-buildkite-plugin
	// thing#main => thing-buildkite-plugin#main
	lastSegment := func(n, f string) string {
		n += "-buildkite-plugin"
		if f == "" {
			return n
		}
		return n + "#" + f
	}

	paths := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	switch len(paths) {
	case 1:
		// trimmed path contained no slash
		return path.Join("github.com", "buildkite-plugins", lastSegment(paths[0], u.Fragment))

	case 2:
		// trimmed path contained one slash
		return path.Join("github.com", paths[0], lastSegment(paths[1], u.Fragment))

	default:
		// trimmed path contained more than one slash - apply no smarts
		return p.Source
	}
}

func (p *Plugin) interpolate(tf stringTransformer) error {
	name, err := tf.Transform(p.Source)
	if err != nil {
		return err
	}
	cfg, err := interpolateAny(tf, p.Config)
	if err != nil {
		return err
	}
	p.Source = name
	p.Config = cfg
	return nil
}
