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

import "net/netip"

// ReferrersAPI represents the Referrers API capability of a registry.
type ReferrersAPI int

const (
	// ReferrersAPIUnknown indicates that the Referrers API capability is unknown.
	ReferrersAPIUnknown ReferrersAPI = iota
	// ReferrersAPISupported indicates that the registry supports the Referrers API.
	ReferrersAPISupported
	// ReferrersAPIUnsupported indicates that the registry does not support the Referrers API.
	ReferrersAPIUnsupported
)

// String returns the string representation of ReferrersAPI.
func (r ReferrersAPI) String() string {
	switch r {
	case ReferrersAPISupported:
		return "supported"
	case ReferrersAPIUnsupported:
		return "unsupported"
	default:
		return "unknown"
	}
}

// TokenFlow selects how a bearer token is acquired when a registry answers with
// a Bearer challenge and the credential is a username/password pair.
//
// It does not choose between authentication schemes: the scheme always follows
// the registry's WWW-Authenticate challenge, and under both flows the registry
// itself is authenticated with a bearer token. Only the exchange at the token
// endpoint differs. Anonymous, access-token, and refresh-token credentials are
// unaffected.
type TokenFlow int

const (
	// TokenFlowDefault leaves the choice to the client, which uses OAuth2 and
	// falls back to the distribution-spec flow when the token endpoint does not
	// support it.
	TokenFlowDefault TokenFlow = iota
	// TokenFlowOAuth2 posts an OAuth2 password grant to the token endpoint and
	// does not fall back to the distribution-spec flow if it is rejected.
	// Reference: https://distribution.github.io/distribution/spec/auth/oauth/
	TokenFlowOAuth2
	// TokenFlowDistribution performs a distribution-spec GET against the token
	// endpoint, passing the credential as HTTP Basic. This is the flow oras-go
	// v1 and v2 used. The OAuth2 endpoint is a Docker extension rather than
	// part of the OCI distribution spec, so registries that implement only the
	// spec'd endpoint require this flow.
	// Reference: https://distribution.github.io/distribution/spec/auth/token/
	TokenFlowDistribution
)

// String returns the string representation of TokenFlow.
func (t TokenFlow) String() string {
	switch t {
	case TokenFlowOAuth2:
		return "oauth2"
	case TokenFlowDistribution:
		return "distribution"
	default:
		return "default"
	}
}

// Attributes contains properties specific to the registry itself.
type Attributes struct {
	// ReferrersAPI indicates the Referrers API capability of the registry.
	// - ReferrersAPISupported: the registry supports the Referrers API
	// - ReferrersAPIUnsupported: the registry does not support the Referrers API
	// - ReferrersAPIUnknown: the capability is unknown and will be auto-detected
	ReferrersAPI ReferrersAPI

	// TokenFlow selects how a bearer token is acquired for username/password
	// credentials. TokenFlowDefault leaves the choice to the client.
	TokenFlow TokenFlow

	// BlockedRealmPrefixes is copied onto auth.Client as-is. nil uses
	// auth.DefaultBlockedRealmPrefixes; an empty non-nil slice blocks
	// nothing; a non-empty slice is the exact denylist.
	BlockedRealmPrefixes []netip.Prefix
}
