// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package hooks contains the mutation hooks for the Zarf agent.
package hooks

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/internal/agent/operations"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/images"
	"github.com/zarf-dev/zarf/src/pkg/pki"
	"github.com/zarf-dev/zarf/src/pkg/state"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry"
	orasRemote "oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

func getLabelPatch(currLabels map[string]string) operations.PatchOperation {
	if currLabels == nil {
		currLabels = make(map[string]string)
	}
	currLabels["zarf-agent"] = "patched"
	return operations.ReplacePatchOperation("/metadata/labels", currLabels)
}

// getRegistryClientMTLS retrieves the mTLS cert for the registry client if available.
// Returns the cert, a boolean indicating if mTLS should be used, and any error encountered.
// FIXME: de-duplicate this
func getRegistryClientMTLS(ctx context.Context, c *cluster.Cluster) (pki.GeneratedPKI, bool, error) {
	var certs pki.GeneratedPKI
	useMTLS := false

	if c != nil {
		var err error
		certs, err = c.GetRegistryClientMTLSCert(ctx)
		if err != nil && !kerrors.IsNotFound(err) {
			return pki.GeneratedPKI{}, false, err
		}
		useMTLS = !kerrors.IsNotFound(err)
	}

	return certs, useMTLS, nil
}

// transportFromClientCert creates an HTTP transport configured with client mTLS certificates.
func transportFromClientCert(certs pki.GeneratedPKI) (http.RoundTripper, error) {
	cert, err := tls.X509KeyPair(certs.Cert, certs.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to load client certificate: %w", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(certs.CA) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
	}
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, errors.New("could not get default transport")
	}
	transport = transport.Clone()
	transport.TLSClientConfig = tlsConfig
	return transport, nil
}

func getManifestConfigMediaType(ctx context.Context, zarfState *state.State, transport http.RoundTripper, imageAddress string) (string, error) {
	ref, err := registry.ParseReference(imageAddress)
	if err != nil {
		return "", err
	}

	client := &auth.Client{
		Client: &http.Client{
			Transport: transport,
		},
		Cache: auth.NewCache(),
		Credential: auth.StaticCredential(ref.Registry, auth.Credential{
			Username: zarfState.RegistryInfo.PullUsername,
			Password: zarfState.RegistryInfo.PullPassword,
		}),
	}

	plainHTTP, err := images.ShouldUsePlainHTTP(ctx, ref.Registry, client)
	if err != nil {
		return "", err
	}

	registry := &orasRemote.Repository{
		PlainHTTP: plainHTTP,
		Reference: ref,
		Client:    client,
	}

	_, b, err := oras.FetchBytes(ctx, registry, imageAddress, oras.DefaultFetchBytesOptions)

	if err != nil {
		return "", fmt.Errorf("got an error when trying to access the manifest for %s, error %w", imageAddress, err)
	}

	var manifest ocispec.Manifest
	if err := json.Unmarshal(b, &manifest); err != nil {
		return "", fmt.Errorf("unable to unmarshal the manifest json for %s", imageAddress)
	}

	return manifest.Config.MediaType, nil
}
