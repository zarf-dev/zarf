package helm

import (
	"bytes"
	"fmt"
	"os"

	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/k8s"
	"github.com/defenseunicorns/zarf/cli/internal/message"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/releaseutil"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const secretName = "zarf-registry"
const managedByLabel = "app.kubernetes.io/managed-by"

type renderer struct {
	actionConfig   *action.Configuration
	connectStrings ConnectStrings
	options        ChartOptions
	namespaces     map[string]*corev1.Namespace
}

func NewRenderer(options ChartOptions, actionConfig *action.Configuration) *renderer {
	message.Debugf("helm.NewRenderer(%v)", options)
	return &renderer{
		actionConfig:   actionConfig,
		connectStrings: make(ConnectStrings),
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
	k8s.ProcessYamlFilesInPath(tempDir, r.options.Images)

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
				return nil, fmt.Errorf("failed to unmarshal manifest: %v", err)
			}

			switch rawData.GetKind() {
			case "Namespace":
				var namespace corev1.Namespace
				// parse the namespace resource so it can be applied out-of-band by zarf instead of helm to avoid helm ns shennanigans
				if err := runtime.DefaultUnstructuredConverter.FromUnstructured(rawData.UnstructuredContent(), &namespace); err != nil {
					message.Errorf(err, "could not parse namespace %s", rawData.GetName())
				} else {
					message.Debugf("Matched helm namespace %s for zarf annotation", &namespace.Name)
					if namespace.Labels == nil {
						// Ensure label map exists to avoid nil panic
						namespace.Labels = make(map[string]string)
					}
					// Now track this namespace by zarf
					namespace.Labels[managedByLabel] = "zarf"
					namespace.Labels["zarf-helm-release"] = r.options.ReleaseName

					// Add it to the stack
					r.namespaces[namespace.Name] = &namespace
				}
				// skip so we can strip namespaces from helms brain
				continue

			case "ServiceAccount":
				var svcAccount corev1.ServiceAccount
				if err := runtime.DefaultUnstructuredConverter.FromUnstructured(rawData.UnstructuredContent(), &svcAccount); err != nil {
					message.Errorf(err, "could not parse service account %s", rawData.GetName())
				} else {
					message.Debugf("Matched helm svc account %s for zarf annotation", &svcAccount.Name)

					// Add the zarf image pull secret to the sa
					svcAccount.ImagePullSecrets = append(svcAccount.ImagePullSecrets, corev1.LocalObjectReference{
						Name: secretName,
					})

					if byteData, err := yaml.Marshal(svcAccount); err != nil {
						message.Error(err, "unable to marshal svc account")
					} else {
						// Update the contents of the svc account
						resource.Content = string(byteData)
					}
				}

			case "Service":
				// Check service resources for the zarf-connect label
				labels := rawData.GetLabels()
				annotations := rawData.GetAnnotations()

				if key, keyExists := labels[config.ZarfConnectLabelName]; keyExists {
					// If there is a zarf-connect label
					message.Debugf("Match helm service %s for zarf connection %s", rawData.GetName(), key)

					// Add the connectstring for processing later in the deployment
					r.connectStrings[key] = ConnectString{
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

		// Try to get an existing secret
		if secret, _ := k8s.GetSecret(name, secretName); secret.Name != secretName {
			// create the missing zarf secret
			secret = k8s.GenerateRegistryPullCreds(name, secretName)
			if err := k8s.CreateSecret(secret); err != nil {
				message.Errorf(err, "Problem creating registry secret for the %s namespace", name)
			}
		}

		// Keep the default SAs up-to-date so they will use the zarf-registry pull secret for the namespace
		if defaultSvcAccount, err := k8s.GetServiceAccount(name, corev1.NamespaceDefault); err != nil {
			return nil, fmt.Errorf("unable to get service acocunts for namespace %s", name)
		} else {
			// Look to see if the service account is already patched
			if defaultSvcAccount.Labels[managedByLabel] != "zarf" {
				updateSvcAccount(defaultSvcAccount)
			}
		}
	}

	// Cleanup the temp file
	_ = os.RemoveAll(tempDir)

	// Send the bytes back to helm
	return finalManifestsOutput, nil
}

func updateSvcAccount(svcAccount *corev1.ServiceAccount) error {
	// This service acocunt needs the pull secret added
	svcAccount.ImagePullSecrets = append(svcAccount.ImagePullSecrets, corev1.LocalObjectReference{
		Name: secretName,
	})

	if svcAccount.Labels == nil {
		// Ensure label map exists to avoid nil panic
		svcAccount.Labels = make(map[string]string)
	}

	// Track this by zarf
	svcAccount.Labels[managedByLabel] = "zarf"

	// Finally update the chnage on the server
	if _, err := k8s.SaveServiceAccount(svcAccount); err != nil {
		return fmt.Errorf("unable to update the default service account for the %s namespace: %w", svcAccount.Namespace, err)
	}

	return nil
}
