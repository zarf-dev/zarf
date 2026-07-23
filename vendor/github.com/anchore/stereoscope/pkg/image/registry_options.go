package image

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/google/go-containerregistry/pkg/authn"

	"github.com/anchore/stereoscope/internal/log"
)

// RegistryOptions for the OCI registry provider and containerd provider.
// If no specific Credential is found in the RegistryCredentials, will check
// for Keychain, and barring that will use Default Keychain.
type RegistryOptions struct {
	InsecureSkipTLSVerify bool
	InsecureUseHTTP       bool
	Credentials           []RegistryCredentials
	Keychain              authn.Keychain
	CAFileOrDir           string
}

type credentialSelection struct {
	credentials RegistryCredentials
	index       int
}

func (r RegistryOptions) selectMostSpecificCredentials(registry string) []credentialSelection {
	var selection []credentialSelection
	for idx, credentials := range r.Credentials {
		if !credentials.canBeUsedWithRegistry(registry) {
			continue
		}

		selection = append(selection, credentialSelection{
			credentials: credentials,
			index:       idx,
		})
	}

	sort.Slice(selection, func(i, j int) bool {
		iHasAuthority := selection[i].credentials.hasAuthoritySpecified()
		jHasAuthority := selection[j].credentials.hasAuthoritySpecified()
		if iHasAuthority && jHasAuthority {
			return selection[i].index < selection[j].index
		}
		if iHasAuthority && !jHasAuthority {
			return true
		}

		if jHasAuthority && !iHasAuthority {
			return false
		}

		return false
	})

	return selection
}

// Authenticator selects the credentials used to authenticate with a registry. Returns an authn.Authenticator
// object capable for handling high level credentials for the registry.
func (r RegistryOptions) Authenticator(registry string) authn.Authenticator {
	var authenticator authn.Authenticator
	for _, selection := range r.selectMostSpecificCredentials(registry) {
		authenticator = selection.credentials.authenticator()

		if authenticator != nil {
			log.Tracef("using registry credentials from config index %d", selection.index+1)
			break
		}
	}

	return authenticator
}

// TLSConfig selects the tls.Config object for handling TLS authentication with a registry.
func (r RegistryOptions) TLSConfig(registry string) (*tls.Config, error) {
	if r.InsecureSkipTLSVerify {
		log.Debugf("TLS verification is disabled for registry %q", registry)
	}

	tlsOptions := r.tlsOptions(registry)

	if tlsOptions == nil {
		tlsOptions = &tlsconfig.Options{
			InsecureSkipVerify: r.InsecureSkipTLSVerify,
		}
	}

	// note: tlsOptions allows for CAFile, however, this doesn't allow us to provide possibly multiple CA certs
	// to the underlying root pool. In order to support this we need to do the work to load the certs ourselves.
	tlsConfig, err := tlsconfig.Client(*tlsOptions)
	if err != nil {
		return nil, fmt.Errorf("unable to configure TLS client config: %w", err)
	}

	if !r.InsecureSkipTLSVerify && r.CAFileOrDir != "" {
		fi, err := os.Stat(r.CAFileOrDir)
		if err != nil {
			return nil, fmt.Errorf("unable to stat %q: %w", r.CAFileOrDir, err)
		}
		// load all the files in the directory as CA certs
		rootCAs := tlsConfig.RootCAs
		if rootCAs == nil {
			rootCAs, err = x509.SystemCertPool()
			if err != nil {
				log.Warnf("unable to load system cert pool: %w", err)
				rootCAs = x509.NewCertPool()
			}
		}

		var files []string
		if fi.IsDir() {
			// glob all *.crt, *.pem, and *.cert files in the directory
			var err error

			files, err = doublestar.Glob(os.DirFS("."), filepath.Join(r.CAFileOrDir, "*.{crt,pem,cert}"))
			if err != nil {
				return nil, fmt.Errorf("unable to find certs in %q: %w", r.CAFileOrDir, err)
			}
		} else {
			files = []string{r.CAFileOrDir}
		}

		for _, certFile := range files {
			log.Tracef("loading CA certificate from %q", certFile)
			pem, err := os.ReadFile(certFile)
			if err != nil {
				return nil, fmt.Errorf("could not read CA certificate %q: %v", certFile, err)
			}
			if !rootCAs.AppendCertsFromPEM(pem) {
				return nil, fmt.Errorf("failed to append certificates from PEM file: %q", certFile)
			}
		}

		tlsConfig.RootCAs = rootCAs
	}

	return tlsConfig, nil
}

// tlsOptions selects the tlsconfig.Options object for handling TLS authentication with a registry. Note: this will
// not consider the CAFileOrDir option, as that is handled by TLSConfig.
func (r RegistryOptions) tlsOptions(registry string) *tlsconfig.Options {
	var options *tlsconfig.Options
	for _, selection := range r.selectMostSpecificCredentials(registry) {
		c := selection.credentials
		if c.ClientCert != "" || c.ClientKey != "" {
			options = &tlsconfig.Options{
				InsecureSkipVerify: r.InsecureSkipTLSVerify,
				CertFile:           c.ClientCert,
				KeyFile:            c.ClientKey,
			}
		}

		if options != nil {
			log.Tracef("using custom TLS credentials from config index %d", selection.index+1)
			break
		}
	}

	if r.InsecureSkipTLSVerify && options == nil {
		options = &tlsconfig.Options{
			InsecureSkipVerify: r.InsecureSkipTLSVerify,
		}
	}

	return options
}
