package image

import (
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/scylladb/go-set/strset"

	"github.com/anchore/stereoscope/internal/log"
)

// RegistryCredentials contains any information necessary to authenticate against an OCI-distribution-compliant
// registry (either with basic auth, or bearer token, or ggcr authenticator implementation).
// Note: only valid for the OCI registry provider.
type RegistryCredentials struct {
	Authority string
	Username  string
	Password  string
	Token     string

	// Explicitly pass in the Authenticator, allowing for things like
	// k8schain to be passed through explicitly.
	Authenticator authn.Authenticator

	// MTLS configuration
	ClientCert string
	ClientKey  string
}

// authenticator returns an authn.Authenticator for the given credentials.
// Authentication methods are attempted in the following order until a viable method is found: (1) basic auth,
// (2) bearer token. If no viable authentication method is found, authenticator returns nil.
func (c RegistryCredentials) authenticator() authn.Authenticator {
	if c.Authenticator != nil {
		return c.Authenticator
	}
	if c.Username != "" && c.Password != "" {
		log.Debugf("using basic auth for registry %q", c.Authority)
		return &authn.Basic{
			Username: c.Username,
			Password: c.Password,
		}
	}

	if c.Token != "" {
		log.Debugf("using token for registry %q", c.Authority)
		return &authn.Bearer{
			Token: c.Token,
		}
	}

	return nil
}

// canBeUsedWithRegistry returns a bool indicating if these credentials should be used when accessing the given registry.
func (c RegistryCredentials) canBeUsedWithRegistry(registry string) bool {
	if !c.hasAuthoritySpecified() {
		return true
	}

	// the containerd code will normalize docker.io requests to registry-1.docker.io , however
	// it might be that the user has configured docker.io specifically in the credentials.
	// try again with the new host. The same can occur when asking for docker.io directly, containerd
	// will transform this to index.docker.io.
	dockerAliases := strset.New("registry-1.docker.io", "index.docker.io", "docker.io")
	if dockerAliases.Has(c.Authority) && dockerAliases.Has(registry) {
		// these are all the same in terms of auth
		return true
	}

	// find an exact match
	return registry == c.Authority
}

// hasAuthoritySpecified returns a bool indicating if there is a specified "authority" value,
// meaning that the user has requested these credentials to be used for retrieving only the images whose registry
// matches this "authority" value.
func (c RegistryCredentials) hasAuthoritySpecified() bool {
	return c.Authority != ""
}
