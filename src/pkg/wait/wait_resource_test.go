// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package wait

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
)

func newUnstructured(apiVersion, kind, namespace, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]any{
				"namespace":       namespace,
				"name":            name,
				"uid":             "test-uid",
				"resourceVersion": "1",
			},
		},
	}
}

func addCondition(t *testing.T, in *unstructured.Unstructured, condType, status string) *unstructured.Unstructured {
	t.Helper()
	conditions, _, err := unstructured.NestedSlice(in.Object, "status", "conditions")
	require.NoError(t, err)
	conditions = append(conditions, map[string]any{
		"type":   condType,
		"status": status,
	})
	err = unstructured.SetNestedSlice(in.Object, conditions, "status", "conditions")
	require.NoError(t, err)
	return in
}

func setNestedField(t *testing.T, in *unstructured.Unstructured, value any, fields ...string) *unstructured.Unstructured {
	t.Helper()
	err := unstructured.SetNestedField(in.Object, value, fields...)
	require.NoError(t, err)
	return in
}

// mustEncode encodes v to w, panicking on error (acceptable in test HTTP handlers)
func mustEncode(w http.ResponseWriter, v any) {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		panic(err)
	}
}

// fakeAPIServer creates a test HTTP server that mimics Kubernetes API responses
func fakeAPIServer(resources map[string]*unstructured.Unstructured) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Handle API group list (used by discovery)
		if r.URL.Path == "/apis" {
			resp := &metav1.APIGroupList{
				TypeMeta: metav1.TypeMeta{Kind: "APIGroupList", APIVersion: "v1"},
				Groups:   []metav1.APIGroup{},
			}
			mustEncode(w, resp)
			return
		}

		// Handle API discovery root
		if r.URL.Path == "/api" {
			resp := &metav1.APIVersions{
				TypeMeta: metav1.TypeMeta{Kind: "APIVersions"},
				Versions: []string{"v1"},
			}
			mustEncode(w, resp)
			return
		}

		// Handle API discovery for core v1
		if r.URL.Path == "/api/v1" {
			resp := &metav1.APIResourceList{
				TypeMeta:     metav1.TypeMeta{Kind: "APIResourceList", APIVersion: "v1"},
				GroupVersion: "v1",
				APIResources: []metav1.APIResource{
					{Name: "pods", Namespaced: true, Kind: "Pod", Verbs: metav1.Verbs{"get", "list", "watch"}},
				},
			}
			mustEncode(w, resp)
			return
		}

		// Handle list/watch requests for pods
		if r.URL.Path == "/api/v1/namespaces/default/pods" {
			// Parse fieldSelector to filter by name if present
			fieldSelector := r.URL.Query().Get("fieldSelector")
			nameFilter, _ := strings.CutPrefix(fieldSelector, "metadata.name=")

			// Check if this is a watch request
			if r.URL.Query().Get("watch") == "true" {
				flusher, ok := w.(http.Flusher)
				if !ok {
					http.Error(w, "streaming not supported", http.StatusInternalServerError)
					return
				}

				// Send ADDED events for matching resources (filtered by name if specified)
				for path, resource := range resources {
					if !strings.HasPrefix(path, "/api/v1/namespaces/default/pods/") {
						continue
					}
					// Apply name filter if present
					if nameFilter != "" {
						resourceName, _, err := unstructured.NestedString(resource.Object, "metadata", "name")
						if err != nil {
							http.Error(w, err.Error(), http.StatusInternalServerError)
							return
						}
						if resourceName != nameFilter {
							continue
						}
					}
					event := map[string]any{
						"type":   "ADDED",
						"object": resource.Object,
					}
					mustEncode(w, event)
					flusher.Flush()
				}

				// Send BOOKMARK to indicate initial events are complete (required for sendInitialEvents)
				if r.URL.Query().Get("sendInitialEvents") == "true" {
					bookmark := map[string]any{
						"type": "BOOKMARK",
						"object": map[string]any{
							"apiVersion": "v1",
							"kind":       "Pod",
							"metadata": map[string]any{
								"resourceVersion": "1",
								"annotations": map[string]any{
									"k8s.io/initial-events-end": "true",
								},
							},
						},
					}
					mustEncode(w, bookmark)
					flusher.Flush()
				}
				// Keep connection open until client disconnects or timeout
				// The client will close when condition is met
				<-r.Context().Done()
				return
			}

			// Regular list request - collect matching pod resources into a list
			items := []any{}
			for path, resource := range resources {
				if !strings.HasPrefix(path, "/api/v1/namespaces/default/pods/") {
					continue
				}
				// Apply name filter if present
				if nameFilter != "" {
					resourceName, _, err := unstructured.NestedString(resource.Object, "metadata", "name")
					if err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					if resourceName != nameFilter {
						continue
					}
				}
				items = append(items, resource.Object)
			}
			resp := map[string]any{
				"apiVersion": "v1",
				"kind":       "PodList",
				"metadata":   map[string]any{"resourceVersion": "1"},
				"items":      items,
			}
			mustEncode(w, resp)
			return
		}

		// Handle resource requests
		for path, resource := range resources {
			if r.URL.Path == path {
				mustEncode(w, resource)
				return
			}
		}

		// Not found
		w.WriteHeader(http.StatusNotFound)
		resp := &metav1.Status{
			TypeMeta: metav1.TypeMeta{Kind: "Status", APIVersion: "v1"},
			Status:   "Failure",
			Message:  "not found",
			Code:     http.StatusNotFound,
		}
		mustEncode(w, resp)
	}))
}

// newTestConfigFlagsWithServer creates ConfigFlags pointing to a test server
func newTestConfigFlagsWithServer(server *httptest.Server, namespace string) *genericclioptions.ConfigFlags {
	configFlags := genericclioptions.NewConfigFlags(false)
	configFlags.APIServer = ptr.To(server.URL)
	configFlags.Insecure = ptr.To(true)
	if namespace != "" {
		configFlags.Namespace = ptr.To(namespace)
	}
	return configFlags
}

// TestForResourceSimplePodWait tests waiting for a pod to exist (the default "create" condition)
func TestForResourceSimplePodWait(t *testing.T) {
	t.Parallel()

	pod := newUnstructured("v1", "Pod", "default", "my-pod")
	server := fakeAPIServer(map[string]*unstructured.Unstructured{
		"/api/v1/namespaces/default/pods/my-pod": pod,
	})
	defer server.Close()

	configFlags := newTestConfigFlagsWithServer(server, "default")
	restConfig, err := configFlags.ToRESTConfig()
	require.NoError(t, err)
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	require.NoError(t, err)

	ctx := context.Background()

	// Test with empty condition (should default to "create" which waits for existence)
	err = forResource(ctx, configFlags, dynamicClient, "", "pods", "my-pod", 5*time.Second)
	require.NoError(t, err)
}

// TestForResourceExistsCondition tests that "exists" condition maps to "create"
func TestForResourceExistsCondition(t *testing.T) {
	t.Parallel()

	pod := newUnstructured("v1", "Pod", "default", "my-pod")
	server := fakeAPIServer(map[string]*unstructured.Unstructured{
		"/api/v1/namespaces/default/pods/my-pod": pod,
	})
	defer server.Close()

	configFlags := newTestConfigFlagsWithServer(server, "default")
	restConfig, err := configFlags.ToRESTConfig()
	require.NoError(t, err)
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	require.NoError(t, err)

	ctx := context.Background()

	// Test with "exists" condition - should work the same as empty condition
	err = forResource(ctx, configFlags, dynamicClient, "exists", "pods", "my-pod", 5*time.Second)
	require.NoError(t, err)

	// Test with "exist" condition (singular)
	err = forResource(ctx, configFlags, dynamicClient, "exist", "pods", "my-pod", 5*time.Second)
	require.NoError(t, err)

	// Test case insensitivity
	err = forResource(ctx, configFlags, dynamicClient, "EXISTS", "pods", "my-pod", 5*time.Second)
	require.NoError(t, err)
}

// TestForResourceRegularCondition tests waiting for a standard condition like "Ready"
func TestForResourceRegularCondition(t *testing.T) {
	t.Parallel()

	pod := addCondition(t, newUnstructured("v1", "Pod", "default", "my-pod"), "Ready", "True")
	server := fakeAPIServer(map[string]*unstructured.Unstructured{
		"/api/v1/namespaces/default/pods/my-pod": pod,
	})
	defer server.Close()

	configFlags := newTestConfigFlagsWithServer(server, "default")
	restConfig, err := configFlags.ToRESTConfig()
	require.NoError(t, err)
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	require.NoError(t, err)

	ctx := context.Background()

	// Test waiting for condition=Ready
	err = forResource(ctx, configFlags, dynamicClient, "Ready", "pods", "my-pod", 5*time.Second)
	require.NoError(t, err)
}

// TestForResourceJSONPathCondition tests waiting for a JSONPath condition
func TestForResourceJSONPathCondition(t *testing.T) {
	t.Parallel()

	pod := setNestedField(t, newUnstructured("v1", "Pod", "default", "my-pod"), "Running", "status", "phase")
	server := fakeAPIServer(map[string]*unstructured.Unstructured{
		"/api/v1/namespaces/default/pods/my-pod": pod,
	})
	defer server.Close()

	configFlags := newTestConfigFlagsWithServer(server, "default")
	restConfig, err := configFlags.ToRESTConfig()
	require.NoError(t, err)
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	require.NoError(t, err)

	ctx := context.Background()

	// Test waiting for jsonpath condition
	err = forResource(ctx, configFlags, dynamicClient, "{.status.phase}=Running", "pods", "my-pod", 5*time.Second)
	require.NoError(t, err)
}

// TestForResourceInputValidation tests validation of required inputs
func TestForResourceInputValidation(t *testing.T) {
	t.Parallel()

	// These tests don't need a server since they fail before connecting
	ctx := context.Background()
	configFlags := genericclioptions.NewConfigFlags(false)
	restConfig := &rest.Config{Host: "http://localhost:0"}
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	require.NoError(t, err)

	t.Run("empty kind returns error", func(t *testing.T) {
		err := forResource(ctx, configFlags, dynamicClient, "", "", "my-pod", time.Second)
		require.Error(t, err)
		require.Contains(t, err.Error(), "arguments in resource/name form")
	})
}

// TestForResourcePublicAPIValidation tests the public ForResource API validation
func TestForResourcePublicAPIValidation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("empty kind returns error", func(t *testing.T) {
		err := ForResource(ctx, "default", "", "", "my-pod", time.Second)
		require.Error(t, err)
		require.Contains(t, err.Error(), "kind is required")
	})

	t.Run("empty identifier returns error", func(t *testing.T) {
		err := ForResource(ctx, "default", "", "pod", "", time.Second)
		require.Error(t, err)
		require.Contains(t, err.Error(), "identifier is required")
	})
}
