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

// Package authtype provides the shared AuthConfig type used by both the config
// and credentials packages to avoid import cycles.
package authtype

import (
	"encoding/base64"
	"errors"
	"strings"
)

// ErrInvalidAuthConfig is returned when the auth config format is invalid.
var ErrInvalidAuthConfig = errors.New("invalid auth config")

// AuthConfig contains authorization information for connecting to a Registry.
// References:
//   - https://github.com/docker/cli/blob/v24.0.0-beta.2/cli/config/configfile/file.go#L17-L45
//   - https://github.com/docker/cli/blob/v24.0.0-beta.2/cli/config/types/authconfig.go#L3-L22
type AuthConfig struct {
	// Auth is a base64-encoded string of "{username}:{password}".
	Auth string `json:"auth,omitempty"`
	// IdentityToken is used to authenticate the user and get an access token
	// for the registry.
	IdentityToken string `json:"identitytoken,omitempty"`
	// RegistryToken is a bearer token to be sent to a registry.
	RegistryToken string `json:"registrytoken,omitempty"`

	Username string `json:"username,omitempty"` // legacy field for compatibility
	Password string `json:"password,omitempty"` // legacy field for compatibility
}

// NewAuthConfig creates an AuthConfig based on credential components.
func NewAuthConfig(username, password, refreshToken, accessToken string) AuthConfig {
	return AuthConfig{
		Auth:          EncodeAuth(username, password),
		IdentityToken: refreshToken,
		RegistryToken: accessToken,
	}
}

// EncodeAuth base64-encodes username and password into base64(username:password).
func EncodeAuth(username, password string) string {
	if username == "" && password == "" {
		return ""
	}
	return base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
}

// DecodeAuth decodes the base64-encoded Auth field and returns username and password.
func (ac AuthConfig) DecodeAuth() (username string, password string, err error) {
	if ac.Auth == "" {
		return "", "", nil
	}

	decoded, err := base64.StdEncoding.DecodeString(ac.Auth)
	if err != nil {
		return "", "", err
	}
	decodedStr := string(decoded)
	username, password, ok := strings.Cut(decodedStr, ":")
	if !ok {
		return "", "", errors.New("auth does not conform the base64(username:password) format")
	}
	return username, password, nil
}
