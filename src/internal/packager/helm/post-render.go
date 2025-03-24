// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helm contains operations for working with helm charts.
package helm

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/variables"
	"github.com/zarf-dev/zarf/src/types"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/releaseutil"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
	"sigs.k8s.io/yaml"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type renderer struct {
	// *Helm
	chartPath              string
	adoptExistingResources bool
	chart                  v1alpha1.ZarfChart
	cluster                *cluster.Cluster
	airgap                 bool
	state                  *types.ZarfState
	actionConfig           *action.Configuration
	variableConfig         *variables.VariableConfig
	connectStrings         types.ConnectStrings
	namespaces             map[string]*corev1.Namespace
}

type templateRenderer struct {
	chartPath      string
	actionConfig   *action.Configuration
	variableConfig *variables.VariableConfig
}

func newTemplateRenderer(chartPath string, actionConfig *action.Configuration, vc *variables.VariableConfig) (*templateRenderer, error) {
	rend := &templateRenderer{
		chartPath: chartPath,
		actionConfig: actionConfig,
		variableConfig: vc,
	}
	return rend, nil
}

func (r *templateRenderer) Run(renderedManifests *bytes.Buffer) (*bytes.Buffer, error) {
	// This is very low cost and consistent for how we replace elsewhere, also good for debugging
	tempDir, err := utils.MakeTempDir(r.chartPath)
	if err != nil {
		return nil, fmt.Errorf("unable to create tmpdir:  %w", err)
	}
	path := filepath.Join(tempDir, "chart.yaml")

	if err := os.WriteFile(path, renderedManifests.Bytes(), helpers.ReadWriteUser); err != nil {
		return nil, fmt.Errorf("unable to write the post-render file for the helm chart")
	}

	// Run the template engine against the chart output
	if err := r.variableConfig.ReplaceTextTemplate(path); err != nil {
		return nil, fmt.Errorf("error templating the helm chart: %w", err)
	}

	// Read back the templated file contents
	buff, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading temporary post-rendered helm chart: %w", err)
	}

	// Use helm to re-split the manifest byte (same call used by helm to pass this data to postRender)
	_, resources, err := releaseutil.SortManifests(map[string]string{path: string(buff)},
		r.actionConfig.Capabilities.APIVersions,
		releaseutil.InstallOrder,
	)
	if err != nil {
		return nil, fmt.Errorf("error re-rendering helm output: %w", err)
	}

	finalManifestsOutput := bytes.NewBuffer(nil)

	for _, resource := range resources {
		fmt.Fprintf(finalManifestsOutput, "---\n# Source: %s\n%s\n", resource.Name, resource.Content)
	}

	return finalManifestsOutput, nil
}

func (h *Helm) newRenderer(ctx context.Context) (*renderer, error) {
	rend := &renderer{
		connectStrings: types.ConnectStrings{},
		namespaces:     map[string]*corev1.Namespace{},
	}
	if h.cluster == nil {
		return rend, nil
	}

	namespace, err := h.cluster.Clientset.CoreV1().Namespaces().Get(ctx, h.chart.Namespace, metav1.GetOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		return nil, fmt.Errorf("unable to check for existing namespace %q in cluster: %w", h.chart.Namespace, err)
	}
	if kerrors.IsNotFound(err) {
		rend.namespaces[h.chart.Namespace] = cluster.NewZarfManagedNamespace(h.chart.Namespace)
	} else if rend.adoptExistingResources {
		namespace.Labels = cluster.AdoptZarfManagedLabels(namespace.Labels)
		rend.namespaces[h.chart.Namespace] = namespace
	}

	return rend, nil
}

func (r *renderer) Run(renderedManifests *bytes.Buffer) (*bytes.Buffer, error) {
	// This is very low cost and consistent for how we replace elsewhere, also good for debugging
	tempDir, err := utils.MakeTempDir(r.chartPath)
	if err != nil {
		return nil, fmt.Errorf("unable to create tmpdir:  %w", err)
	}
	path := filepath.Join(tempDir, "chart.yaml")

	if err := os.WriteFile(path, renderedManifests.Bytes(), helpers.ReadWriteUser); err != nil {
		return nil, fmt.Errorf("unable to write the post-render file for the helm chart")
	}

	// Run the template engine against the chart output
	if err := r.variableConfig.ReplaceTextTemplate(path); err != nil {
		return nil, fmt.Errorf("error templating the helm chart: %w", err)
	}

	// Read back the templated file contents
	buff, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading temporary post-rendered helm chart: %w", err)
	}

	// Use helm to re-split the manifest byte (same call used by helm to pass this data to postRender)
	_, resources, err := releaseutil.SortManifests(map[string]string{path: string(buff)},
		r.actionConfig.Capabilities.APIVersions,
		releaseutil.InstallOrder,
	)

	if err != nil {
		return nil, fmt.Errorf("error re-rendering helm output: %w", err)
	}

	finalManifestsOutput := bytes.NewBuffer(nil)

	if r.cluster != nil {
		ctx := context.Background()

		if err := r.editHelmResources(ctx, resources, finalManifestsOutput); err != nil {
			return nil, err
		}

		if err := r.adoptAndUpdateNamespaces(ctx); err != nil {
			return nil, err
		}
	} else {
		for _, resource := range resources {
			fmt.Fprintf(finalManifestsOutput, "---\n# Source: %s\n%s\n", resource.Name, resource.Content)
		}
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
		if !r.airgap && r.state.Distro == "YOLO" {
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
	dc, err := dynamic.NewForConfig(r.cluster.RestConfig)
	if err != nil {
		return err
	}
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
				resource, err := dc.Resource(mapping.Resource).Namespace(deployedNamespace).Get(ctx, rawData.GetName(), metav1.GetOptions{})
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
				_, err = dc.Resource(mapping.Resource).Namespace(deployedNamespace).Update(ctx, resource, metav1.UpdateOptions{})
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
