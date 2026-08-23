package gguf_parser

import (
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gpustack/gguf-parser-go/util/osx"
)

type (
	_GGUFReadOptions struct {
		Debug             bool
		SkipLargeMetadata bool

		// Local.
		MMap bool

		// Remote.
		BearerAuthToken            string
		Headers                    map[string]string
		ProxyURL                   *url.URL
		SkipProxy                  bool
		SkipTLSVerification        bool
		SkipDNSCache               bool
		BufferSize                 int
		SkipRangeDownloadDetection bool
		CachePath                  string
		CacheExpiration            time.Duration
	}

	// GGUFReadOption is the option for reading the file.
	GGUFReadOption func(o *_GGUFReadOptions)
)

// UseDebug uses debug mode to read the file.
func UseDebug() GGUFReadOption {
	return func(o *_GGUFReadOptions) {
		o.Debug = true
	}
}

// SkipLargeMetadata skips reading large GGUFMetadataKV items,
// which are not necessary for most cases.
func SkipLargeMetadata() GGUFReadOption {
	return func(o *_GGUFReadOptions) {
		o.SkipLargeMetadata = true
	}
}

// UseMMap uses mmap to read the local file.
func UseMMap() GGUFReadOption {
	return func(o *_GGUFReadOptions) {
		o.MMap = true
	}
}

// UseBearerAuth uses the given token as a bearer auth when reading from remote.
func UseBearerAuth(token string) GGUFReadOption {
	return func(o *_GGUFReadOptions) {
		o.BearerAuthToken = token
	}
}

// UseHeaders uses the given headers when reading from remote.
func UseHeaders(headers map[string]string) GGUFReadOption {
	return func(o *_GGUFReadOptions) {
		o.Headers = headers
	}
}

// UseProxy uses the given url as a proxy when reading from remote.
func UseProxy(url *url.URL) GGUFReadOption {
	return func(o *_GGUFReadOptions) {
		o.ProxyURL = url
	}
}

// SkipProxy skips the proxy when reading from remote.
func SkipProxy() GGUFReadOption {
	return func(o *_GGUFReadOptions) {
		o.SkipProxy = true
	}
}

// SkipTLSVerification skips the TLS verification when reading from remote.
func SkipTLSVerification() GGUFReadOption {
	return func(o *_GGUFReadOptions) {
		o.SkipTLSVerification = true
	}
}

// SkipDNSCache skips the DNS cache when reading from remote.
func SkipDNSCache() GGUFReadOption {
	return func(o *_GGUFReadOptions) {
		o.SkipDNSCache = true
	}
}

// UseBufferSize sets the buffer size when reading from remote.
func UseBufferSize(size int) GGUFReadOption {
	const minSize = 32 * 1024
	if size < minSize {
		size = minSize
	}
	return func(o *_GGUFReadOptions) {
		o.BufferSize = size
	}
}

// SkipRangeDownloadDetection skips the range download detection when reading from remote.
func SkipRangeDownloadDetection() GGUFReadOption {
	return func(o *_GGUFReadOptions) {
		o.SkipRangeDownloadDetection = true
	}
}

// UseCache caches the remote reading result.
func UseCache() GGUFReadOption {
	return func(o *_GGUFReadOptions) {
		o.CachePath = DefaultCachePath()
		o.CacheExpiration = 24 * time.Hour
	}
}

// SkipCache skips the cache when reading from remote.
func SkipCache() GGUFReadOption {
	return func(o *_GGUFReadOptions) {
		o.CachePath = ""
		o.CacheExpiration = 0
	}
}

// DefaultCachePath returns the default cache path.
func DefaultCachePath() string {
	cd := filepath.Join(osx.UserHomeDir(), ".cache")
	if runtime.GOOS == "windows" {
		cd = osx.Getenv("APPDATA", cd)
	}
	return filepath.Join(cd, "gguf-parser")
}

// UseCachePath uses the given path to cache the remote reading result.
func UseCachePath(path string) GGUFReadOption {
	path = strings.TrimSpace(filepath.Clean(osx.InlineTilde(path)))
	return func(o *_GGUFReadOptions) {
		if path == "" {
			return
		}
		o.CachePath = path
	}
}

// UseCacheExpiration uses the given expiration to cache the remote reading result.
//
// Disable cache expiration by setting it to 0.
func UseCacheExpiration(expiration time.Duration) GGUFReadOption {
	if expiration < 0 {
		expiration = 0
	}
	return func(o *_GGUFReadOptions) {
		o.CacheExpiration = expiration
	}
}
