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

	"github.com/gpustack/gguf-parser-go/util/ptr"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	cmdwait "k8s.io/kubectl/pkg/cmd/wait"
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

	// Fill these out in the Retry loop, which handles the cluster not yet being available
	var restConfig *rest.Config
	var configFlags *genericclioptions.ConfigFlags
	var resInfo resourceInfo
	deadline := time.Now().Add(timeout)
	waitInterval := time.Second
	for {
		configFlags = genericclioptions.NewConfigFlags(true)
		if namespace != "" {
			configFlags.Namespace = ptr.To(namespace)
		}

		var err error
		restConfig, err = configFlags.ToRESTConfig()
		if err != nil {
			return fmt.Errorf("failed to get REST config: %w", err)
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("timed out waiting for %s", kind)
		}

		resInfo, err = resolveResourceKind(restConfig, kind)
		if err == nil {
			break
		}

		l.Debug("failed to resolve resource kind, retrying", "kind", kind, "error", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitInterval):
			continue
		}
	}

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// If no identifier specified, wait for any resource of this kind to exist
	if identifier == "" {
		return waitForAnyResource(ctx, dynamicClient, resInfo, namespace, deadline)
	}

	// Calculate remaining time for the resource wait
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return fmt.Errorf("timed out waiting for %s/%s", kind, identifier)
	}

	return forResource(ctx, configFlags, dynamicClient, condition, resInfo.name, identifier, namespace, remaining)
}

// waitForAnyResource waits for at least one resource of the given kind to exist.
func waitForAnyResource(ctx context.Context, dynamicClient dynamic.Interface, resInfo resourceInfo, namespace string, deadline time.Time) error {
	l := logger.From(ctx)
	waitInterval := time.Second
	l.Info("waiting for any resource", "kind", resInfo.name, "namespace", namespace)

	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("timed out waiting for any %s", resInfo.name)
		}

		var resourceClient dynamic.ResourceInterface
		// FIXME: do I really need to check namespace
		if resInfo.namespaced && namespace != "" {
			resourceClient = dynamicClient.Resource(resInfo.gvr).Namespace(namespace)
		} else {
			resourceClient = dynamicClient.Resource(resInfo.gvr)
		}

		list, err := resourceClient.List(ctx, metav1.ListOptions{Limit: 1})
		if err == nil && len(list.Items) > 0 {
			fmt.Println("item found was", list.Items[0].GetName())
			l.Info("found resource", "kind", resInfo.name, "namespace", namespace)
			return nil
		}
		if err != nil {
			l.Debug("error listing resources", "error", err)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitInterval):
			continue
		}
	}
}

type resourceInfo struct {
	name       string
	gvr        schema.GroupVersionResource
	namespaced bool
}

// FIXME: I need to check if something like deployment.apps/v1beta1 works
// resolveResourceKind searches all API groups to find the canonical resource name for user input.
// This handles aliases like "po" -> "pods", "svc" -> "services", "sc" -> "storageclasses".
func resolveResourceKind(restConfig *rest.Config, givenKind string) (resourceInfo, error) {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return resourceInfo{}, fmt.Errorf("failed to create discovery client: %w", err)
	}

	// Get all API resources from all groups
	_, resourceLists, err := discoveryClient.ServerGroupsAndResources()
	if err != nil {
		return resourceInfo{}, fmt.Errorf("failed to get resource list: %w", err)
	}

	userInputLower := strings.ToLower(givenKind)

	for _, resourceList := range resourceLists {
		if resourceList == nil {
			continue
		}
		gv, err := schema.ParseGroupVersion(resourceList.GroupVersion)
		if err != nil {
			continue
		}
		for _, resource := range resourceList.APIResources {
			// Skip subresources (they contain "/"), for instance pods/status
			if strings.Contains(resource.Name, "/") {
				continue
			}
			// Match against: plural name, singular name, kind, or short names
			if strings.ToLower(resource.Name) == userInputLower ||
				strings.ToLower(resource.SingularName) == userInputLower ||
				strings.ToLower(resource.Kind) == userInputLower ||
				containsIgnoreCase(resource.ShortNames, givenKind) {
				return resourceInfo{
					name: resource.Name,
					gvr: schema.GroupVersionResource{
						Group:    gv.Group,
						Version:  gv.Version,
						Resource: resource.Name,
					},
					namespaced: resource.Namespaced,
				}, nil
			}
		}
	}

	return resourceInfo{}, fmt.Errorf("failed to find kind %s in cluster", givenKind)
}

func isJSONPathWaitType(condition string) bool {
	if len(condition) == 0 || condition[0] != '{' || !strings.Contains(condition, "=") || !strings.Contains(condition, "}") {
		return false
	}
	return true
}

// containsIgnoreCase checks if a slice contains a string (case-insensitive).
func containsIgnoreCase(slice []string, str string) bool {
	strLower := strings.ToLower(str)
	for _, s := range slice {
		if strings.ToLower(s) == strLower {
			return true
		}
	}
	return false
}

// forResource is the internal implementation that can be tested with fake clients.
func forResource(ctx context.Context, configFlags *genericclioptions.ConfigFlags, dynamicClient dynamic.Interface, condition, kind, identifier, namespace string, timeout time.Duration) error {
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
	if condition != "" && !strings.EqualFold(condition, "exist") && !strings.EqualFold(condition, "exists") {
		if isJSONPathWaitType(condition) {
			forCondition = fmt.Sprintf("jsonpath=%s", condition)
		} else {
			forCondition = fmt.Sprintf("condition=%s", condition)
		}
	}

	l.Info("waiting for resource", "kind", kind, "identifier", identifier, "condition", condition, "namespace", namespace)

	waitInterval := time.Second
	deadline := time.Now().Add(timeout)

	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("timed out waiting for %s/%s", kind, identifier)
		}

		streams := genericiooptions.IOStreams{
			In:     strings.NewReader(""),
			Out:    io.Discard,
			ErrOut: io.Discard,
		}
		flags := cmdwait.NewWaitFlags(configFlags, streams)
		flags.Timeout = timeout
		flags.ForCondition = forCondition
		if labelSelector != "" {
			flags.ResourceBuilderFlags.LabelSelector = &labelSelector
		}

		opts, err := flags.ToOptions(args)
		if err != nil {
			return fmt.Errorf("failed to create wait options: %w", err)
		}
		opts.DynamicClient = dynamicClient

		err = opts.RunWait()
		if err == nil {
			l.Info("wait-for condition met", "kind", kind, "identifier", identifier, "condition", condition, "namespace", namespace)
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitInterval):
			l.Debug("retrying wait", "err", err)
			continue
		}
	}
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
					return errors.New("http status code %s is unknown")
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
