// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package hooks

import (
	"context"
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/defenseunicorns/pkg/helpers/v2"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry/remote"
)

const (
	// Kubernetes’ compiled-in default if the apiserver flag
	// --service-node-port-range is not overridden.
	defaultNodePortMin = 30000
	defaultNodePortMax = 32767
	// Hard safety cap so we never spin forever if someone mis-configures a range.
	maxAttemptsFactor = 2
)

func populateLocalRegistry(ctx context.Context, t *testing.T, localURL string, artifact transform.Image, copyOpts oras.CopyOptions) {
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

func setupRegistry(ctx context.Context, t *testing.T, port int, artifacts []transform.Image, copyOpts oras.CopyOptions) (string, error) {
	localURL := testutil.SetupInMemoryRegistry(ctx, t, port)

	for _, art := range artifacts {
		populateLocalRegistry(ctx, t, localURL, art, copyOpts)
	}

	return localURL, nil
}

type mediaTypeTest struct {
	name     string
	image    string
	expected string
	artifact []transform.Image
	Opts     oras.CopyOptions
}

func TestConfigMediaTypes(t *testing.T) {
	t.Parallel()
	port, err := helpers.GetAvailablePort()
	require.NoError(t, err)

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
			image:    fmt.Sprintf("localhost:%d/stefanprodan/manifests/podinfo:6.9.0-zarf-2823281104", port),
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
			image:    fmt.Sprintf("localhost:%d/stefanprodan/charts/podinfo:6.9.0", port),
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
			//
			name:     "docker image manifest",
			expected: "application/vnd.oci.image.config.v1+json",
			image:    fmt.Sprintf("localhost:%d/zarf-dev/images/hello-world:latest", port),
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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.TestContext(t)
			url, err := setupRegistry(ctx, t, port, tt.artifact, tt.Opts)
			require.NoError(t, err)

			s := &state.State{RegistryInfo: state.RegistryInfo{Address: url}}
			mediaType, err := getManifestConfigMediaType(ctx, s, tt.image)
			require.NoError(t, err)
			require.Equal(t, tt.expected, mediaType)
		})
	}
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

	// Seed a *local* rand.Rand so concurrent callers don’t step on each other.
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
