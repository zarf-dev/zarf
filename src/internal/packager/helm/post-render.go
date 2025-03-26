// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helm contains operations for working with helm charts.
package helm

import (
	"bytes"
	"context"
	"fmt"
	"slices"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/variables"
	"github.com/zarf-dev/zarf/src/types"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/releaseutil"
	"k8s.io/client-go/restmapper"
	"sigs.k8s.io/yaml"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type renderer struct {
	chart v1alpha1.ZarfChart

	adoptExistingResources bool
	cluster                *cluster.Cluster
	skipSecretUpdates      bool
	state                  *types.ZarfState
	actionConfig           *action.Configuration
	variableConfig         *variables.VariableConfig

	connectStrings types.ConnectStrings
	namespaces     map[string]*corev1.Namespace
}

func newRenderer(ctx context.Context, chart v1alpha1.ZarfChart, adoptExistingResources bool, c *cluster.Cluster, airgapMode bool, state *types.ZarfState,
	actionConfig *action.Configuration, variableConfig *variables.VariableConfig) (*renderer, error) {
	if c == nil {
		return nil, fmt.Errorf("cluster required to run post renderer")
	}
	skipSecretUpdates := !airgapMode && state.Distro == "YOLO"
	rend := &renderer{
		chart:                  chart,
		adoptExistingResources: adoptExistingResources,
		cluster:                c,
		skipSecretUpdates:      skipSecretUpdates,
		state:                  state,
		actionConfig:           actionConfig,
		variableConfig:         variableConfig,
		connectStrings:         types.ConnectStrings{},
		namespaces:             map[string]*corev1.Namespace{},
	}

	namespace, err := rend.cluster.Clientset.CoreV1().Namespaces().Get(ctx, rend.chart.Namespace, metav1.GetOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		return nil, fmt.Errorf("unable to check for existing namespace %q in cluster: %w", rend.chart.Namespace, err)
	}
	if kerrors.IsNotFound(err) {
		rend.namespaces[rend.chart.Namespace] = cluster.NewZarfManagedNamespace(rend.chart.Namespace)
	} else if rend.adoptExistingResources {
		namespace.Labels = cluster.AdoptZarfManagedLabels(namespace.Labels)
		rend.namespaces[rend.chart.Namespace] = namespace
	}

	return rend, nil
}

func (r *renderer) Run(renderedManifests *bytes.Buffer) (*bytes.Buffer, error) {
	// This is very low cost and consistent for how we replace elsewhere, also good for debugging
	resources, err := getTemplatedManifests(renderedManifests, r.variableConfig, r.actionConfig)
	if err != nil {
		return nil, err
	}
	finalManifestsOutput := bytes.NewBuffer(nil)
	ctx := context.Background()
	if err := r.editHelmResources(ctx, resources, finalManifestsOutput); err != nil {
		return nil, err
	}
	if err := r.adoptAndUpdateNamespaces(ctx); err != nil {
		return nil, err
	}
	// Send the bytes back to helm
	return finalManifestsOutput, nil
}

func (r *renderer) adoptAndUpdateNamespaces(ctx context.Context) error {
	l := logger.From(ctx)
	c := r.cluster
	namespaceList, err := r.cluster.Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	for name, namespace := range r.namespaces {
		// Check to see if this namespace already exists
		var existingNamespace bool
		for _, serverNamespace := range namespaceList.Items {
			if serverNamespace.Name == name {
				existingNamespace = true
			}
		}

		if !existingNamespace {
			// This is a new namespace, add it
			_, err := c.Clientset.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("unable to create the missing namespace %s", name)
			}
		} else if r.adoptExistingResources {
			// Refuse to adopt namespace if it is one of four initial Kubernetes namespaces.
			// https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/#initial-namespaces
			if slices.Contains([]string{"default", "kube-node-lease", "kube-public", "kube-system"}, name) {
				message.Warnf("Refusing to adopt the initial namespace: %s", name)
				l.Warn("refusing to adopt initial namespace", "name", name)
			} else {
				// This is an existing namespace to adopt
				_, err := c.Clientset.CoreV1().Namespaces().Update(ctx, namespace, metav1.UpdateOptions{})
				if err != nil {
					return fmt.Errorf("unable to adopt the existing namespace %s", name)
				}
			}
		}

		// If the package is marked as YOLO and the state is empty, skip the secret creation for this namespace
		if r.skipSecretUpdates {
			continue
		}

		// Create the secret
		validRegistrySecret, err := c.GenerateRegistryPullCreds(ctx, name, config.ZarfImagePullSecretName, r.state.RegistryInfo)
		if err != nil {
			return err
		}
		_, err = c.Clientset.CoreV1().Secrets(*validRegistrySecret.Namespace).Apply(ctx, validRegistrySecret, metav1.ApplyOptions{Force: true, FieldManager: cluster.FieldManagerName})
		if err != nil {
			return fmt.Errorf("problem applying registry secret for the %s namespace: %w", name, err)
		}
		gitServerSecret := c.GenerateGitPullCreds(name, config.ZarfGitServerSecretName, r.state.GitServer)
		_, err = c.Clientset.CoreV1().Secrets(*gitServerSecret.Namespace).Apply(ctx, gitServerSecret, metav1.ApplyOptions{Force: true, FieldManager: cluster.FieldManagerName})
		if err != nil {
			return fmt.Errorf("problem applying git server secret for the %s namespace: %w", name, err)
		}
	}
	return nil
}

func (r *renderer) editHelmResources(ctx context.Context, resources []releaseutil.Manifest, finalManifestsOutput *bytes.Buffer) error {
	l := logger.From(ctx)
	groupResources, err := restmapper.GetAPIGroupResources(r.cluster.Clientset.Discovery())
	if err != nil {
		return err
	}
	mapper := restmapper.NewDiscoveryRESTMapper(groupResources)

	for _, resource := range resources {
		// parse to unstructured to have access to more data than just the name
		rawData := &unstructured.Unstructured{}
		if err := yaml.Unmarshal([]byte(resource.Content), rawData); err != nil {
			return fmt.Errorf("failed to unmarshal manifest: %w", err)
		}

		switch rawData.GetKind() {
		case "Namespace":
			namespace := &corev1.Namespace{}
			// parse the namespace resource so it can be applied out-of-band by zarf instead of helm to avoid helm ns shenanigans
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(rawData.UnstructuredContent(), namespace); err != nil {
				message.WarnErrf(err, "could not parse namespace %s", rawData.GetName())
				l.Warn("failed to parse namespace", "name", rawData.GetName(), "error", err)
			} else {
				message.Debugf("Matched helm namespace %s for zarf annotation", namespace.Name)
				l.Debug("matched helm namespace for zarf annotation", "name", namespace.Name)
				namespace.Labels = cluster.AdoptZarfManagedLabels(namespace.Labels)
				// Add it to the stack
				r.namespaces[namespace.Name] = namespace
			}
			// skip so we can strip namespaces from helm's brain
			continue

		case "Service":
			// Check service resources for the zarf-connect label
			labels := rawData.GetLabels()
			if labels == nil {
				labels = map[string]string{}
			}
			annotations := rawData.GetAnnotations()
			if annotations == nil {
				annotations = map[string]string{}
			}
			if key, keyExists := labels[cluster.ZarfConnectLabelName]; keyExists {
				// If there is a zarf-connect label
				message.Debugf("Match helm service %s for zarf connection %s", rawData.GetName(), key)
				l.Debug("match helm service for zarf connection", "service", rawData.GetName(), "connection-key", key)

				// Add the connectString for processing later in the deployment
				r.connectStrings[key] = types.ConnectString{
					Description: annotations[cluster.ZarfConnectAnnotationDescription],
					URL:         annotations[cluster.ZarfConnectAnnotationURL],
				}
			}
		}

		namespace := rawData.GetNamespace()
		if _, exists := r.namespaces[namespace]; !exists && namespace != "" {
			// if this is the first time seeing this ns, we need to track that to create it as well
			r.namespaces[namespace] = cluster.NewZarfManagedNamespace(namespace)
		}

		// If we have been asked to adopt existing resources, process those now as well
		if r.adoptExistingResources {
			deployedNamespace := namespace
			if deployedNamespace == "" {
				deployedNamespace = r.chart.Namespace
			}

			err := func() error {
				mapping, err := mapper.RESTMapping(rawData.GroupVersionKind().GroupKind())
				if err != nil {
					return err
				}
				resource, err := r.cluster.DynamicClient.Resource(mapping.Resource).Namespace(deployedNamespace).Get(ctx, rawData.GetName(), metav1.GetOptions{})
				// Ignore resources that are yet to be created
				if kerrors.IsNotFound(err) {
					return nil
				}
				if err != nil {
					return err
				}
				labels := resource.GetLabels()
				if labels == nil {
					labels = map[string]string{}
				}
				labels["app.kubernetes.io/managed-by"] = "Helm"
				resource.SetLabels(labels)
				annotations := resource.GetAnnotations()
				if annotations == nil {
					annotations = map[string]string{}
				}
				annotations["meta.helm.sh/release-name"] = r.chart.ReleaseName
				annotations["meta.helm.sh/release-namespace"] = r.chart.Namespace
				resource.SetAnnotations(annotations)
				_, err = r.cluster.DynamicClient.Resource(mapping.Resource).Namespace(deployedNamespace).Update(ctx, resource, metav1.UpdateOptions{})
				if err != nil {
					return err
				}
				return nil
			}()
			if err != nil {
				return fmt.Errorf("unable to adopt the resource %s: %w", rawData.GetName(), err)
			}
		}
		// Finally place this back onto the output buffer
		fmt.Fprintf(finalManifestsOutput, "---\n# Source: %s\n%s\n", resource.Name, resource.Content)
	}
	return nil
}
