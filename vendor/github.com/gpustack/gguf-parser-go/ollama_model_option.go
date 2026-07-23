package gguf_parser

import (
	"net/url"
	"strings"
)

type (
	_OllamaModelOptions struct {
		DefaultScheme    string
		DefaultRegistry  string
		DefaultNamespace string
		DefaultTag       string
	}
	OllamaModelOption func(*_OllamaModelOptions)
)

// SetOllamaModelBaseURL parses the given base URL,
// and sets default schema/registry for OllamaModel.
func SetOllamaModelBaseURL(baseURL string) OllamaModelOption {
	baseURL = strings.TrimSpace(baseURL)
	return func(o *_OllamaModelOptions) {
		if baseURL == "" {
			return
		}

		if !strings.Contains(baseURL, "://") {
			baseURL = "https://" + baseURL
		}

		u, err := url.Parse(baseURL)
		if err != nil {
			return
		}

		o.DefaultScheme = u.Scheme
		o.DefaultRegistry = u.Host
	}
}

// SetOllamaModelDefaultScheme sets the default scheme for OllamaModel.
func SetOllamaModelDefaultScheme(scheme string) OllamaModelOption {
	return func(o *_OllamaModelOptions) {
		if scheme == "" {
			return
		}
		o.DefaultScheme = scheme
	}
}

// SetOllamaModelDefaultRegistry sets the default registry for OllamaModel.
func SetOllamaModelDefaultRegistry(registry string) OllamaModelOption {
	return func(o *_OllamaModelOptions) {
		if registry == "" {
			return
		}
		o.DefaultRegistry = registry
	}
}

// SetOllamaModelDefaultNamespace sets the default namespace for OllamaModel.
func SetOllamaModelDefaultNamespace(namespace string) OllamaModelOption {
	return func(o *_OllamaModelOptions) {
		if namespace == "" {
			return
		}
		o.DefaultNamespace = namespace
	}
}

// SetOllamaModelDefaultTag sets the default tag for OllamaModel.
func SetOllamaModelDefaultTag(tag string) OllamaModelOption {
	return func(o *_OllamaModelOptions) {
		if tag == "" {
			return
		}
		o.DefaultTag = tag
	}
}
