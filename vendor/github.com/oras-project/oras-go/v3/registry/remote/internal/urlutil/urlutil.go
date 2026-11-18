/*
Copyright The ORAS Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package urlutil

import (
	"net"
	"net/netip"
	"net/url"
	"strings"
)

// CanonicalHostPort returns the canonical host (canonical IP literal or
// lower-cased host name) plus explicit-or-default port (80 for http, 443 for
// https). It uses net.SplitHostPort and net.JoinHostPort to properly handle
// IPv6 brackets and standard ports.
func CanonicalHostPort(u *url.URL) string {
	if u == nil {
		return ""
	}
	host, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		host = u.Hostname()
		port = u.Port()
	}
	if port == "" {
		switch strings.ToLower(u.Scheme) {
		case "https":
			port = "443"
		case "http":
			port = "80"
		}
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		host = addr.String()
	} else {
		host = strings.ToLower(host)
	}
	return net.JoinHostPort(host, port)
}

// SameOrigin reports whether URLs a and b share the same origin, that is,
// the same scheme (case-insensitive) and the same canonical host:port.
func SameOrigin(a, b *url.URL) bool {
	if a == nil || b == nil {
		return a == b
	}
	if !strings.EqualFold(a.Scheme, b.Scheme) {
		return false
	}
	return CanonicalHostPort(a) == CanonicalHostPort(b)
}

// SameHostname reports whether URLs a and b share the same hostname,
// compared case-insensitively.
func SameHostname(a, b *url.URL) bool {
	if a == nil || b == nil {
		return a == b
	}
	return strings.EqualFold(a.Hostname(), b.Hostname())
}
