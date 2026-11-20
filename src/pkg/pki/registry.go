// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package pki

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"

	"github.com/zarf-dev/zarf/src/types"
)

// RegistryTransport clones base and adds registry trust roots and a client identity.
// A nil base uses http.DefaultTransport. Existing transport settings are preserved.
func RegistryTransport(base *http.Transport, opts types.RegistryTLSOptions) (*http.Transport, error) {
	if (opts.CertFile == "") != (opts.KeyFile == "") {
		return nil, fmt.Errorf("registry client certificate and private key must be provided together")
	}
	if base == nil {
		var ok bool
		base, ok = http.DefaultTransport.(*http.Transport)
		if !ok {
			return nil, fmt.Errorf("default HTTP transport is not an http.Transport")
		}
	}
	transport := base.Clone()
	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = &tls.Config{}
	} else {
		transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	}
	if opts.CAFile != "" {
		ca, err := os.ReadFile(opts.CAFile)
		if err != nil {
			return nil, fmt.Errorf("unable to read registry CA bundle: %w", err)
		}
		roots := transport.TLSClientConfig.RootCAs
		if roots == nil {
			roots, err = x509.SystemCertPool()
			if err != nil || roots == nil {
				roots = x509.NewCertPool()
			}
		} else {
			roots = roots.Clone()
		}
		if !roots.AppendCertsFromPEM(ca) {
			return nil, fmt.Errorf("registry CA bundle contains no valid certificates")
		}
		transport.TLSClientConfig.RootCAs = roots
	}
	if opts.CertFile != "" {
		cert, err := tls.LoadX509KeyPair(opts.CertFile, opts.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("unable to load registry client certificate and private key: %w", err)
		}
		transport.TLSClientConfig.Certificates = []tls.Certificate{cert}
	}
	return transport, nil
}
