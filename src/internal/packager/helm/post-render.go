// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helm contains operations for working with helm charts.
package helm

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/packager/template"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/releaseutil"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type renderer struct {
	actionConfig   *action.Configuration
	options        *Helm
	connectStrings types.ConnectStrings
	namespaces     map[string]*corev1.Namespace
	values         template.Values
}

func (h *Helm) newRenderer() (*renderer, error) {
	message.Debugf("helm.NewRenderer()")

	valueTemplate, err := template.Generate(h.Cfg)
	if err != nil {
		return nil, err
	}

	return &renderer{
		connectStrings: make(types.ConnectStrings),
		options:        h,
		namespaces: map[string]*corev1.Namespace{
			// Add the passed-in namespace to the list
			h.Chart.Namespace: h.Cluster.NewZarfManagedNamespace(h.Chart.Namespace),
		},
		values:       *valueTemplate,
		actionConfig: h.actionConfig,
	}, nil
}

func (r *renderer) Run(renderedManifests *bytes.Buffer) (*bytes.Buffer, error) {
	// This is very low cost and consistent for how we replace elsewhere, also good for debugging
	tempDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return nil, fmt.Errorf("unable to create tmpdir:  %w", err)
	}
	defer os.RemoveAll(tempDir)
	path := filepath.Join(tempDir, "chart.yaml")

	// Write the context to a file for processing
	if err := utils.WriteFile(path, renderedManifests.Bytes()); err != nil {
		return nil, fmt.Errorf("unable to write the post-render file for the helm chart")
	}

	// Run the template engine against the chart output
	if _, err := template.ProcessYamlFilesInPath(tempDir, r.options.Component, r.values); err != nil {
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

	// Otherwise, loop over the resources,
	for _, resource := range resources {

		// parse to unstructured to have access to more data than just the name
		rawData := &unstructured.Unstructured{}
		if err := yaml.Unmarshal([]byte(resource.Content), rawData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal manifest: %#v", err)
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
				namespace.Labels["zarf-helm-release"] = r.options.ReleaseName

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
			r.namespaces[namespace] = r.options.Cluster.NewZarfManagedNamespace(namespace)
		}

		// If we have been asked to adopt existing resources, process those now as well
		if r.options.Cfg.DeployOpts.AdoptExistingResources {
			deployedNamespace := namespace
			if deployedNamespace == "" {
				deployedNamespace = r.options.Chart.Namespace
			}

			helmLabels := map[string]string{"app.kubernetes.io/managed-by": "Helm"}
			helmAnnotations := map[string]string{
				"meta.helm.sh/release-name":      r.options.ReleaseName,
				"meta.helm.sh/release-namespace": r.options.Chart.Namespace,
			}

			if err := r.options.Cluster.AddLabelsAndAnnotations(deployedNamespace, rawData.GetName(), rawData.GroupVersionKind().GroupKind(), helmLabels, helmAnnotations); err != nil {
				// Print a debug message since this could just be because the resource doesn't exist
				message.Debugf("Unable to adopt resource %s: %s", rawData.GetName(), err.Error())
			}
		}

		// Finally place this back onto the output buffer
		fmt.Fprintf(finalManifestsOutput, "---\n# Source: %s\n%s\n", resource.Name, resource.Content)
	}

	c := r.options.Cluster
	existingNamespaces, _ := c.GetNamespaces()

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
			if _, err := c.CreateNamespace(namespace); err != nil {
				return nil, fmt.Errorf("unable to create the missing namespace %s", name)
			}
		} else if r.options.Cfg.DeployOpts.AdoptExistingResources {
			if r.options.Cluster.IsInitialNamespace(name) {
				// If this is a K8s initial namespace, refuse to adopt it
				message.Warnf("Refusing to adopt the initial namespace: %s", name)
			} else {
				// This is an existing namespace to adopt
				if _, err := c.UpdateNamespace(namespace); err != nil {
					return nil, fmt.Errorf("unable to adopt the existing namespace %s", name)
				}
			}
		}

		// If the package is marked as YOLO and the state is empty, skip the secret creation for this namespace
		if r.options.Cfg.Pkg.Metadata.YOLO && r.options.Cfg.State.Distro == "YOLO" {
			continue
		}

		// Create the secret
		validRegistrySecret := c.GenerateRegistryPullCreds(name, config.ZarfImagePullSecretName, r.options.Cfg.State.RegistryInfo)

		// Try to get a valid existing secret
		currentRegistrySecret, _ := c.GetSecret(name, config.ZarfImagePullSecretName)
		if currentRegistrySecret.Name != config.ZarfImagePullSecretName || !reflect.DeepEqual(currentRegistrySecret.Data, validRegistrySecret.Data) {
			// Create or update the zarf registry secret
			if err := c.CreateOrUpdateSecret(validRegistrySecret); err != nil {
				message.WarnErrf(err, "Problem creating registry secret for the %s namespace", name)
			}

			// Generate the git server secret
			gitServerSecret := c.GenerateGitPullCreds(name, config.ZarfGitServerSecretName, r.options.Cfg.State.GitServer)

			// Create or update the zarf git server secret
			if err := c.CreateOrUpdateSecret(gitServerSecret); err != nil {
				message.WarnErrf(err, "Problem creating git server secret for the %s namespace", name)
			}
		}
	}

	// Send the bytes back to helm
	return finalManifestsOutput, nil
}
