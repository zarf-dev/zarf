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

package credentials

import (
	"github.com/oras-project/oras-go/v3/internal/authtype"
)

// AuthConfig contains authorization information for connecting to a Registry.
// Deprecated: Use config.AuthConfig instead.
type AuthConfig = authtype.AuthConfig

// ErrInvalidAuthConfig is returned when the auth config format is invalid.
// Deprecated: Use config.ErrInvalidAuthConfig instead.
var ErrInvalidAuthConfig = authtype.ErrInvalidAuthConfig

// NewAuthConfig creates an AuthConfig based on credential components.
// Deprecated: Use config.NewAuthConfig instead.
var NewAuthConfig = authtype.NewAuthConfig

// EncodeAuth base64-encodes username and password into base64(username:password).
// Deprecated: Use config.EncodeAuth instead.
var EncodeAuth = authtype.EncodeAuth
