package httpx

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"time"
)

type TransportOption struct {
	dialer    *net.Dialer
	transport *http.Transport
}

func TransportOptions() *TransportOption {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	transport := &http.Transport{
		Proxy: ProxyFromEnvironment,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
		DialContext:           DNSCacheDialContext(dialer),
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &TransportOption{
		dialer:    dialer,
		transport: transport,
	}
}

// WithProxy sets the proxy.
func (o *TransportOption) WithProxy(proxy func(*http.Request) (*url.URL, error)) *TransportOption {
	if o == nil || o.transport == nil {
		return o
	}
	o.transport.Proxy = proxy
	return o
}

// WithoutProxy disables the proxy.
func (o *TransportOption) WithoutProxy() *TransportOption {
	if o == nil || o.transport == nil {
		return o
	}
	o.transport.Proxy = nil
	return o
}

// WithKeepalive sets the keepalive.
func (o *TransportOption) WithKeepalive(timeoutAndKeepalive ...time.Duration) *TransportOption {
	if o == nil || o.transport == nil || o.dialer == nil {
		return o
	}
	tak := [2]time.Duration{30 * time.Second, 30 * time.Second}
	if len(timeoutAndKeepalive) > 0 {
		tak[0] = timeoutAndKeepalive[0]
		if len(timeoutAndKeepalive) > 1 {
			tak[1] = timeoutAndKeepalive[1]
		}
	}
	o.dialer.Timeout, o.dialer.KeepAlive = tak[0], tak[1]
	o.transport.MaxIdleConns = 100
	o.transport.IdleConnTimeout = 90 * time.Second
	return o
}

// WithoutKeepalive disables the keepalive.
func (o *TransportOption) WithoutKeepalive() *TransportOption {
	if o == nil || o.transport == nil {
		return o
	}
	o.dialer.KeepAlive = -1
	o.transport.MaxIdleConns = 0
	o.transport.IdleConnTimeout = 0
	return o
}

// WithInsecureVerify verifies the insecure connection.
func (o *TransportOption) WithInsecureVerify() *TransportOption {
	if o == nil || o.transport == nil || o.transport.TLSClientConfig == nil {
		return o
	}
	o.transport.TLSClientConfig.InsecureSkipVerify = false
	return o
}

// WithoutInsecureVerify skips the insecure connection verify.
func (o *TransportOption) WithoutInsecureVerify() *TransportOption {
	if o == nil || o.transport == nil || o.transport.TLSClientConfig == nil {
		return o
	}
	o.transport.TLSClientConfig.InsecureSkipVerify = true
	return o
}

// TimeoutForDial sets the timeout for network dial.
//
// This timeout controls the [network dial] only.
//
// Use 0 to disable timeout.
func (o *TransportOption) TimeoutForDial(timeout time.Duration) *TransportOption {
	if o == nil || o.dialer == nil {
		return o
	}
	o.dialer.Timeout = timeout
	return o
}

// TimeoutForResponseHeader sets the timeout for response header.
//
// This timeout controls the [response header reading] only.
//
// Use 0 to disable timeout.
func (o *TransportOption) TimeoutForResponseHeader(timeout time.Duration) *TransportOption {
	if o == nil || o.transport == nil {
		return o
	}
	o.transport.ResponseHeaderTimeout = timeout
	return o
}

// TimeoutForTLSHandshake sets the timeout for tls handshake.
//
// This timeout controls the [tls handshake] only.
//
// Use 0 to disable timeout.
func (o *TransportOption) TimeoutForTLSHandshake(timeout time.Duration) *TransportOption {
	if o == nil || o.transport == nil {
		return o
	}
	o.transport.TLSHandshakeTimeout = timeout
	return o
}

// TimeoutForIdleConn sets the timeout for idle connection.
//
// This timeout controls the [idle connection lifetime] only.
//
// Use 0 to disable timeout.
func (o *TransportOption) TimeoutForIdleConn(timeout time.Duration) *TransportOption {
	if o == nil || o.transport == nil {
		return o
	}
	o.transport.IdleConnTimeout = timeout
	return o
}

// WithTLSClientConfig sets the tls.Config.
func (o *TransportOption) WithTLSClientConfig(config *tls.Config) *TransportOption {
	if o == nil || o.transport == nil {
		return o
	}
	o.transport.TLSClientConfig = config
	return o
}

// WithoutDNSCache disables the dns cache.
func (o *TransportOption) WithoutDNSCache() *TransportOption {
	if o == nil || o.transport == nil || o.dialer == nil {
		return o
	}
	o.transport.DialContext = o.dialer.DialContext
	return o
}

// WithDialer sets the dialer.
func (o *TransportOption) WithDialer(dialer *net.Dialer) *TransportOption {
	if o == nil || o.transport == nil || dialer == nil {
		return o
	}
	o.dialer = dialer
	o.transport.DialContext = DNSCacheDialContext(o.dialer)
	return o
}

// Customize sets the transport.
func (o *TransportOption) Customize(fn func(*http.Transport)) *TransportOption {
	if o == nil || o.transport == nil {
		return o
	}
	o.dialer = nil
	fn(o.transport)
	return o
}

// If is a conditional option,
// which receives a boolean condition to trigger the given function or not.
func (o *TransportOption) If(condition bool, then func(*TransportOption) *TransportOption) *TransportOption {
	if condition {
		return then(o)
	}
	return o
}
