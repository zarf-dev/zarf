// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package hooks

import (
	"context"
	crand "crypto/rand"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	orasRetry "oras.land/oras-go/v2/registry/remote/retry"
)

const (
	// Kubernetes' compiled-in default if the apiserver flag
	// --service-node-port-range is not overridden.
	defaultNodePortMin = 30000
	defaultNodePortMax = 32767
	// Hard safety cap so we never spin forever if someone misconfigures a range.
	maxAttemptsFactor = 2
)

func pushToRegistry(ctx context.Context, t *testing.T, localURL string, artifact transform.Image, copyOpts oras.CopyOptions) {
	localReg, err := remote.NewRegistry(localURL)
	require.NoError(t, err)

	localReg.PlainHTTP = true

	remoteReg, err := remote.NewRegistry(artifact.Host)
	require.NoError(t, err)

	src, err := remoteReg.Repository(ctx, artifact.Path)
	require.NoError(t, err)

	dst, err := localReg.Repository(ctx, artifact.Path)
	require.NoError(t, err)

	_, err = oras.Copy(ctx, src, artifact.Tag, dst, artifact.Tag, copyOpts)
	require.NoError(t, err)

	hashedTag, err := transform.ImageTransformHost(localURL, fmt.Sprintf("%s/%s:%s", artifact.Host, artifact.Path, artifact.Tag))
	require.NoError(t, err)

	_, err = oras.Copy(ctx, src, artifact.Tag, dst, hashedTag, copyOpts)
	require.NoError(t, err)
}

func populateRegistry(ctx context.Context, t *testing.T, registryURL string, artifacts []transform.Image, copyOpts oras.CopyOptions) {
	t.Helper()
	for _, art := range artifacts {
		pushToRegistry(ctx, t, registryURL, art, copyOpts)
	}
}

type mediaTypeTest struct {
	name     string
	relRef   string
	expected string
	artifact []transform.Image
	Opts     oras.CopyOptions
}

func TestConfigMediaTypes(t *testing.T) {
	t.Parallel()

	linuxAmd64Opts := oras.DefaultCopyOptions
	linuxAmd64Opts.WithTargetPlatform(&v1.Platform{
		Architecture: "amd64",
		OS:           "linux",
	})

	tests := []mediaTypeTest{
		{
			// https://oci.dag.dev/?image=ghcr.io%2Fstefanprodan%2Fmanifests%2Fpodinfo%3A6.9.0
			name:     "flux manifest",
			expected: "application/vnd.cncf.flux.config.v1+json",
			relRef:   "stefanprodan/manifests/podinfo:6.9.0-zarf-2823281104",
			Opts:     oras.DefaultCopyOptions,
			artifact: []transform.Image{
				{
					Host: "ghcr.io",
					Path: "stefanprodan/manifests/podinfo",
					Tag:  "6.9.0",
				},
			},
		},
		{
			// https://oci.dag.dev/?image=ghcr.io%2Fstefanprodan%2Fcharts%2Fpodinfo%3A6.9.0
			name:     "helm chart manifest",
			expected: "application/vnd.cncf.helm.config.v1+json",
			relRef:   "stefanprodan/charts/podinfo:6.9.0",
			Opts:     oras.DefaultCopyOptions,
			artifact: []transform.Image{
				{
					Host: "ghcr.io",
					Path: "stefanprodan/charts/podinfo",
					Tag:  "6.9.0",
				},
			},
		},
		{
			name:     "docker image manifest",
			expected: "application/vnd.oci.image.config.v1+json",
			relRef:   "zarf-dev/images/hello-world:latest",
			Opts:     linuxAmd64Opts,
			artifact: []transform.Image{
				{
					Host: "ghcr.io",
					Path: "zarf-dev/images/hello-world",
					Tag:  "latest",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.TestContext(t)
			url := testutil.SetupInMemoryRegistryDynamic(ctx, t)
			populateRegistry(ctx, t, url, tt.artifact, tt.Opts)

			s := &state.State{RegistryInfo: state.RegistryInfo{Address: url}}
			mediaType, err := getManifestConfigMediaType(ctx, s, orasRetry.DefaultClient.Transport, fmt.Sprintf("%s/%s", url, tt.relRef))
			require.NoError(t, err)
			require.Equal(t, tt.expected, mediaType)
		})
	}
}

func TestGetManifestConfigMediaType_FailsWhenRegistryBecomesUnreachable(t *testing.T) {
	ctx := testutil.TestContext(t)
	url, stop := testutil.SetupInMemoryRegistryStoppable(ctx, t)

	// A minimal local manifest is enough here: this test is about surfacing a
	// sensible error when the registry disappears, not about resolving a real
	// image, so it doesn't need a real registry pull.
	repo := testutil.NewRepo(t, url+"/fixtures/agent")
	config := testutil.PushBlob(ctx, t, repo, v1.MediaTypeImageConfig, []byte(`{"architecture":"amd64"}`))
	manifest := testutil.PushManifest(ctx, t, repo, config, nil)
	require.NoError(t, repo.Tag(ctx, manifest, "v1"))

	s := &state.State{RegistryInfo: state.RegistryInfo{Address: url}}
	ref := fmt.Sprintf("%s/fixtures/agent:v1", url)

	_, err := getManifestConfigMediaType(ctx, s, orasRetry.DefaultClient.Transport, ref)
	require.NoError(t, err)

	// The registry becomes completely unreachable.
	stop()

	// Each call negotiates fresh -- there is no cache to go stale -- so this fails
	// at the negotiation step itself, before ever attempting the manifest fetch, and
	// surfaces the negotiator's own error rather than the fetch-path wrapper.
	_, err = getManifestConfigMediaType(ctx, s, orasRetry.DefaultClient.Transport, ref)
	require.Error(t, err)
	require.Contains(t, err.Error(), "refusing to downgrade to plain HTTP")
}

func TestGetManifestConfigMediaType_RecoversWhenRegistrySchemeChanges(t *testing.T) {
	ctx := testutil.TestContext(t)
	url, stop := testutil.SetupInMemoryRegistryStoppable(ctx, t)
	_, portStr, err := net.SplitHostPort(url)
	require.NoError(t, err)
	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)

	// Registry, phase 1: plain HTTP. Negotiate and cache plainHTTP=true for this host.
	repoA := testutil.NewRepo(t, url+"/fixtures/agent")
	configA := testutil.PushBlob(ctx, t, repoA, v1.MediaTypeImageConfig, []byte(`{"architecture":"amd64"}`))
	manifestA := testutil.PushManifest(ctx, t, repoA, configA, nil)
	require.NoError(t, repoA.Tag(ctx, manifestA, "v1"))

	s := &state.State{RegistryInfo: state.RegistryInfo{Address: url}}
	ref := fmt.Sprintf("%s/fixtures/agent:v1", url)

	// A transport that accepts the self-signed cert used in phase 2 below; also
	// used as-is for phase 1, since InsecureSkipTLSVerify has no effect over plain
	// HTTP.
	transport := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} //nolint:gosec // test-only, talking to our own self-signed registry

	mediaType, err := getManifestConfigMediaType(ctx, s, transport, ref)
	require.NoError(t, err)
	require.Equal(t, v1.MediaTypeImageConfig, mediaType)

	// The registry migrates from plain HTTP to HTTPS on the exact same address --
	// stop the plain-HTTP instance and start a TLS one on the same port.
	stop()
	certFile, keyFile := testutil.SelfSignedCert(t, "127.0.0.1")
	testutil.SetupInMemoryRegistryTLSOnPort(ctx, t, port, certFile, keyFile)

	repoB, err := remote.NewRepository(url + "/fixtures/agent")
	require.NoError(t, err)
	repoB.Client = &auth.Client{Client: &http.Client{Transport: transport}}
	configB := testutil.PushBlob(ctx, t, repoB, v1.MediaTypeImageConfig, []byte(`{"architecture":"amd64"}`))
	manifestB := testutil.PushManifest(ctx, t, repoB, configB, nil)
	require.NoError(t, repoB.Tag(ctx, manifestB, "v1"))

	// The registry now speaks HTTPS instead of plain HTTP. getManifestConfigMediaType
	// negotiates fresh on every call, so this call's own initial probe picks up the
	// new scheme directly -- not just failing sanely, but actually succeeding
	// against the corrected scheme.
	mediaType, err = getManifestConfigMediaType(ctx, s, transport, ref)
	require.NoError(t, err)
	require.Equal(t, v1.MediaTypeImageConfig, mediaType)
}

// GetAvailableNodePort returns a free TCP port that falls within the current
// NodePort range.
//
// The range is discovered in this order:
//  1. The env var SERVICE_NODE_PORT_RANGE (format "min-max") – matches the
//     kube-apiserver flag name & format.
//  2. The Kubernetes default range 30000-32767.
//
// The function randomly probes ports in that range until it finds one the OS
// will allow us to bind.  If every port in the range is in use it returns an
// error.
func GetAvailableNodePort() (int, error) {
	minPort, maxPort, err := nodePortRange()
	if err != nil {
		return 0, err
	}

	// Seed a *local* rand.Rand so concurrent callers don't step on each other.
	seed := int64(binary.LittleEndian.Uint64(random64()))
	r := rand.New(rand.NewSource(seed))

	size := maxPort - minPort + 1
	maxAttempts := size * maxAttemptsFactor // statistically enough even on busy hosts

	for i := 0; i < maxAttempts; i++ {
		port := r.Intn(size) + minPort
		l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			continue // busy; try another candidate
		}
		_ = l.Close() //nolint: errcheck
		return port, nil
	}
	return 0, fmt.Errorf("unable to find a free NodePort in range %d-%d after %d attempts", minPort, maxPort, maxAttempts)
}

// nodePortRange resolves the active NodePort range.
func nodePortRange() (int, int, error) {
	if v := os.Getenv("SERVICE_NODE_PORT_RANGE"); v != "" {
		parts := strings.SplitN(strings.TrimSpace(v), "-", 2)
		if len(parts) == 2 {
			minPort, err1 := strconv.Atoi(parts[0])
			maxPort, err2 := strconv.Atoi(parts[1])
			if err1 == nil && err2 == nil && minPort > 0 && maxPort >= minPort {
				return minPort, maxPort, nil
			}
		}
		return 0, 0, fmt.Errorf("invalid SERVICE_NODE_PORT_RANGE value %q (expected \"min-max\")", v)
	}
	return defaultNodePortMin, defaultNodePortMax, nil
}

// random64 returns 8 cryptographically-secure random bytes.  We fall back to
// time.Now if /dev/urandom becomes unavailable (extremely rare).
func random64() []byte {
	var b [8]byte
	if _, err := crand.Read(b[:]); err != nil {
		binary.LittleEndian.PutUint64(b[:], uint64(time.Now().UnixNano()))
	}
	return b[:]
}
