// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	autoscalingV2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
)

// GetDeployedZarfPackages gets metadata information about packages that have been deployed to the cluster.
// We determine what packages have been deployed to the cluster by looking for specific secrets in the Zarf namespace.
// Returns a list of DeployedPackage structs and a list of errors.
func (c *Cluster) GetDeployedZarfPackages(ctx context.Context) ([]types.DeployedPackage, []error) {
	var deployedPackages = []types.DeployedPackage{}
	var errorList []error
	// Get the secrets that describe the deployed packages
	secrets, err := c.Clientset.CoreV1().Secrets(ZarfNamespaceName).List(ctx, metav1.ListOptions{LabelSelector: ZarfPackageInfoLabel})
	if err != nil {
		return deployedPackages, append(errorList, err)
	}

	// Process the k8s secret into our internal structs
	for _, secret := range secrets.Items {
		if strings.HasPrefix(secret.Name, config.ZarfPackagePrefix) {
			var deployedPackage types.DeployedPackage
			err := json.Unmarshal(secret.Data["data"], &deployedPackage)
			// add the error to the error list
			if err != nil {
				errorList = append(errorList, fmt.Errorf("unable to unmarshal the secret %s/%s", secret.Namespace, secret.Name))
			} else {
				deployedPackages = append(deployedPackages, deployedPackage)
			}
		}
	}

	// TODO: If we move this function out of `internal` we should return a more standard singular error.
	return deployedPackages, errorList
}

// GetDeployedPackage gets the metadata information about the package name provided (if it exists in the cluster).
// We determine what packages have been deployed to the cluster by looking for specific secrets in the Zarf namespace.
func (c *Cluster) GetDeployedPackage(ctx context.Context, packageName string) (deployedPackage *types.DeployedPackage, err error) {
	// Get the secret that describes the deployed package
	secret, err := c.Clientset.CoreV1().Secrets(ZarfNamespaceName).Get(ctx, config.ZarfPackagePrefix+packageName, metav1.GetOptions{})
	if err != nil {
		return deployedPackage, err
	}

	return deployedPackage, json.Unmarshal(secret.Data["data"], &deployedPackage)
}

// StripZarfLabelsAndSecretsFromNamespaces removes metadata and secrets from existing namespaces no longer manged by Zarf.
func (c *Cluster) StripZarfLabelsAndSecretsFromNamespaces(ctx context.Context) {
	spinner := message.NewProgressSpinner("Removing zarf metadata & secrets from existing namespaces not managed by Zarf")
	defer spinner.Stop()

	deleteOptions := metav1.DeleteOptions{}
	listOptions := metav1.ListOptions{
		LabelSelector: config.ZarfManagedByLabel + "=zarf",
	}

	namespaceList, err := c.Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		spinner.Errorf(err, "Unable to get k8s namespaces")
	} else {
		for _, namespace := range namespaceList.Items {
			if _, ok := namespace.Labels[agentLabel]; ok {
				spinner.Updatef("Removing Zarf Agent label for namespace %s", namespace.Name)
				delete(namespace.Labels, agentLabel)
				// NOTE: Why make a copy of namespace?
				namespaceCopy := namespace
				_, err := c.Clientset.CoreV1().Namespaces().Update(ctx, &namespaceCopy, metav1.UpdateOptions{})
				if err != nil {
					// This is not a hard failure, but we should log it
					spinner.Errorf(err, "Unable to update the namespace labels for %s", namespace.Name)
				}
			}

			spinner.Updatef("Removing Zarf secrets for namespace %s", namespace.Name)
			err := c.Clientset.CoreV1().
				Secrets(namespace.Name).
				DeleteCollection(ctx, deleteOptions, listOptions)
			if err != nil {
				spinner.Errorf(err, "Unable to delete secrets from namespace %s", namespace.Name)
			}
		}
	}

	spinner.Success()
}

// PackageSecretNeedsWait checks if a package component has a running webhook that needs to be waited on.
func (c *Cluster) PackageSecretNeedsWait(deployedPackage *types.DeployedPackage, component types.ZarfComponent, skipWebhooks bool) (needsWait bool, waitSeconds int, hookName string) {

	// Skip checking webhook status when '--skip-webhooks' flag is provided and for YOLO packages
	if skipWebhooks || deployedPackage == nil || deployedPackage.Data.Metadata.YOLO {
		return false, 0, ""
	}

	// Look for the specified component
	hookMap, componentExists := deployedPackage.ComponentWebhooks[component.Name]
	if !componentExists {
		return false, 0, "" // Component not found, no need to wait
	}

	// Check if there are any "Running" webhooks for the component that we need to wait for
	for hookName, webhook := range hookMap {
		if webhook.Status == types.WebhookStatusRunning {
			return true, webhook.WaitDurationSeconds, hookName
		}
	}

	// If we get here, the component doesn't need to wait for a webhook to run
	return false, 0, ""
}

// RecordPackageDeploymentAndWait records the deployment of a package to the cluster and waits for any webhooks to complete.
func (c *Cluster) RecordPackageDeploymentAndWait(ctx context.Context, pkg types.ZarfPackage, components []types.DeployedComponent, connectStrings types.ConnectStrings, generation int, component types.ZarfComponent, skipWebhooks bool) (deployedPackage *types.DeployedPackage, err error) {
	deployedPackage, err = c.RecordPackageDeployment(ctx, pkg, components, connectStrings, generation)
	if err != nil {
		return nil, err
	}

	packageNeedsWait, waitSeconds, hookName := c.PackageSecretNeedsWait(deployedPackage, component, skipWebhooks)
	// If no webhooks need to complete, we can return immediately.
	if !packageNeedsWait {
		return deployedPackage, nil
	}

	waitDuration := types.DefaultWebhookWaitDuration
	if waitSeconds > 0 {
		waitDuration = time.Duration(waitSeconds) * time.Second
	}

	waitCtx, cancel := context.WithTimeout(ctx, waitDuration)
	defer cancel()

	spinner := message.NewProgressSpinner("Waiting for webhook %q to complete for component %q", hookName, component.Name)
	defer spinner.Stop()

	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-waitCtx.Done():
			return nil, fmt.Errorf("error waiting for webhook %q to complete for component %q: %w", hookName, component.Name, waitCtx.Err())
		case <-timer.C:
			deployedPackage, err = c.GetDeployedPackage(ctx, deployedPackage.Name)
			if err != nil {
				return nil, err
			}

			packageNeedsWait, _, _ = c.PackageSecretNeedsWait(deployedPackage, component, skipWebhooks)
			if !packageNeedsWait {
				spinner.Success()
				return deployedPackage, nil
			}

			timer.Reset(1 * time.Second)
		}
	}
}

// RecordPackageDeployment saves metadata about a package that has been deployed to the cluster.
func (c *Cluster) RecordPackageDeployment(ctx context.Context, pkg types.ZarfPackage, components []types.DeployedComponent, connectStrings types.ConnectStrings, generation int) (deployedPackage *types.DeployedPackage, err error) {
	packageName := pkg.Metadata.Name

	// Generate a secret that describes the package that is being deployed
	secretName := config.ZarfPackagePrefix + packageName
	deployedPackageSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: ZarfNamespaceName,
			Labels: map[string]string{
				config.ZarfManagedByLabel: "zarf",
				ZarfPackageInfoLabel:      packageName,
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{},
	}

	// Attempt to load information about webhooks for the package
	var componentWebhooks map[string]map[string]types.Webhook
	existingPackageSecret, err := c.GetDeployedPackage(ctx, packageName)
	if err != nil {
		message.Debugf("Unable to fetch existing secret for package '%s': %s", packageName, err.Error())
	}
	if existingPackageSecret != nil {
		componentWebhooks = existingPackageSecret.ComponentWebhooks
	}

	deployedPackage = &types.DeployedPackage{
		Name:               packageName,
		CLIVersion:         config.CLIVersion,
		Data:               pkg,
		DeployedComponents: components,
		ConnectStrings:     connectStrings,
		Generation:         generation,
		ComponentWebhooks:  componentWebhooks,
	}

	packageData, err := json.Marshal(deployedPackage)
	if err != nil {
		return nil, err
	}

	// Update the package secret
	deployedPackageSecret.Data = map[string][]byte{"data": packageData}
	var updatedSecret *corev1.Secret
	// TODO: Refactor this
	// NOTE: Kept the original error message even if it is not wrapping the original error
	_, err = c.Clientset.CoreV1().Secrets(deployedPackageSecret.Namespace).Get(ctx, deployedPackageSecret.Name, metav1.GetOptions{})
	if err != nil {
		updatedSecret, err = c.Clientset.CoreV1().Secrets(deployedPackageSecret.Namespace).Create(ctx, deployedPackageSecret, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to record package deployment in secret '%s'", deployedPackageSecret.Name)
		}

	} else {
		updatedSecret, err = c.Clientset.CoreV1().Secrets(deployedPackageSecret.Namespace).Update(ctx, deployedPackageSecret, metav1.UpdateOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to record package deployment in secret '%s'", deployedPackageSecret.Name)
		}
	}

	if err := json.Unmarshal(updatedSecret.Data["data"], &deployedPackage); err != nil {
		return nil, err
	}

	return deployedPackage, nil
}

// EnableRegHPAScaleDown enables the HPA scale down for the Zarf Registry.
func (c *Cluster) EnableRegHPAScaleDown(ctx context.Context) error {
	hpa, err := c.Clientset.AutoscalingV2().HorizontalPodAutoscalers(ZarfNamespaceName).Get(ctx, "zarf-docker-registry", metav1.GetOptions{})
	if err != nil {
		return err
	}

	// Enable HPA scale down.
	policy := autoscalingV2.MinChangePolicySelect
	hpa.Spec.Behavior.ScaleDown.SelectPolicy = &policy

	// Save the HPA changes.
	_, err = c.Clientset.AutoscalingV2().HorizontalPodAutoscalers(hpa.Namespace).Update(ctx, hpa, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

// DisableRegHPAScaleDown disables the HPA scale down for the Zarf Registry.
func (c *Cluster) DisableRegHPAScaleDown(ctx context.Context) error {
	hpa, err := c.Clientset.AutoscalingV2().HorizontalPodAutoscalers(ZarfNamespaceName).Get(ctx, "zarf-docker-registry", metav1.GetOptions{})
	if err != nil {
		return err
	}

	// Disable HPA scale down.
	policy := autoscalingV2.DisabledPolicySelect
	hpa.Spec.Behavior.ScaleDown.SelectPolicy = &policy

	// Save the HPA changes.
	_, err = c.Clientset.AutoscalingV2().HorizontalPodAutoscalers(hpa.Namespace).Update(ctx, hpa, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

// GetInstalledChartsForComponent returns any installed Helm Charts for the provided package component.
func (c *Cluster) GetInstalledChartsForComponent(ctx context.Context, packageName string, component types.ZarfComponent) (installedCharts []types.InstalledChart, err error) {
	deployedPackage, err := c.GetDeployedPackage(ctx, packageName)
	if err != nil {
		return installedCharts, err
	}

	for _, deployedComponent := range deployedPackage.DeployedComponents {
		if deployedComponent.Name == component.Name {
			installedCharts = append(installedCharts, deployedComponent.InstalledCharts...)
		}
	}

	return installedCharts, nil
}
