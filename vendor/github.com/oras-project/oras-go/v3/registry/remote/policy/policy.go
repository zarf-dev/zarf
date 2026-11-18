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

// Package policy implements support for containers-policy.json format
// for OCI image signature verification policies.
//
// Reference: https://man.archlinux.org/man/containers-policy.json.5.en
package policy

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	// ErrDenied is returned when a repository operation is denied by policy.
	ErrDenied = errors.New("access denied by policy")
	// ErrNoPolicyFound is returned by GetDefaultPolicyPath when no policy.json
	// file exists at any of the default search locations.
	ErrNoPolicyFound = errors.New("no policy.json found")
)

const (
	// policyConfUserDir is the user-level configuration directory for policy.json
	policyConfUserDir = ".config/containers"
	// policyConfFileName is the name of the policy configuration file
	policyConfFileName = "policy.json"
)

// Policy represents a containers-policy.json configuration
type Policy struct {
	// Default is the default policy requirement
	Default PolicyRequirements `json:"default"`
	// Transports contains transport-specific policy scopes
	Transports map[TransportName]TransportScopes `json:"transports,omitempty"`
}

// TransportScopes represents scopes within a transport
type TransportScopes map[string]PolicyRequirements

// PolicyRequirements is a list of policy requirements
type PolicyRequirements []PolicyRequirement

// PolicyRequirement represents a single policy requirement
type PolicyRequirement interface {
	// Type returns the type of requirement
	Type() string
	// Validate validates the requirement configuration
	Validate() error
}

// NewPolicy creates a new empty Policy.
// Use this for programmatic policy construction without a file.
func NewPolicy() *Policy {
	return &Policy{
		Default:    make(PolicyRequirements, 0),
		Transports: make(map[TransportName]TransportScopes),
	}
}

// NewInsecureAcceptAnythingPolicy creates a policy that accepts all images.
// This is useful for testing or development environments.
func NewInsecureAcceptAnythingPolicy() *Policy {
	return &Policy{
		Default: PolicyRequirements{&InsecureAcceptAnything{}},
	}
}

// NewRejectAllPolicy creates a policy that rejects all images.
// This is a safe default that requires explicit configuration to allow images.
func NewRejectAllPolicy() *Policy {
	return &Policy{
		Default: PolicyRequirements{&Reject{}},
	}
}

// SetDefault sets the default policy requirements.
func (p *Policy) SetDefault(reqs ...PolicyRequirement) *Policy {
	p.Default = reqs
	return p
}

// SetTransportScope sets the policy requirements for a specific transport and scope.
func (p *Policy) SetTransportScope(transport TransportName, scope string, reqs ...PolicyRequirement) *Policy {
	if p.Transports == nil {
		p.Transports = make(map[TransportName]TransportScopes)
	}
	if p.Transports[transport] == nil {
		p.Transports[transport] = make(TransportScopes)
	}
	p.Transports[transport][scope] = reqs
	return p
}

// GetDefaultPolicyPath returns the default path to policy.json.
// It checks $HOME/.config/containers/policy.json first, then falls back to
// the system-wide path.
//
// On Linux, the system-wide path is /etc/containers/policy.json.
// On other platforms (macOS, Windows, etc.), only the user-level path
// ($HOME/.config/containers/policy.json) is checked, since the system-wide
// path is Linux-specific. Use [LoadPolicy] with an explicit path
// for cross-platform usage.
func GetDefaultPolicyPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	// Try user-specific path first (works on all platforms)
	userPath := filepath.Join(homeDir, policyConfUserDir, policyConfFileName)
	if _, err := os.Stat(userPath); err == nil {
		return userPath, nil
	}

	// Fall back to system-wide path (Linux only)
	if systemPolicyPath != "" {
		return systemPolicyPath, nil
	}

	return "", fmt.Errorf("%w: checked %s and no system-wide default is available on this platform", ErrNoPolicyFound, userPath)
}

// LoadPolicy loads a policy from the specified file path.
func LoadPolicy(path string) (*Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read policy file %s: %w", path, err)
	}

	var policy Policy
	if err := json.Unmarshal(data, &policy); err != nil {
		return nil, fmt.Errorf("failed to parse policy file %s: %w", path, err)
	}

	return &policy, nil
}

// LoadDefault loads the policy from the default location.
// On non-Linux platforms, this only checks the user-level path.
// See [GetDefaultPolicyPath] for details on path resolution.
func LoadDefault() (*Policy, error) {
	path, err := GetDefaultPolicyPath()
	if err != nil {
		return nil, err
	}

	return LoadPolicy(path)
}

// Save saves a policy to the specified file path.
func (p *Policy) Save(path string) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal policy: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write policy file %s: %w", path, err)
	}

	return nil
}

// Validate validates the policy configuration.
func (p *Policy) Validate() error {
	// Per spec: the global default set of policy requirements is mandatory
	// and the array must not be empty.
	if len(p.Default) == 0 {
		return fmt.Errorf("default policy requirements must not be empty")
	}

	// Validate default requirements
	for _, req := range p.Default {
		if err := req.Validate(); err != nil {
			return fmt.Errorf("invalid default requirement: %w", err)
		}
	}

	// Validate transport-specific requirements
	for transport, scopes := range p.Transports {
		for scope, reqs := range scopes {
			for _, req := range reqs {
				if err := req.Validate(); err != nil {
					return fmt.Errorf("invalid requirement for transport %s scope %s: %w", transport, scope, err)
				}
			}
		}
	}

	return nil
}

// GetRequirementsForImage returns the policy requirements for a given transport and scope.
// It follows the containers-policy.json precedence rules for the docker transport:
// exact match > longest-prefix match > wildcard subdomain match > transport default > global default.
// For non-docker transports, it uses exact match, then transport default, then global default.
func (p *Policy) GetRequirementsForImage(transport TransportName, scope string) PolicyRequirements {
	transportScopes, ok := p.Transports[transport]
	if !ok {
		return p.Default
	}

	// Try exact scope match first
	if reqs, ok := transportScopes[scope]; ok {
		return reqs
	}

	// A scope may name an exact tag or digest, which is the most specific form
	// containers-policy.json allows. An entry for the repository still applies
	// to a tagged or digested image, so fall back to it. This is deliberately
	// not limited to the docker transport: an oci scope is legitimately
	// "/path/to/dir:tag", and gating it would let such an entry be skipped.
	if repoScope := trimScopeReference(scope); repoScope != scope {
		if reqs, ok := transportScopes[repoScope]; ok {
			return reqs
		}
		scope = repoScope
	}

	// For docker transport, try longest-prefix match and wildcard subdomain match
	if transport == TransportNameDocker {
		// Try longest-prefix match: the scope key is a prefix of the image
		// scope at a "/" boundary
		bestMatch := ""
		for key := range transportScopes {
			if key == "" {
				continue
			}
			if strings.HasPrefix(key, "*.") {
				continue // Skip wildcard entries during prefix matching
			}
			if isPathPrefix(key, scope) && len(key) > len(bestMatch) {
				bestMatch = key
			}
		}
		if bestMatch != "" {
			return transportScopes[bestMatch]
		}

		// Try wildcard subdomain match: *.example.com matches sub.example.com/repo
		bestWildcard := ""
		for key := range transportScopes {
			if !strings.HasPrefix(key, "*.") {
				continue
			}
			// *.example.com should match sub.example.com (and sub.example.com/repo)
			suffix := key[1:] // ".example.com"
			host := scope
			if idx := strings.Index(scope, "/"); idx != -1 {
				host = scope[:idx]
			}
			// A port is not part of the subdomain match.
			host, _, _ = strings.Cut(host, ":")
			if strings.HasSuffix(host, suffix) && len(key) > len(bestWildcard) {
				bestWildcard = key
			}
		}
		if bestWildcard != "" {
			return transportScopes[bestWildcard]
		}
	}

	// Try transport default (empty scope)
	if reqs, ok := transportScopes[""]; ok {
		return reqs
	}

	// Fall back to global default
	return p.Default
}

// trimScopeReference drops a trailing tag or digest from a scope, leaving the
// registry and repository. A reference can only carry one in its last path
// component, so the search starts after the last "/": that keeps a registry
// port ("example.com:5000"), which has no "/" after it, and an "@" or ":"
// inside an oci filesystem path ("/mnt/data@2024/img"). The first separator
// wins, so "example.com/repo:tag@sha256:..." trims to "example.com/repo" in
// one step.
func trimScopeReference(scope string) string {
	slash := strings.LastIndex(scope, "/")
	if slash == -1 {
		return scope
	}
	if i := strings.IndexAny(scope[slash+1:], ":@"); i != -1 {
		return scope[:slash+1+i]
	}
	return scope
}

// isPathPrefix reports whether prefix is a prefix of s at a "/" boundary.
// That is, prefix matches s if s == prefix or s starts with prefix + "/".
func isPathPrefix(prefix, s string) bool {
	if s == prefix {
		return true
	}
	return strings.HasPrefix(s, prefix+"/")
}
