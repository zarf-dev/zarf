package k8s

import (
	"context"
	"encoding/json"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetDeployedZarfPackages gets metadata information about packages that have been deployed to the cluster.
// We determine what packages have been deployed to the cluster by looking for specific secrets in the Zarf namespace.
func GetDeployedZarfPackages() ([]types.DeployedPackage, error) {
	var deployedPackages = []types.DeployedPackage{}

	// Get the secrets that describe the deployed packages
	namespace := "zarf"
	labelSelector := "package-deploy-info"
	secrets, err := GetSecretsWithLabel(namespace, labelSelector)
	if err != nil {
		message.Fatalf(err, "unable to get secrets with the label selector")
	}

	// Process the k8s secret into our internal structs
	for _, secret := range secrets.Items {
		var deployedPackage types.DeployedPackage
		err := json.Unmarshal(secret.Data["data"], &deployedPackage)
		if err != nil {
			message.Warnf("Unable to unmarshal package secret")

			return deployedPackages, err
		}

		deployedPackages = append(deployedPackages, deployedPackage)
	}

	return deployedPackages, nil
}

// StripZarfLabelsAndSecretsFromNamespaces removes metadata and secrets from existing namespaces no longer manged by Zarf.
func StripZarfLabelsAndSecretsFromNamespaces() {
	spinner := message.NewProgressSpinner("Removing zarf metadata & secrets from existing namespaces not managed by Zarf")
	defer spinner.Stop()

	clientset, err := getClientset()
	if err != nil {
		spinner.Errorf(err, "unable to get k8s clientset")
	}

	deleteOptions := metav1.DeleteOptions{}
	listOptions := metav1.ListOptions{
		LabelSelector: config.ZarfManagedByLabel + "=zarf",
	}

	if namespaces, err := GetNamespaces(); err != nil {
		spinner.Errorf(err, "Unable to get k8s namespaces")
	} else {
		for _, namespace := range namespaces.Items {
			if _, ok := namespace.Labels["zarf.dev/agent"]; ok {
				spinner.Updatef("Removing Zarf Agent label for namespace %s", namespace.Name)
				delete(namespace.Labels, "zarf.dev/agent")
				if _, err = UpdateNamespace(&namespace); err != nil {
					// This is not a hard failure, but we should log it
					spinner.Errorf(err, "Unable to update the namespace labels for %s", namespace.Name)
				}
			}

			for _, namespace := range namespaces.Items {
				spinner.Updatef("Removing Zarf secrets for namespace %s", namespace.Name)
				err := clientset.CoreV1().
					Secrets(namespace.Name).
					DeleteCollection(context.TODO(), deleteOptions, listOptions)
				if err != nil {
					spinner.Errorf(err, "Unable to delete secrets from namespace %s", namespace.Name)
				}
			}
		}
	}

	spinner.Success()
}
