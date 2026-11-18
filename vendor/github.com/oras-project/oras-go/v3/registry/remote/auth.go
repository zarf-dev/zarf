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

package remote

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/oras-project/oras-go/v3/errdef"
	"github.com/oras-project/oras-go/v3/registry/remote/auth"
	"github.com/oras-project/oras-go/v3/registry/remote/credentials"
	"github.com/oras-project/oras-go/v3/registry/remote/properties"
)

// ErrClientTypeUnsupported is thrown by Login() when the registry's client type
// is not supported.
var ErrClientTypeUnsupported = errors.New("client type not supported")

// Login provides the login functionality with the given credentials. The
// target registry reference may be narrowed to a namespace or repository, but
// must not contain a tag or digest. The target registry's client should be nil
// or of type *auth.Client. Login uses a client local to the function and will
// not modify the original client of the registry.
//
// Login validates the credentials against the registry, but does not prove
// access to the target namespace or repository.
func Login(ctx context.Context, store credentials.Putter, reg *Registry, cred credentials.Credential) error {
	if reg.Reference.Tag != "" || reg.Reference.Digest != "" {
		return fmt.Errorf("%w: login target must not include a tag or digest", errdef.ErrInvalidReference)
	}
	resource := properties.Resource{
		Registry: reg.Reference.Registry,
		Path:     reg.Reference.Repository,
	}
	serverAddress, err := serverAddressFromResource(resource)
	if err != nil {
		return err
	}

	// Registry only contains copyable configuration. Make a local copy so
	// authentication changes do not modify the caller's registry.
	regClone := *reg
	// we use the original client if applicable, otherwise use a default client
	var authClient *auth.Client
	if reg.Client == nil {
		authClient = auth.DefaultClient.Clone()
	} else if client, ok := reg.Client.(*auth.Client); ok {
		authClient = client.Clone()
	} else {
		return ErrClientTypeUnsupported
	}
	// Disable the cache so that a token cached from an earlier authentication
	// cannot be replayed in place of the credential being validated.
	authClient.Cache = nil
	regClone.Client = authClient
	// update credentials with the client
	authClient.CredentialFunc = credentials.StaticCredentialFunc(reg.Reference.Registry, cred)
	// validate and store the credential
	if err := regClone.Ping(ctx); err != nil {
		return fmt.Errorf("failed to validate the credentials for %s: %w", resource, err)
	}
	if err := store.Put(ctx, serverAddress, cred); err != nil {
		return fmt.Errorf("failed to store the credentials for %s: %w", serverAddress, err)
	}
	return nil
}

// Logout provides the logout functionality for the given registry resource.
func Logout(ctx context.Context, store credentials.Deleter, resource properties.Resource) error {
	serverAddress, err := serverAddressFromResource(resource)
	if err != nil {
		return err
	}
	if err := store.Delete(ctx, serverAddress); err != nil {
		return fmt.Errorf("failed to delete the credential for %s: %w", serverAddress, err)
	}
	return nil
}

func serverAddressFromResource(resource properties.Resource) (string, error) {
	serverAddress := ServerAddressFromRegistry(resource.Registry)
	if resource.Path == "" {
		return serverAddress, nil
	}
	// Reject a path the read side could never produce, so a credential is
	// never stored under, or deleted by, an unreachable key.
	if err := (properties.Reference{Repository: resource.Path}).ValidateRepository(); err != nil {
		return "", err
	}
	if serverAddress != resource.Host() {
		return "", fmt.Errorf("%w: registry %q does not support path-scoped credentials", errdef.ErrUnsupported, resource.Registry)
	}
	return serverAddress + "/" + resource.Path, nil
}

// NewCredentialFunc returns a CredentialFunc that retrieves credentials from
// the given store. If store is nil, the returned function always returns
// EmptyCredential without error.
//
// # Lookup order
//
// The resource's namespaces are considered from most specific to least
// specific, following containers-auth.json. For
// "my-registry.local/namespace/user/image" that is:
//
//   - my-registry.local/namespace/user/image
//   - my-registry.local/namespace/user
//   - my-registry.local/namespace
//   - my-registry.local
//
// Which namespace wins when a more specific one is present but holds no
// credential depends on whether the store matches namespaces itself, reported
// through [credentials.NamespaceMatcher]:
//
//   - A matching store, such as a [credentials.DynamicStore] with
//     [credentials.StoreOptions].Hierarchical set, is asked once with the full
//     namespaced key and its answer is final, empty included. A present but
//     empty entry therefore means anonymous access for that namespace, and the
//     walk is left to the store rather than repeated over the same keys here.
//   - Any other store is asked for each namespace in turn and the first
//     non-empty credential wins. [credentials.Getter.Get] reports an absent key
//     and a key holding no credential alike, as
//     [credentials.EmptyCredential] with a nil error, so an empty result
//     cannot be read as a deliberate anonymous entry and the walk continues to
//     the parent namespace.
//
// Credentials for "docker.io" are keyed by a sentinel server address that is
// not a path root, so namespaces are not considered for it.
func NewCredentialFunc(store credentials.Getter) credentials.CredentialFunc {
	if store == nil {
		return func(context.Context, properties.Resource) (credentials.Credential, error) {
			return credentials.EmptyCredential, nil
		}
	}
	return func(ctx context.Context, res properties.Resource) (credentials.Credential, error) {
		host := ServerAddressFromHostname(res.Host())
		if host == "" {
			return credentials.EmptyCredential, nil
		}
		// ServerAddressFromHostname may map to a sentinel server address
		// (docker.io), which is a store key rather than a path root.
		if res.Path == "" || host != res.Host() {
			return store.Get(ctx, host)
		}

		key := host + "/" + res.Path
		if matcher, ok := store.(credentials.NamespaceMatcher); ok && matcher.MatchesNamespace(key) {
			// The store matches namespaces itself. Walking here as well would
			// repeat its work over the same keys and, by treating an empty
			// result as a miss, override the anonymous entry it just matched.
			return store.Get(ctx, key)
		}

		// Most-specific to least-specific, so namespaced credentials resolve
		// over any store, not only the hierarchical file store.
		for path := res.Path; path != ""; {
			cred, err := store.Get(ctx, host+"/"+path)
			if err != nil {
				return credentials.EmptyCredential, err
			}
			if cred != credentials.EmptyCredential {
				return cred, nil
			}
			index := strings.LastIndex(path, "/")
			if index < 0 {
				break
			}
			path = path[:index]
		}
		return store.Get(ctx, host)
	}
}

// ServerAddressFromRegistry maps a registry to a server address, which is used as
// a key for credentials store. The Docker CLI expects that the credentials of
// the registry 'registry-1.docker.io' or the alias 'docker.io' will be added
// under the key "https://index.docker.io/v1/".
// See: https://github.com/moby/moby/blob/v24.0.2/registry/config.go#L25-L48
func ServerAddressFromRegistry(registry string) string {
	if registry == "docker.io" ||
		registry == "registry-1.docker.io" {
		return "https://index.docker.io/v1/"
	}
	return registry
}

// ServerAddressFromHostname maps a hostname to a server address, which is used as
// a key for credentials store. It is expected that the traffic targetting the
// host "registry-1.docker.io" will be redirected to "https://index.docker.io/v1/".
// See: https://github.com/moby/moby/blob/v24.0.2/registry/config.go#L25-L48
func ServerAddressFromHostname(hostname string) string {
	if hostname == "registry-1.docker.io" {
		return "https://index.docker.io/v1/"
	}
	return hostname
}
