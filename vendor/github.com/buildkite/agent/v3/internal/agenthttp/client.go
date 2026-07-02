// Package agenthttp creates standard Go [net/http.Client]s with common config
// options.
package agenthttp

import (
	"crypto/tls"
	"net/http"
	"sync"
	"time"

	"golang.org/x/net/http2"
)

// NewClient creates a HTTP client. Note that the default timeout is 60 seconds;
// for some use cases (e.g. artifact operations) use [WithNoTimeout].
func NewClient(opts ...ClientOption) *http.Client {
	conf := clientConfig{
		// This spells out the defaults, even if some of them are zero values.
		Bearer:     "",
		Token:      "",
		AllowHTTP2: true,
		Timeout:    60 * time.Second,
		TLSConfig:  nil,
	}
	for _, opt := range opts {
		opt(&conf)
	}

	cacheKey := transportCacheKey{
		AllowHTTP2: conf.AllowHTTP2,
		TLSConfig:  conf.TLSConfig,
	}

	transportCacheMu.Lock()
	transport := transportCache[cacheKey]
	if transport == nil {
		transport = newTransport(&conf)
		transportCache[cacheKey] = transport
	}
	transportCacheMu.Unlock()

	if conf.Bearer == "" && conf.Token == "" {
		// No credentials, no authenticatedTransport wrapper.
		return &http.Client{
			Timeout:   conf.Timeout,
			Transport: transport,
		}
	}

	// Wrap the transport in authenticatedTransport.
	return &http.Client{
		Timeout: conf.Timeout,
		Transport: &authenticatedTransport{
			Bearer:   conf.Bearer,
			Token:    conf.Token,
			Delegate: transport,
		},
	}
}

// Various NewClient options.
func WithAuthBearer(b string) ClientOption     { return func(c *clientConfig) { c.Bearer = b } }
func WithAuthToken(t string) ClientOption      { return func(c *clientConfig) { c.Token = t } }
func WithAllowHTTP2(a bool) ClientOption       { return func(c *clientConfig) { c.AllowHTTP2 = a } }
func WithTimeout(d time.Duration) ClientOption { return func(c *clientConfig) { c.Timeout = d } }
func WithNoTimeout(c *clientConfig)            { c.Timeout = 0 }
func WithTLSConfig(t *tls.Config) ClientOption { return func(c *clientConfig) { c.TLSConfig = t } }

type ClientOption = func(*clientConfig)

func newTransport(conf *clientConfig) *http.Transport {
	// Base any modifications on the default transport.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// Allow override of TLSConfig. This must be set prior to calling
	// http2.ConfigureTransports.
	if conf.TLSConfig != nil {
		transport.TLSClientConfig = conf.TLSConfig
	}

	if conf.AllowHTTP2 {
		// There is a bug in http2 on Linux regarding using dead connections.
		// This is a workaround. See https://github.com/golang/go/issues/59690
		//
		// Note that http2.ConfigureTransports alters its argument in order to
		// supply http2 functionality, and the http2.Transport does not support
		// HTTP/1.1 as a protocol, so we get slightly odd-looking code where
		// we use `transport` later on instead of the just-returned `tr2`.
		// tr2 is needed merely to configure the http2 option.
		tr2, err := http2.ConfigureTransports(transport)
		if err != nil {
			// ConfigureTransports is documented to only return an error if
			// the transport arg was already HTTP2-enabled, which it should not
			// have been...
			panic("http2.ConfigureTransports: " + err.Error())
		}
		if tr2 != nil {
			tr2.ReadIdleTimeout = 30 * time.Second
		}
	} else {
		transport.TLSNextProto = make(map[string]func(string, *tls.Conn) http.RoundTripper)
		// The default TLSClientConfig has h2 in NextProtos, so the
		// negotiated TLS connection will assume h2 support.
		// see https://github.com/golang/go/issues/50571
		transport.TLSClientConfig.NextProtos = []string{"http/1.1"}
	}

	return transport
}

type clientConfig struct {
	// The authentication token/ bearer credential to use
	// For agent API usage, Token is usually an agent registration or access token
	// For GraphQL usage, Bearer is usually a user token
	Token  string
	Bearer string

	// If false, HTTP2 is disabled
	AllowHTTP2 bool

	// Timeout used as the client timeout.
	Timeout time.Duration

	// optional TLS configuration primarily used for testing
	TLSConfig *tls.Config
}

// The underlying http.Transport is cached, mainly so that multiple clients with
// the same options can reuse connections. The options that affect the transport
// are also usually the same throughout the process.
type transportCacheKey struct {
	AllowHTTP2 bool
	TLSConfig  *tls.Config
}

var (
	transportCacheMu sync.Mutex
	transportCache   = make(map[transportCacheKey]*http.Transport)
)
