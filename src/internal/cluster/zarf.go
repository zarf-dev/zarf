// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"context"
	"encoding/json"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
	"k8s.io/api/autoscaling/v2beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetDeployedZarfPackages gets metadata information about packages that have been deployed to the cluster.
// We determine what packages have been deployed to the cluster by looking for specific secrets in the Zarf namespace.
func (c *Cluster) GetDeployedZarfPackages() ([]types.DeployedPackage, error) {
	var deployedPackages = []types.DeployedPackage{}

	// Get the secrets that describe the deployed packages
	secrets, err := c.Kube.GetSecretsWithLabel(ZarfNamespace, ZarfPackageInfoLabel)
	if err != nil {
		return deployedPackages, err
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
func (c *Cluster) StripZarfLabelsAndSecretsFromNamespaces() {
	spinner := message.NewProgressSpinner("Removing zarf metadata & secrets from existing namespaces not managed by Zarf")
	defer spinner.Stop()

	deleteOptions := metav1.DeleteOptions{}
	listOptions := metav1.ListOptions{
		LabelSelector: config.ZarfManagedByLabel + "=zarf",
	}

	if namespaces, err := c.Kube.GetNamespaces(); err != nil {
		spinner.Errorf(err, "Unable to get k8s namespaces")
	} else {
		for _, namespace := range namespaces.Items {
			if _, ok := namespace.Labels[agentLabel]; ok {
				spinner.Updatef("Removing Zarf Agent label for namespace %s", namespace.Name)
				delete(namespace.Labels, agentLabel)
				if _, err = c.Kube.UpdateNamespace(&namespace); err != nil {
					// This is not a hard failure, but we should log it
					spinner.Errorf(err, "Unable to update the namespace labels for %s", namespace.Name)
				}
			}

			for _, namespace := range namespaces.Items {
				spinner.Updatef("Removing Zarf secrets for namespace %s", namespace.Name)
				err := c.Kube.Clientset.CoreV1().
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

// RecordPackageDeployment saves metadata about a package that has been deployed to the cluster.
func (c *Cluster) RecordPackageDeployment(pkg types.ZarfPackage, components []types.DeployedComponent) {
	// Generate a secret that describes the package that is being deployed
	packageName := pkg.Metadata.Name
	deployedPackageSecret := c.Kube.GenerateSecret(ZarfNamespace, config.ZarfPackagePrefix+packageName, corev1.SecretTypeOpaque)
	deployedPackageSecret.Labels[ZarfPackageInfoLabel] = packageName

	stateData, _ := json.Marshal(types.DeployedPackage{
		Name:               packageName,
		CLIVersion:         config.CLIVersion,
		Data:               pkg,
		DeployedComponents: components,
	})

	deployedPackageSecret.Data = map[string][]byte{"data": stateData}

	c.Kube.CreateOrUpdateSecret(deployedPackageSecret)
}

// EnableRegistryHPAScaleDown enables or disables the HPA scale down for the Zarf Registry.
func (c *Cluster) EnableRegistryHPAScaleDown(enable bool) error {
	hpa, err := c.Kube.GetHPA(ZarfNamespace, "zarf-docker-registry")
	if err != nil {
		return err
	}

	var policy v2beta2.ScalingPolicySelect

	if enable {
		// Enable HPA scale down.
		policy = v2beta2.MinPolicySelect
	} else {
		// Disable HPA scale down.
		policy = v2beta2.DisabledPolicySelect
	}

	hpa.Spec.Behavior.ScaleDown.SelectPolicy = &policy

	// Save the HPA changes.
	if _, err = c.Kube.UpdateHPA(hpa); err != nil {
		return err
	}

	return nil
}
