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

// Package properties provides types for describing registry configuration
// and attributes. These types are designed to be used for configuring
// connections to remote OCI registries, including transport settings,
// authentication credentials, and registry-specific attributes.
//
// The main types in this package are:
//   - Registry: Contains complete configuration for connecting to a remote registry
//   - Transport: Describes transport layer settings (TLS, HTTP, headers)
//   - Attributes: Registry-specific settings (Referrers API, token flow,
//     blocked realm prefixes). BlockedRealmPrefixes nil uses auth defaults;
//     an empty non-nil slice blocks no realm addresses.
//     registries.conf drop-ins replace a whole [[registry]] row by prefix,
//     so field-level settings are not merged across layers.
//     Mirror clients inherit BlockedRealmPrefixes from the primary, like TokenFlow.
//
// Example usage:
//
//	// Create a new registry configuration
//	reg := properties.NewRegistry("ghcr.io", "myorg/myrepo")
//
//	// Configure transport settings
//	reg.Transport.PlainHTTP = false
//	reg.Transport.Insecure = false
//
//	// Set credentials
//	reg.Credential = credentials.Credential{
//		Username: "user",
//		Password: "token",
//	}
//
//	// Configure registry attributes
//	reg.Attributes.ReferrersAPI = properties.ReferrersAPISupported
//	reg.Attributes.BlockedRealmPrefixes = auth.PrivateRealmPrefixes()
package properties
