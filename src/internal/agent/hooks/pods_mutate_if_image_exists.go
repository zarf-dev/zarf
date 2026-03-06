// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package hooks

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/zarf-dev/zarf/src/internal/agent/operations"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/pki"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// checkNamespaceMutationBehavior checks the namespace labels to determine mutation behavior.
// Returns useMutateIfExists flag and skipResult. If skipResult is non-nil, mutation should be skipped.
func checkNamespaceMutationBehavior(ctx context.Context, c *cluster.Cluster, namespaceName string) (useMutateIfExists bool, skipResult *operations.Result) {
	l := logger.From(ctx)

	// Fetch namespace to check labels
	namespace, err := c.Clientset.CoreV1().Namespaces().Get(ctx, namespaceName, metav1.GetOptions{})
	if err != nil {
		l.Warn("failed to get namespace labels, using legacy behavior", "namespace", namespaceName, "error", err.Error())
		return false, nil
	}

	// Check if namespace should be completely skipped
	if namespace.Labels != nil {
		agentLabel := namespace.Labels["zarf.dev/agent"]
		if agentLabel == "skip" || agentLabel == "ignore" {
			l.Info("skipping pod mutation for ignored namespace", "namespace", namespaceName, "label", agentLabel)
			return false, &operations.Result{
				Allowed:  true,
				PatchOps: []operations.PatchOperation{},
			}
		}

		// Check if this namespace should use mutate-if-exists behavior
		if strings.EqualFold(agentLabel, "mutate-if-exists") {
			l.Info("namespace opted into mutate-if-exists behavior", "namespace", namespaceName)
			return true, nil
		}
	}

	return false, nil
}

// imageExistsInRegistry checks if an image exists in the registry using HTTP HEAD request.
func imageExistsInRegistry(ctx context.Context, c *cluster.Cluster, imageRef string, registryInfo *state.RegistryInfo) (bool, error) {
	l := logger.From(ctx)

	// Parse the image reference
	image, err := transform.ParseImageRef(imageRef)
	if err != nil {
		return false, fmt.Errorf("failed to parse image reference: %w", err)
	}

	// Build the manifest API URL
	var reference string
	if image.Digest != "" {
		reference = image.Digest
	} else {
		reference = image.Tag
	}

	// Use in-cluster service address when accessing the internal registry
	registryAddress := registryInfo.Address
	if registryInfo.IsInternal() {
		// Use the in-cluster service DNS name and port
		registryAddress = "zarf-docker-registry.zarf.svc.cluster.local:5000"
	}

	manifestURL := fmt.Sprintf("http://%s/v2/%s/manifests/%s", registryAddress, image.Path, reference)

	// Create HTTP client with mTLS if needed
	var transport = http.DefaultTransport
	if registryInfo.ShouldUseMTLS() {
		certs, certErr := c.GetRegistryClientMTLSCert(ctx)
		if certErr != nil {
			l.Warn("failed to get registry mTLS cert, using default transport", "error", certErr.Error())
		} else {
			mtlsTransport, transportErr := pki.TransportWithKey(certs)
			if transportErr != nil {
				l.Warn("failed to create mTLS transport, using default transport", "error", transportErr.Error())
			} else {
				transport = mtlsTransport
			}
		}
	}

	httpClient := &http.Client{Timeout: 10 * time.Second, Transport: transport}

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, manifestURL, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	// Set Accept headers for Docker and OCI manifest types
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json, application/vnd.oci.image.manifest.v1+json")

	// Add Basic Auth if credentials are provided
	if registryInfo.PullUsername != "" && registryInfo.PullPassword != "" {
		auth := registryInfo.PullUsername + ":" + registryInfo.PullPassword
		encodedAuth := base64.StdEncoding.EncodeToString([]byte(auth))
		req.Header.Set("Authorization", "Basic "+encodedAuth)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to check manifest: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			l.Warn("failed to close response body", "error", closeErr.Error())
		}
	}()

	switch resp.StatusCode {
	case http.StatusOK:
		l.Debug("image exists in registry", "image", imageRef, "url", manifestURL)
		return true, nil
	case http.StatusNotFound:
		l.Debug("image not found in registry", "image", imageRef, "url", manifestURL)
		return false, nil
	default:
		return false, fmt.Errorf("unexpected status code %d checking manifest for %s", resp.StatusCode, imageRef)
	}
}
