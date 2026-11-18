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

// Package configfile implements the Docker configuration file format and the
// interface credential stores consume it through.
//
// It exists so that registry/remote/config and registry/remote/credentials can
// share these types without either importing the other. config imports
// credentials (to build credential stores), so credentials cannot import config
// to load a config file. Putting the file format here breaks that cycle
// directly, instead of routing around it with a registration hook.
package configfile

import (
	"github.com/oras-project/oras-go/v3/internal/authtype"
)

// AuthConfig contains authorization information for connecting to a Registry.
type AuthConfig = authtype.AuthConfig

// ErrInvalidAuthConfig is returned when the auth config format is invalid.
var ErrInvalidAuthConfig = authtype.ErrInvalidAuthConfig

// ConfigFile is the interface for a Docker configuration file that provides
// credential storage capabilities. It is implemented by [Config].
type ConfigFile interface {
	// GetAuthConfig returns the AuthConfig for the given server address using
	// exact hostname matching (Docker config.json semantics).
	GetAuthConfig(serverAddress string) (AuthConfig, error)
	// GetAuthConfigHierarchical returns the AuthConfig for the given server
	// address using longest-prefix namespace matching (containers-auth.json
	// semantics).
	GetAuthConfigHierarchical(serverAddress string) (AuthConfig, error)
	// PutAuthConfig saves the AuthConfig for the given server address.
	PutAuthConfig(serverAddress string, authCfg AuthConfig) error
	// DeleteAuthConfig removes the AuthConfig for the given server address.
	DeleteAuthConfig(serverAddress string) error
	// GetCredentialHelper returns the credential helper configured for the
	// server, and whether the config has an entry for it. An entry may be empty.
	GetCredentialHelper(serverAddress string) (helper string, found bool)
	// CredentialsStore returns the configured credentials store name.
	CredentialsStore() string
	// IsAuthConfigured returns true if any authentication is configured.
	IsAuthConfigured() bool
	// Path returns the path to the config file.
	Path() string
}

// Compile-time check that Config satisfies the interface credential stores use.
var _ ConfigFile = (*Config)(nil)
