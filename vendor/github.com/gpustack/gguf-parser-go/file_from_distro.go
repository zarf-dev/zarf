package gguf_parser

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gpustack/gguf-parser-go/util/httpx"
)

var (
	ErrOllamaInvalidModel      = errors.New("ollama invalid model")
	ErrOllamaBaseLayerNotFound = errors.New("ollama base layer not found")
)

// ParseGGUFFileFromOllama parses a GGUF file from Ollama model's base layer,
// and returns a GGUFFile, or an error if any.
func ParseGGUFFileFromOllama(ctx context.Context, model string, opts ...GGUFReadOption) (*GGUFFile, error) {
	return ParseGGUFFileFromOllamaModel(ctx, ParseOllamaModel(model), opts...)
}

// ParseGGUFFileFromOllamaModel is similar to ParseGGUFFileFromOllama,
// but inputs an OllamaModel instead of a string.
//
// The given OllamaModel will be completed(fetching MediaType, Config and Layers) after calling this function.
func ParseGGUFFileFromOllamaModel(ctx context.Context, model *OllamaModel, opts ...GGUFReadOption) (gf *GGUFFile, err error) {
	if model == nil {
		return nil, ErrOllamaInvalidModel
	}

	opts = append(opts[:len(opts):len(opts)], SkipRangeDownloadDetection())

	var o _GGUFReadOptions
	for _, opt := range opts {
		opt(&o)
	}

	// Cache.
	{
		if o.CachePath != "" {
			o.CachePath = filepath.Join(o.CachePath, "distro", "ollama")
		}
		c := GGUFFileCache(o.CachePath)

		// Get from cache.
		if gf, err = c.Get(model.String(), o.CacheExpiration); err == nil {
			return gf, nil
		}

		// Put to cache.
		defer func() {
			if err == nil {
				_ = c.Put(model.String(), gf)
			}
		}()
	}

	var cli *http.Client
	cli = httpx.Client(
		httpx.ClientOptions().
			WithUserAgent(OllamaUserAgent()).
			If(o.Debug, func(x *httpx.ClientOption) *httpx.ClientOption {
				return x.WithDebug()
			}).
			WithTimeout(0).
			WithRetryBackoff(1*time.Second, 5*time.Second, 10).
			WithRetryIf(func(resp *http.Response, err error) bool {
				return httpx.DefaultRetry(resp, err) || OllamaRegistryAuthorizeRetry(resp, cli)
			}).
			WithTransport(
				httpx.TransportOptions().
					WithoutKeepalive().
					TimeoutForDial(10*time.Second).
					TimeoutForTLSHandshake(5*time.Second).
					If(o.SkipProxy, func(x *httpx.TransportOption) *httpx.TransportOption {
						return x.WithoutProxy()
					}).
					If(o.ProxyURL != nil, func(x *httpx.TransportOption) *httpx.TransportOption {
						return x.WithProxy(http.ProxyURL(o.ProxyURL))
					}).
					If(o.SkipTLSVerification, func(x *httpx.TransportOption) *httpx.TransportOption {
						return x.WithoutInsecureVerify()
					}).
					If(o.SkipDNSCache, func(x *httpx.TransportOption) *httpx.TransportOption {
						return x.WithoutDNSCache()
					})))

	var ml OllamaModelLayer
	{
		err := model.Complete(ctx, cli)
		if err != nil {
			return nil, fmt.Errorf("complete ollama model: %w", err)
		}

		var ok bool
		ml, ok = model.GetLayer("application/vnd.ollama.image.model")
		if !ok {
			return nil, ErrOllamaBaseLayerNotFound
		}
	}

	return parseGGUFFileFromRemote(ctx, cli, ml.BlobURL().String(), o)
}
