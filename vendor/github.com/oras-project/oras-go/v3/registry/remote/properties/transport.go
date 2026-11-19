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

package properties

// Transport contains transport configuration of a remote registry.
type Transport struct {
	// CACert is the path to the CA certificate file for verifying the registry's certificate.
	CACert string

	// CACerts is a list of paths to CA certificate files. All certificates are
	// added to the trust pool. This is used by containers-certs.d discovery.
	CACerts []string

	// Cert is the path to the client certificate file for mutual TLS authentication.
	Cert string

	// Key is the path to the client private key file for mutual TLS authentication.
	Key string

	// Insecure allows connections to registries with invalid or self-signed certificates.
	// When true, TLS certificate verification is skipped.
	Insecure bool

	// PlainHTTP forces the use of HTTP instead of HTTPS when connecting to the registry.
	PlainHTTP bool

	// HeaderFlags contains custom HTTP headers to include in requests to the registry.
	// The key is the header name and the value is the header value.
	HeaderFlags map[string]string
}
