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
	"reflect"

	"github.com/defenseunicorns/pkg/helpers"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"helm.sh/helm/v3/pkg/releaseutil"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type renderer struct {
	*Helm
	connectStrings types.ConnectStrings
	namespaces     map[string]*corev1.Namespace
}

func (h *Helm) newRenderer() (*renderer, error) {
	message.Debugf("helm.NewRenderer()")

	namespaces := make(map[string]*corev1.Namespace)
	if h.cluster != nil {
		namespaces[h.chart.Namespace] = h.cluster.NewZarfManagedNamespace(h.chart.Namespace)
	}

	return &renderer{
		Helm:           h,
		connectStrings: make(types.ConnectStrings),
		namespaces:     namespaces,
	}, nil
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
	c := r.cluster
	existingNamespaces, _ := c.GetNamespaces(ctx)
	for name, namespace := range r.namespaces {

		// Check to see if this namespace already exists
		var existingNamespace bool
		for _, serverNamespace := range existingNamespaces.Items {
			if serverNamespace.Name == name {
				existingNamespace = true
			}
		}

		if !existingNamespace {
			// This is a new namespace, add it
			if _, err := c.CreateNamespace(ctx, namespace); err != nil {
				return fmt.Errorf("unable to create the missing namespace %s", name)
			}
		} else if r.cfg.DeployOpts.AdoptExistingResources {
			if r.cluster.IsInitialNamespace(name) {
				// If this is a K8s initial namespace, refuse to adopt it
				message.Warnf("Refusing to adopt the initial namespace: %s", name)
			} else {
				// This is an existing namespace to adopt
				if _, err := c.UpdateNamespace(ctx, namespace); err != nil {
					return fmt.Errorf("unable to adopt the existing namespace %s", name)
				}
			}
		}

		// If the package is marked as YOLO and the state is empty, skip the secret creation for this namespace
		if r.cfg.Pkg.Metadata.YOLO && r.state.Distro == "YOLO" {
			continue
		}

		// Create the secret
		validRegistrySecret := c.GenerateRegistryPullCreds(ctx, name, config.ZarfImagePullSecretName, r.state.RegistryInfo)

		// Try to get a valid existing secret
		currentRegistrySecret, _ := c.GetSecret(ctx, name, config.ZarfImagePullSecretName)
		if currentRegistrySecret.Name != config.ZarfImagePullSecretName || !reflect.DeepEqual(currentRegistrySecret.Data, validRegistrySecret.Data) {
			// Create or update the zarf registry secret
			if _, err := c.CreateOrUpdateSecret(ctx, validRegistrySecret); err != nil {
				message.WarnErrf(err, "Problem creating registry secret for the %s namespace", name)
			}

			// Generate the git server secret
			gitServerSecret := c.GenerateGitPullCreds(name, config.ZarfGitServerSecretName, r.state.GitServer)

			// Create or update the zarf git server secret
			if _, err := c.CreateOrUpdateSecret(ctx, gitServerSecret); err != nil {
				message.WarnErrf(err, "Problem creating git server secret for the %s namespace", name)
			}
		}
	}
	return nil
}

func (r *renderer) editHelmResources(ctx context.Context, resources []releaseutil.Manifest, finalManifestsOutput *bytes.Buffer) error {
	for _, resource := range resources {
		// parse to unstructured to have access to more data than just the name
		rawData := &unstructured.Unstructured{}
		if err := yaml.Unmarshal([]byte(resource.Content), rawData); err != nil {
			return fmt.Errorf("failed to unmarshal manifest: %#v", err)
		}

		switch rawData.GetKind() {
		case "Namespace":
			var namespace corev1.Namespace
			// parse the namespace resource so it can be applied out-of-band by zarf instead of helm to avoid helm ns shenanigans
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(rawData.UnstructuredContent(), &namespace); err != nil {
				message.WarnErrf(err, "could not parse namespace %s", rawData.GetName())
			} else {
				message.Debugf("Matched helm namespace %s for zarf annotation", namespace.Name)
				if namespace.Labels == nil {
					// Ensure label map exists to avoid nil panic
					namespace.Labels = make(map[string]string)
				}
				// Now track this namespace by zarf
				namespace.Labels[config.ZarfManagedByLabel] = "zarf"
				namespace.Labels["zarf-helm-release"] = r.chart.ReleaseName

				// Add it to the stack
				r.namespaces[namespace.Name] = &namespace
			}
			// skip so we can strip namespaces from helm's brain
			continue

		case "Service":
			// Check service resources for the zarf-connect label
			labels := rawData.GetLabels()
			annotations := rawData.GetAnnotations()

			if key, keyExists := labels[config.ZarfConnectLabelName]; keyExists {
				// If there is a zarf-connect label
				message.Debugf("Match helm service %s for zarf connection %s", rawData.GetName(), key)

				// Add the connectString for processing later in the deployment
				r.connectStrings[key] = types.ConnectString{
					Description: annotations[config.ZarfConnectAnnotationDescription],
					URL:         annotations[config.ZarfConnectAnnotationURL],
				}
			}
		}

		namespace := rawData.GetNamespace()
		if _, exists := r.namespaces[namespace]; !exists && namespace != "" {
			// if this is the first time seeing this ns, we need to track that to create it as well
			r.namespaces[namespace] = r.cluster.NewZarfManagedNamespace(namespace)
		}

		// If we have been asked to adopt existing resources, process those now as well
		if r.cfg.DeployOpts.AdoptExistingResources {
			deployedNamespace := namespace
			if deployedNamespace == "" {
				deployedNamespace = r.chart.Namespace
			}

			helmLabels := map[string]string{"app.kubernetes.io/managed-by": "Helm"}
			helmAnnotations := map[string]string{
				"meta.helm.sh/release-name":      r.chart.ReleaseName,
				"meta.helm.sh/release-namespace": r.chart.Namespace,
			}

			if err := r.cluster.AddLabelsAndAnnotations(ctx, deployedNamespace, rawData.GetName(), rawData.GroupVersionKind().GroupKind(), helmLabels, helmAnnotations); err != nil {
				// Print a debug message since this could just be because the resource doesn't exist
				message.Debugf("Unable to adopt resource %s: %s", rawData.GetName(), err.Error())
			}
		}
		// Finally place this back onto the output buffer
		fmt.Fprintf(finalManifestsOutput, "---\n# Source: %s\n%s\n", resource.Name, resource.Content)
	}
	return nil
}
