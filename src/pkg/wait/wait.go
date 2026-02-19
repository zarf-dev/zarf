// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package wait provides functions for waiting on Kubernetes resources and network endpoints.
package wait

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	cmdwait "k8s.io/kubectl/pkg/cmd/wait"
	"k8s.io/utils/ptr"
)

// ForResource waits for a Kubernetes resource to meet the specified condition.
// It uses the same logic as `kubectl wait`, with retry logic for resources that don't exist yet.
// If identifier is empty, it will wait for any resource of the given kind to exist.
// This function retries on cluster connection errors, allowing it to wait for a cluster to become available.
func ForResource(ctx context.Context, kind, identifier, condition, namespace string, timeout time.Duration) error {
	l := logger.From(ctx)
	if kind == "" {
		return errors.New("kind is required")
	}

	waitInterval := time.Second
	deadline := time.Now().Add(timeout)

	condition = strings.ReplaceAll(condition, "'", "")

	// Wait for the cluster to become available by polling for a successful REST config.
	var restConfig *rest.Config
	var clientCfg clientcmd.ClientConfig
	var discoveryClient *discovery.DiscoveryClient
	err := wait.PollUntilContextTimeout(ctx, waitInterval, timeout, true, func(_ context.Context) (bool, error) {
		var err error
		loader := clientcmd.NewDefaultClientConfigLoadingRules()
		clientCfg = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loader, nil)
		_, restConfig, err = cluster.ClientAndConfig()
		if err != nil {
			l.Debug("failed to get REST config, retrying", "error", err)
			return false, nil
		}
		discoveryClient, err = discovery.NewDiscoveryClientForConfig(restConfig)
		if err != nil {
			l.Debug("failed to get discovery client, retrying", "error", err)
			return false, nil
		}
		_, err = discoveryClient.ServerVersion()
		if err != nil {
			l.Debug("cluster not reachable, retrying", "error", err)
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("timed out waiting for REST config: %w", err)
	}

	// Wait for the resource kind to be resolvable (e.g. CRDs may not be registered yet).
	var mapping *meta.RESTMapping
	err = wait.PollUntilContextTimeout(ctx, waitInterval, time.Until(deadline), true, func(_ context.Context) (bool, error) {
		groupResources, err := restmapper.GetAPIGroupResources(discoveryClient)
		if err != nil {
			return true, fmt.Errorf("failed to get API group resources: %w", err)
		}
		restMapper := restmapper.NewShortcutExpander(restmapper.NewDiscoveryRESTMapper(groupResources), discoveryClient, nil)
		mapping, err = resolveResourceKind(restMapper, kind)
		if err != nil {
			l.Debug("failed to resolve resource kind, retrying", "kind", kind, "error", err)
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("timed out waiting to resolve resource kind %q: %w", kind, err)
	}

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	if namespace == "" && mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		ns, _, err := clientCfg.Namespace()
		if err != nil {
			return fmt.Errorf("failed to get users' default namespace: %w", err)
		}
		namespace = ns
	}
	// If no identifier specified, wait for any resource of this kind to exist
	if identifier == "" {
		return waitForAnyResource(ctx, dynamicClient, mapping.Resource, namespace, deadline)
	}

	return forResource(ctx, dynamicClient, condition, mapping.Resource.Resource, identifier, namespace, deadline)
}

// waitForAnyResource waits for at least one resource of the given kind to exist.
func waitForAnyResource(ctx context.Context, dynamicClient dynamic.Interface, resource schema.GroupVersionResource, namespace string, deadline time.Time) error {
	l := logger.From(ctx)
	waitInterval := time.Second
	l.Info("waiting for any resource of kind to exist", "kind", resource.Resource, "namespace", namespace)

	var resourceClient dynamic.ResourceInterface
	resourceClient = dynamicClient.Resource(resource)
	if namespace != "" {
		resourceClient = dynamicClient.Resource(resource).Namespace(namespace)
	}
	err := wait.PollUntilContextTimeout(ctx, waitInterval, time.Until(deadline), true, func(ctx context.Context) (bool, error) {
		list, err := resourceClient.List(ctx, metav1.ListOptions{Limit: 1})
		if err != nil {
			l.Debug("error listing resources", "error", err)
			return false, nil
		}
		if len(list.Items) > 0 {
			return true, nil
		}
		l.Debug("retrying wait for any resource of kind", "kind", resource.Resource, "namespace", namespace)
		return false, nil
	})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("timed out waiting for resource of kind %s", resource)
		}
		return err
	}
	l.Info("found resource", "kind", resource.Resource, "namespace", namespace)
	return nil
}

// resolveResourceKind resolves user input (like "pods", "po", "deployments.v1.apps") to a
// canonical resource mapping. This follows the same approach as kubectl wait's mappingFor function
// and the code here was taken directly from https://github.com/kubernetes/kubernetes/blob/eba75de1565852be1b1f27c811d1b44527b266e5/staging/src/k8s.io/cli-runtime/pkg/resource/builder.go#L772
func resolveResourceKind(restMapper meta.RESTMapper, resourceOrKindArg string) (*meta.RESTMapping, error) {
	fullySpecifiedGVR, groupResource := schema.ParseResourceArg(resourceOrKindArg)
	gvk := schema.GroupVersionKind{}

	if fullySpecifiedGVR != nil {
		gvk, _ = restMapper.KindFor(*fullySpecifiedGVR) //nolint:errcheck // mirrors k8s.io/cli-runtime/pkg/resource/builder.go mappingFor
	}
	if gvk.Empty() {
		gvk, _ = restMapper.KindFor(groupResource.WithVersion("")) //nolint:errcheck // mirrors k8s.io/cli-runtime/pkg/resource/builder.go mappingFor
	}

	if !gvk.Empty() {
		return restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	}

	fullySpecifiedGVK, groupKind := schema.ParseKindArg(resourceOrKindArg)
	if fullySpecifiedGVK == nil {
		gvk := groupKind.WithVersion("")
		fullySpecifiedGVK = &gvk
	}

	if !fullySpecifiedGVK.Empty() {
		if mapping, err := restMapper.RESTMapping(fullySpecifiedGVK.GroupKind(), fullySpecifiedGVK.Version); err == nil {
			return mapping, nil
		}
	}

	mapping, err := restMapper.RESTMapping(groupKind, gvk.Version)
	if err != nil {
		if meta.IsNoMatchError(err) {
			return nil, fmt.Errorf("the server doesn't have a resource type %q", groupResource.Resource)
		}
		return nil, err
	}

	return mapping, nil
}

func isJSONPathWaitType(condition string) bool {
	return len(condition) != 0 && condition[0] == '{' && strings.Contains(condition, "=") && strings.Contains(condition, "}")
}

func isExistsCondition(condition string) bool {
	reservedConditions := []string{"create", "exist", "exists"}
	for _, rc := range reservedConditions {
		if strings.EqualFold(condition, rc) {
			return true
		}
	}
	return false
}

func forResource(ctx context.Context, dynamicClient dynamic.Interface, condition, kind, identifier, namespace string, deadline time.Time) error {
	l := logger.From(ctx)
	var args []string
	var labelSelector string
	if strings.ContainsRune(identifier, '=') {
		args = []string{kind}
		labelSelector = identifier
	} else {
		args = []string{fmt.Sprintf("%s/%s", kind, identifier)}
	}

	forCondition := "create" // default: wait for existence
	if condition != "" && condition != "delete" && !isExistsCondition(condition) {
		if isJSONPathWaitType(condition) {
			forCondition = fmt.Sprintf("jsonpath=%s", condition)
		} else {
			forCondition = fmt.Sprintf("condition=%s", condition)
		}
	}

	if condition == "delete" {
		forCondition = "delete"
	}

	l.Info("waiting for resource", "kind", kind, "identifier", identifier, "condition", forCondition, "namespace", namespace)

	configFlags := genericclioptions.NewConfigFlags(true)
	if namespace != "" {
		configFlags.Namespace = ptr.To(namespace)
	}
	streams := genericiooptions.IOStreams{
		In:     strings.NewReader(""),
		Out:    io.Discard,
		ErrOut: io.Discard,
	}
	flags := cmdwait.NewWaitFlags(configFlags, streams)
	flags.ForCondition = forCondition
	if labelSelector != "" {
		flags.ResourceBuilderFlags.LabelSelector = &labelSelector
	}

	opts, err := flags.ToOptions(args)
	if err != nil {
		return fmt.Errorf("failed to create wait options: %w", err)
	}
	opts.DynamicClient = dynamicClient

	waitInterval := time.Second
	// Give a smaller timeout, so that we can occasionally check context, given that opts.RunWait does not accept context
	flags.Timeout = time.Second * 10
	// We wrap opts.RunWait here because it errors immediately when waiting for a condition of a resource that does not yet exist
	err = wait.PollUntilContextTimeout(ctx, waitInterval, time.Until(deadline), true, func(_ context.Context) (bool, error) {
		err = opts.RunWait()
		if err == nil {
			return true, nil
		}
		l.Debug("retrying wait", "err", err)
		return false, nil
	})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("timed out waiting for %s/%s to be %s", kind, identifier, forCondition)
		}
		return err
	}
	l.Info("wait-for condition met", "kind", kind, "identifier", identifier, "condition", forCondition, "namespace", namespace)
	return nil
}

// ForNetwork waits for a network endpoint to respond.
func ForNetwork(ctx context.Context, protocol, address, condition string, timeout time.Duration) error {
	waitInterval := time.Second
	return forNetwork(ctx, protocol, address, condition, timeout, waitInterval)
}

func forNetwork(ctx context.Context, protocol string, address string, condition string, timeout time.Duration, waitInterval time.Duration) error {
	l := logger.From(ctx)
	expired := time.After(timeout)

	condition = strings.ToLower(condition)
	if condition == "" {
		condition = "success"
	}

	// Create an HTTP client with a per-request timeout that is slightly shorter than our wait-interval to prevent
	// hanging on slow or unresponsive servers.
	httpClient := &http.Client{
		Timeout: waitInterval - (time.Millisecond * 5),
	}

	delay := 100 * time.Millisecond

	for {
		// Delay the check for 100ms the first time and then the wait interval after that
		time.Sleep(delay)
		delay = waitInterval

		select {
		case <-expired:
			return errors.New("wait timed out")
		case <-ctx.Done():
			return errors.New("received interrupt")
		default:
			switch protocol {
			case "http", "https":
				// Handle HTTP and HTTPS endpoints.
				url := fmt.Sprintf("%s://%s", protocol, address)

				// Default to checking for a 2xx response.
				if condition == "success" {
					// Try to get the URL and check the status code.
					resp, err := httpClient.Get(url)
					if err != nil {
						l.Debug(err.Error())
						continue
					}
					err = resp.Body.Close()
					if err != nil {
						l.Debug(err.Error())
					}

					// If the status code is not in the 2xx range, try again.
					if resp.StatusCode < 200 || resp.StatusCode > 299 {
						l.Debug("did not receive 2xx status code", "response_code", resp.StatusCode)
						continue
					}

					// Success, break out of the switch statement.
					break
				}

				// Convert the condition to an int and check if it's a valid HTTP status code.
				code, err := strconv.Atoi(condition)
				if err != nil {
					return fmt.Errorf("http status code %s is not an integer: %w", condition, err)
				}
				if http.StatusText(code) == "" {
					return errors.New("http status code is unknown")
				}

				// Try to get the URL and check the status code.
				resp, err := httpClient.Get(url)
				if err != nil {
					l.Debug(err.Error())
					continue
				}
				err = resp.Body.Close()
				if err != nil {
					l.Debug(err.Error())
				}

				if resp.StatusCode != code {
					l.Debug("did not receive expected status code", "expected", code, "actual", resp.StatusCode)
					continue
				}
			default:
				// Fallback to any generic protocol using net.Dial
				conn, err := net.Dial(protocol, address)
				if err != nil {
					l.Debug(err.Error())
					continue
				}
				err = conn.Close()
				if err != nil {
					l.Debug(err.Error())
					continue
				}
			}

			// Yay, we made it!
			return nil
		}
	}
}
