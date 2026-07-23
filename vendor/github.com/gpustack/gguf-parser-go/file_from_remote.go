package gguf_parser

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gpustack/gguf-parser-go/util/httpx"
	"github.com/gpustack/gguf-parser-go/util/osx"
)

// ParseGGUFFileFromHuggingFace parses a GGUF file from Hugging Face(https://huggingface.co/),
// and returns a GGUFFile, or an error if any.
func ParseGGUFFileFromHuggingFace(ctx context.Context, repo, file string, opts ...GGUFReadOption) (*GGUFFile, error) {
	ep := osx.Getenv("HF_ENDPOINT", "https://huggingface.co")
	return ParseGGUFFileRemote(ctx, fmt.Sprintf("%s/%s/resolve/main/%s", ep, repo, file), opts...)
}

// ParseGGUFFileFromModelScope parses a GGUF file from Model Scope(https://modelscope.cn/),
// and returns a GGUFFile, or an error if any.
func ParseGGUFFileFromModelScope(ctx context.Context, repo, file string, opts ...GGUFReadOption) (*GGUFFile, error) {
	ep := osx.Getenv("MS_ENDPOINT", "https://modelscope.cn")
	opts = append(opts[:len(opts):len(opts)], SkipRangeDownloadDetection())
	return ParseGGUFFileRemote(ctx, fmt.Sprintf("%s/models/%s/resolve/master/%s", ep, repo, file), opts...)
}

// ParseGGUFFileRemote parses a GGUF file from a remote BlobURL,
// and returns a GGUFFile, or an error if any.
func ParseGGUFFileRemote(ctx context.Context, url string, opts ...GGUFReadOption) (gf *GGUFFile, err error) {
	var o _GGUFReadOptions
	for _, opt := range opts {
		opt(&o)
	}

	// Cache.
	{
		if o.CachePath != "" {
			o.CachePath = filepath.Join(o.CachePath, "remote")
			if o.SkipLargeMetadata {
				o.CachePath = filepath.Join(o.CachePath, "brief")
			}
		}
		c := GGUFFileCache(o.CachePath)

		// Get from cache.
		if gf, err = c.Get(url, o.CacheExpiration); err == nil {
			return gf, nil
		}

		// Put to cache.
		defer func() {
			if err == nil {
				_ = c.Put(url, gf)
			}
		}()
	}

	cli := httpx.Client(
		httpx.ClientOptions().
			WithUserAgent("gguf-parser-go").
			If(o.Debug,
				func(x *httpx.ClientOption) *httpx.ClientOption {
					return x.WithDebug()
				},
			).
			If(o.BearerAuthToken != "",
				func(x *httpx.ClientOption) *httpx.ClientOption {
					return x.WithBearerAuth(o.BearerAuthToken)
				},
			).
			If(len(o.Headers) > 0,
				func(x *httpx.ClientOption) *httpx.ClientOption {
					return x.WithHeaders(o.Headers)
				},
			).
			WithTimeout(0).
			WithTransport(
				httpx.TransportOptions().
					WithoutKeepalive().
					TimeoutForDial(5*time.Second).
					TimeoutForTLSHandshake(5*time.Second).
					TimeoutForResponseHeader(5*time.Second).
					If(o.SkipProxy,
						func(x *httpx.TransportOption) *httpx.TransportOption {
							return x.WithoutProxy()
						},
					).
					If(o.ProxyURL != nil,
						func(x *httpx.TransportOption) *httpx.TransportOption {
							return x.WithProxy(http.ProxyURL(o.ProxyURL))
						},
					).
					If(o.SkipTLSVerification || !strings.HasPrefix(url, "https://"),
						func(x *httpx.TransportOption) *httpx.TransportOption {
							return x.WithoutInsecureVerify()
						},
					).
					If(o.SkipDNSCache,
						func(x *httpx.TransportOption) *httpx.TransportOption {
							return x.WithoutDNSCache()
						},
					),
			),
	)

	return parseGGUFFileFromRemote(ctx, cli, url, o)
}

func parseGGUFFileFromRemote(ctx context.Context, cli *http.Client, url string, o _GGUFReadOptions) (*GGUFFile, error) {
	var urls []string
	{
		rs := CompleteShardGGUFFilename(url)
		if rs != nil {
			urls = rs
		} else {
			urls = []string{url}
		}
	}

	fs := make([]_GGUFFileReadSeeker, 0, len(urls))
	defer func() {
		for i := range fs {
			osx.Close(fs[i])
		}
	}()

	for i := range urls {
		req, err := httpx.NewGetRequestWithContext(ctx, urls[i])
		if err != nil {
			return nil, fmt.Errorf("new request: %w", err)
		}

		sf, err := httpx.OpenSeekerFile(cli, req,
			httpx.SeekerFileOptions().
				WithBufferSize(o.BufferSize).
				If(o.SkipRangeDownloadDetection,
					func(x *httpx.SeekerFileOption) *httpx.SeekerFileOption {
						return x.WithoutRangeDownloadDetect()
					},
				),
		)
		if err != nil {
			return nil, fmt.Errorf("open http file: %w", err)
		}

		fs = append(fs, _GGUFFileReadSeeker{
			Closer:     sf,
			ReadSeeker: io.NewSectionReader(sf, 0, sf.Len()),
			Size:       sf.Len(),
		})
	}

	return parseGGUFFile(fs, o)
}
