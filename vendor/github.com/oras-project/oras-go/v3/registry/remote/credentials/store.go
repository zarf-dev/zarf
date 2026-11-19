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

// Package credentials supports reading, saving, and removing credentials from
// Docker configuration files and external credential stores that follow
// the Docker credential helper protocol.
//
// Reference: https://docs.docker.com/engine/reference/commandline/login/#credential-stores
package credentials

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/oras-project/oras-go/v3/registry/remote/internal/configfile"
)

// ConfigFile is the interface for a Docker configuration file that provides
// credential storage capabilities. It is implemented by [config.Config] from
// the config package.
type ConfigFile = configfile.ConfigFile

const (
	dockerConfigDirEnv   = "DOCKER_CONFIG"
	dockerConfigFileDir  = ".docker"
	dockerConfigFileName = "config.json"
)

// Getter retrieves credentials.
type Getter interface {
	// Get retrieves credentials from the store for the given server address.
	Get(ctx context.Context, serverAddress string) (Credential, error)
}

// Putter saves credentials.
type Putter interface {
	// Put saves credentials into the store for the given server address.
	Put(ctx context.Context, serverAddress string, cred Credential) error
}

// Deleter removes credentials.
type Deleter interface {
	// Delete removes credentials from the store for the given server address.
	Delete(ctx context.Context, serverAddress string) error
}

// Store is the interface that any credentials store must implement.
type Store interface {
	Getter
	Putter
	Deleter
}

// NamespaceMatcher is an optional interface that a [Getter] may implement to
// report that its Get already performs most-specific-first namespace matching
// for a server address.
//
// Resolving the credential for a namespaced resource walks that resource's
// namespaces from most specific to least specific. A store that matches
// namespaces itself is asked once, with the full namespaced key, and its
// answer is final; a store that does not is asked for each namespace in turn,
// stopping at the first non-empty credential.
//
// The distinction matters only for a key that is present but holds no
// credential. A matching store can tell that entry apart from an absent one
// and treats it as anonymous access for that namespace, as
// containers-auth.json does. A non-matching store cannot, because
// [Getter.Get] reports both as [EmptyCredential] with a nil error, so the walk
// continues past it.
//
// A store that does not implement this interface is treated as not matching.
//
// NamespaceMatcher is provisional: it is the right shape for a string-keyed
// [Getter], and a future resource-keyed getter would own the namespace walk by
// construction and subsume it.
type NamespaceMatcher interface {
	Getter

	// MatchesNamespace reports whether Get performs most-specific-first
	// namespace matching for serverAddress.
	MatchesNamespace(serverAddress string) bool
}

// DynamicStore dynamically determines which store to use based on the settings
// in the config file.
type DynamicStore struct {
	config             ConfigFile
	options            StoreOptions
	detectedCredsStore string
}

// StoreOptions provides options for NewStore.
type StoreOptions struct {
	// AllowPlaintextPut allows saving credentials in plaintext in the config
	// file.
	//   - If AllowPlaintextPut is set to false (default value), Put() will
	//     return an error when native store is not available.
	//   - If AllowPlaintextPut is set to true, Put() will save credentials in
	//     plaintext in the config file when native store is not available.
	AllowPlaintextPut bool

	// IgnoreDefaultNativeStore disables detecting the platform-default native
	// credentials store when the config file has no authentication information.
	//
	// By default (false), the store will detect and use the platform-default
	// native credentials store:
	//   - Windows: "wincred"
	//   - Linux: "pass" or "secretservice"
	//   - macOS: "osxkeychain"
	//
	// Set to true to skip native store detection and fall back to the
	// plain-text config file.
	//
	// References:
	//   - https://docs.docker.com/engine/reference/commandline/login/#credentials-store
	//   - https://docs.docker.com/engine/reference/commandline/cli/#docker-cli-configuration-file-configjson-properties
	IgnoreDefaultNativeStore bool

	// Hierarchical enables longest-prefix namespace matching when reading
	// credentials from the plaintext config file, as used by
	// containers-auth.json (Podman/Buildah). When false (default), exact
	// hostname matching is used, as in Docker config.json.
	//
	// This only affects credential lookup in the plaintext file store. A
	// credential helper is always selected by nearest configured parent, and
	// the selected helper and native stores are always keyed by the exact
	// server address, regardless of this option.
	//
	// It also makes the store authoritative for namespace matching, reported
	// through [NamespaceMatcher]: a key that is present but holds no
	// credential means anonymous access for that namespace, rather than a
	// miss that falls back to the parent namespace.
	Hierarchical bool
}

// NewStoreFromConfig returns a Store based on the given ConfigFile.
// This allows creating a store from an already-loaded config file.
func NewStoreFromConfig(cfg ConfigFile, opts StoreOptions) *DynamicStore {
	ds := &DynamicStore{
		config:  cfg,
		options: opts,
	}
	if !opts.IgnoreDefaultNativeStore && !cfg.IsAuthConfigured() {
		// no authentication configured, detect the default credentials store
		ds.detectedCredsStore = getDefaultHelperSuffix()
	}
	return ds
}

// NewStore returns a Store based on the given configuration file.
//
// For Get(), Put() and Delete(), the returned Store will dynamically determine
// which underlying credentials store to use for the given server address.
// The underlying credentials store is determined in the following order:
//  1. Native server-specific credential helper
//  2. Native credentials store
//  3. The plain-text config file itself
//
// A server-specific "credHelpers" entry that is empty selects the plain-text
// config file for that server, skipping the native credentials store.
//
// References:
//   - https://docs.docker.com/engine/reference/commandline/login/#credentials-store
//   - https://docs.docker.com/engine/reference/commandline/cli/#docker-cli-configuration-file-configjson-properties
func NewStore(configPath string, opts StoreOptions) (*DynamicStore, error) {
	cfg, err := configfile.Load(configPath)
	if err != nil {
		return nil, err
	}
	return NewStoreFromConfig(cfg, opts), nil
}

// NewStoreFromDocker returns a Store based on the default docker config file.
//   - If the $DOCKER_CONFIG environment variable is set,
//     $DOCKER_CONFIG/config.json will be used.
//   - Otherwise, the default location $HOME/.docker/config.json will be used.
//
// NewStoreFromDocker internally calls [NewStore].
//
// References:
//   - https://docs.docker.com/engine/reference/commandline/cli/#configuration-files
//   - https://docs.docker.com/engine/reference/commandline/cli/#change-the-docker-directory
func NewStoreFromDocker(opt StoreOptions) (*DynamicStore, error) {
	configPath, err := getDockerConfigPath()
	if err != nil {
		return nil, err
	}
	return NewStore(configPath, opt)
}

// Get retrieves credentials from the store for the given server address.
//
// Callers that walk namespaces call this once per path segment, so a helper
// inherited from a parent namespace may be executed several times per lookup.
func (ds *DynamicStore) Get(ctx context.Context, serverAddress string) (Credential, error) {
	return ds.getStore(serverAddress).Get(ctx, serverAddress)
}

// MatchesNamespace reports whether Get performs most-specific-first namespace
// matching for serverAddress. It does when [StoreOptions].Hierarchical is set
// and the store selected for serverAddress is the plaintext config file; a
// native store or credential helper is always keyed by the exact server
// address.
//
// MatchesNamespace implements [NamespaceMatcher].
func (ds *DynamicStore) MatchesNamespace(serverAddress string) bool {
	matcher, ok := ds.getStore(serverAddress).(NamespaceMatcher)
	return ok && matcher.MatchesNamespace(serverAddress)
}

// Put saves credentials into the store for the given server address.
// Put returns ErrPlaintextPutDisabled if native store is not available and
// [StoreOptions].AllowPlaintextPut is set to false.
//
// A detected native store is used for this store's own operations only; it is
// deliberately not written back to the config file. Persisting it would edit a
// file this library does not own — one shared with the Docker CLI and every
// other tool that reads it — as a side effect of saving one credential, and
// would change their behaviour too. Callers that want the setting recorded
// should write it themselves.
func (ds *DynamicStore) Put(ctx context.Context, serverAddress string, cred Credential) error {
	return ds.getStore(serverAddress).Put(ctx, serverAddress, cred)
}

// Delete removes credentials from the store for the given server address.
//
// When the address is served by a credential helper, Delete also removes the
// plain-text entry for it from the config file, as the Docker CLI does. A
// credential saved before the helper was configured would otherwise stay in
// the file.
func (ds *DynamicStore) Delete(ctx context.Context, serverAddress string) error {
	if err := ds.getStore(serverAddress).Delete(ctx, serverAddress); err != nil {
		return err
	}
	return ds.config.DeleteAuthConfig(serverAddress)
}

// IsAuthConfigured returns whether there is authentication configured in the
// config file or not.
//
// IsAuthConfigured returns true when:
//   - The "credsStore" field is not empty
//   - Or the "credHelpers" field is not empty
//   - Or there is any entry in the "auths" field
func (ds *DynamicStore) IsAuthConfigured() bool {
	return ds.config.IsAuthConfigured()
}

// ConfigPath returns the path to the config file.
func (ds *DynamicStore) ConfigPath() string {
	return ds.config.Path()
}

// getHelperSuffix returns the credential helper suffix for the given server
// address, and whether it was configured for that address itself rather than
// inherited from a parent namespace.
//
// "credHelpers" is keyed by registry host, so a namespaced address
// ("host/path") resolves to the helper configured for its nearest parent,
// down to the bare host. The resolved helper is queried with the full address,
// path included. Note that containers/image queries the helper with the host
// alone, so a credential stored here under a namespaced key is not visible to
// Podman or the Docker CLI.
//
// "credsStore" and the detected platform default are global stores configured
// for registry hosts. When queried for a namespaced address, they are probes of
// a key the store was never configured for, so exact is false.
//
// A "credHelpers" entry that is present but empty selects the plain-text config
// file: the returned suffix is empty and "credsStore" is not consulted.
func (ds *DynamicStore) getHelperSuffix(serverAddress string) (suffix string, exact bool) {
	// 1. Look for a server-specific credential helper, then for the helper of
	// each parent namespace.
	key := serverAddress
	for {
		if helper, found := ds.config.GetCredentialHelper(key); found {
			return helper, key == serverAddress
		}
		i := strings.LastIndex(key, "/")
		if i <= 0 {
			break
		}
		key = key[:i]
	}
	// 2. Then look for the configured native store
	if credsStore := ds.config.CredentialsStore(); credsStore != "" {
		return credsStore, !isNamespaced(serverAddress)
	}
	// 3. Use the detected default store
	return ds.detectedCredsStore, !isNamespaced(serverAddress)
}

// isNamespaced reports whether serverAddress represents a namespaced key
// rather than a host-level key.
// The docker.io sentinel "https://index.docker.io/v1/" contains '/' but is
// a host-level key rather than a namespace.
func isNamespaced(serverAddress string) bool {
	addr := strings.TrimPrefix(serverAddress, "http://")
	addr = strings.TrimPrefix(addr, "https://")
	addr = strings.TrimSuffix(addr, "/")
	if addr == "index.docker.io/v1" {
		return false
	}
	return strings.Contains(addr, "/")
}

// getStore returns a store for the given server address.
func (ds *DynamicStore) getStore(serverAddress string) Store {
	if helper, exact := ds.getHelperSuffix(serverAddress); helper != "" {
		if !exact {
			return inheritedHelperStore{NewNativeStore(helper)}
		}
		return NewNativeStore(helper)
	}

	fs := newFileStore(ds.config)
	fs.DisablePut = !ds.options.AllowPlaintextPut
	fs.Hierarchical = ds.options.Hierarchical
	return fs
}

// inheritedHelperStore is a credential helper configured for a parent
// namespace or whole host. Get probes it with a key it was never configured
// for, so an error there is a miss rather than a failure.
type inheritedHelperStore struct {
	Store
}

// Get retrieves credentials from the helper, reporting a failed probe as a miss
// so that the caller falls back to the parent namespace.
func (s inheritedHelperStore) Get(ctx context.Context, serverAddress string) (Credential, error) {
	cred, err := s.Store.Get(ctx, serverAddress)
	if err != nil {
		if ctx.Err() != nil {
			return EmptyCredential, err
		}
		return EmptyCredential, nil
	}
	return cred, nil
}

// getDockerConfigPath returns the path to the default docker config file.
func getDockerConfigPath() (string, error) {
	// first try the environment variable
	configDir := os.Getenv(dockerConfigDirEnv)
	if configDir == "" {
		// then try home directory
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		configDir = filepath.Join(homeDir, dockerConfigFileDir)
	}
	return filepath.Join(configDir, dockerConfigFileName), nil
}

// storeWithFallbacks is a store that has multiple fallback stores.
type storeWithFallbacks struct {
	stores []Store
}

// NewStoreWithFallbacks returns a new store based on the given stores.
//   - Get() searches the primary and the fallback stores
//     for the credentials and returns when it finds the
//     credentials in any of the stores.
//   - Put() saves the credentials into the primary store.
//   - Delete() deletes the credentials from the primary store.
func NewStoreWithFallbacks(primary Store, fallbacks ...Store) Store {
	if len(fallbacks) == 0 {
		return primary
	}
	return &storeWithFallbacks{
		stores: append([]Store{primary}, fallbacks...),
	}
}

// Get retrieves credentials from the StoreWithFallbacks for the given server.
// It searches the primary and the fallback stores for the credentials of serverAddress
// and returns when it finds the credentials in any of the stores.
func (sf *storeWithFallbacks) Get(ctx context.Context, serverAddress string) (Credential, error) {
	for _, s := range sf.stores {
		cred, err := s.Get(ctx, serverAddress)
		if err != nil {
			return EmptyCredential, err
		}
		if cred != EmptyCredential {
			return cred, nil
		}
	}
	return EmptyCredential, nil
}

// MatchesNamespace reports whether Get performs most-specific-first namespace
// matching for serverAddress. It does only when every store in the chain does,
// since a store that does not has to be walked namespace by namespace, and the
// chain cannot walk one store without walking them all.
//
// MatchesNamespace implements [NamespaceMatcher].
func (sf *storeWithFallbacks) MatchesNamespace(serverAddress string) bool {
	for _, s := range sf.stores {
		matcher, ok := s.(NamespaceMatcher)
		if !ok || !matcher.MatchesNamespace(serverAddress) {
			return false
		}
	}
	return true
}

// Put saves credentials into the StoreWithFallbacks. It puts
// the credentials into the primary store.
func (sf *storeWithFallbacks) Put(ctx context.Context, serverAddress string, cred Credential) error {
	return sf.stores[0].Put(ctx, serverAddress, cred)
}

// Delete removes credentials from the StoreWithFallbacks for the given server.
// It deletes the credentials from the primary store.
func (sf *storeWithFallbacks) Delete(ctx context.Context, serverAddress string) error {
	return sf.stores[0].Delete(ctx, serverAddress)
}
