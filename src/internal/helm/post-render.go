package helm

import (
	"bytes"
	"fmt"
	"os"
	"reflect"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/k8s"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"
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
	connectStrings types.ConnectStrings
	options        ChartOptions
	namespaces     map[string]*corev1.Namespace
}

func NewRenderer(options ChartOptions, actionConfig *action.Configuration) *renderer {
	message.Debugf("helm.NewRenderer(%#v)", options)
	return &renderer{
		actionConfig:   actionConfig,
		connectStrings: make(types.ConnectStrings),
		options:        options,
		namespaces: map[string]*corev1.Namespace{
			// Add the passed-in namespace to the list
			options.Chart.Namespace: nil,
		},
	}
}

func (r *renderer) Run(renderedManifests *bytes.Buffer) (*bytes.Buffer, error) {
	message.Debugf("helm.Run(renderedManifests *bytes.Buffer)")
	// This is very low cost and consistent for how we replace elsewhere, also good for debugging
	tempDir, _ := utils.MakeTempDir()
	path := tempDir + "/chart.yaml"

	// Write the context to a file for processing
	if err := utils.WriteFile(path, renderedManifests.Bytes()); err != nil {
		return nil, fmt.Errorf("unable to write the post-render file for the helm chart")
	}

	// Run the template engine against the chart output
	k8s.ProcessYamlFilesInPath(tempDir, r.options.Component)

	// Read back the templated file contents
	buff, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading temporary post-rendered helm chart: %w", err)
	}

	// Use helm to resplit the manifest byte (same call used by helm to pass this data to postRender)
	_, resources, err := releaseutil.SortManifests(map[string]string{path: string(buff)},
		r.actionConfig.Capabilities.APIVersions,
		releaseutil.InstallOrder,
	)

	if err != nil {
		return nil, fmt.Errorf("error re-rendering helm output: %w", err)
	}

	// Dump the contents for debugging
	message.Debug(resources)

	finalManifestsOutput := bytes.NewBuffer(nil)

	if err != nil {
		// On error only drop a warning
		message.Errorf(err, "Problem parsing post-render manifest data")
	} else {
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
				// parse the namespace resource so it can be applied out-of-band by zarf instead of helm to avoid helm ns shennanigans
				if err := runtime.DefaultUnstructuredConverter.FromUnstructured(rawData.UnstructuredContent(), &namespace); err != nil {
					message.Errorf(err, "could not parse namespace %s", rawData.GetName())
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
				// skip so we can strip namespaces from helms brain
				continue

			case "Service":
				// Check service resources for the zarf-connect label
				labels := rawData.GetLabels()
				annotations := rawData.GetAnnotations()

				if key, keyExists := labels[config.ZarfConnectLabelName]; keyExists {
					// If there is a zarf-connect label
					message.Debugf("Match helm service %s for zarf connection %s", rawData.GetName(), key)

					// Add the connectstring for processing later in the deployment
					r.connectStrings[key] = types.ConnectString{
						Description: annotations[config.ZarfConnectAnnotationDescription],
						Url:         annotations[config.ZarfConnectAnnotationUrl],
					}
				}
			}

			namespace := rawData.GetNamespace()
			if _, exists := r.namespaces[namespace]; !exists && namespace != "" {
				// if this is the first time seeing this ns, we need to track that to create it as well
				r.namespaces[namespace] = nil
			}

			// Finally place this back onto the output buffer
			fmt.Fprintf(finalManifestsOutput, "---\n# Source: %s\n%s\n", resource.Name, resource.Content)
		}
	}

	existingNamespaces, _ := k8s.GetNamespaces()

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
			if _, err := k8s.CreateNamespace(name, namespace); err != nil {
				return nil, fmt.Errorf("unable to create the missing namespace %s", name)
			}
		}

		// Create the secret
		validSecret := k8s.GenerateRegistryPullCreds(name, config.ZarfImagePullSecretName)

		// Try to get a valid existing secret
		currentSecret, _ := k8s.GetSecret(name, config.ZarfImagePullSecretName)
		if currentSecret.Name != config.ZarfImagePullSecretName || !reflect.DeepEqual(currentSecret.Data, validSecret.Data) {
			// create/update the missing zarf registry secret
			if err := k8s.ReplaceSecret(validSecret); err != nil {
				message.Errorf(err, "Problem creating registry secret for the %s namespace", name)
			}

			// Generate the git server secret
			gitServerSecret := k8s.GenerateSecret(name, config.ZarfGitServerSecretName, corev1.SecretTypeOpaque)
			gitServerSecret.StringData = map[string]string{
				"username": config.ZarfGitReadUser,
				"password": config.GetSecret(config.StateGitPull),
			}

			// Update the git server secret
			if err := k8s.ReplaceSecret(gitServerSecret); err != nil {
				message.Errorf(err, "Problem creating git server secret for the %s namespace", name)
			}
		}

	}

	// Cleanup the temp file
	_ = os.RemoveAll(tempDir)

	// Send the bytes back to helm
	return finalManifestsOutput, nil
}
