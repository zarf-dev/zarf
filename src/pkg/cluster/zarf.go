// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	autoscalingV2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
)

// GetDeployedZarfPackages gets metadata information about packages that have been deployed to the cluster.
// We determine what packages have been deployed to the cluster by looking for specific secrets in the Zarf namespace.
// Returns a list of DeployedPackage structs and a list of errors.
func (c *Cluster) GetDeployedZarfPackages(ctx context.Context) ([]types.DeployedPackage, error) {
	// Get the secrets that describe the deployed packages
	listOpts := metav1.ListOptions{LabelSelector: ZarfPackageInfoLabel}
	secrets, err := c.Clientset.CoreV1().Secrets(ZarfNamespaceName).List(ctx, listOpts)
	if err != nil {
		return nil, err
	}

	errs := []error{}
	deployedPackages := []types.DeployedPackage{}
	for _, secret := range secrets.Items {
		if !strings.HasPrefix(secret.Name, config.ZarfPackagePrefix) {
			continue
		}
		var deployedPackage types.DeployedPackage
		// Process the k8s secret into our internal structs
		err := json.Unmarshal(secret.Data["data"], &deployedPackage)
		if err != nil {
			errs = append(errs, fmt.Errorf("unable to unmarshal the secret %s/%s", secret.Namespace, secret.Name))
			continue
		}
		deployedPackages = append(deployedPackages, deployedPackage)
	}

	return deployedPackages, errors.Join(errs...)
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
		LabelSelector: ZarfManagedByLabel + "=zarf",
	}

	namespaceList, err := c.Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		spinner.Errorf(err, "Unable to get k8s namespaces")
	} else {
		for _, namespace := range namespaceList.Items {
			if _, ok := namespace.Labels[AgentLabel]; ok {
				spinner.Updatef("Removing Zarf Agent label for namespace %s", namespace.Name)
				delete(namespace.Labels, AgentLabel)
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

// RecordPackageDeployment saves metadata about a package that has been deployed to the cluster.
func (c *Cluster) RecordPackageDeployment(ctx context.Context, pkg types.ZarfPackage, components []types.DeployedComponent, connectStrings types.ConnectStrings) (deployedPackage *types.DeployedPackage, err error) {
	packageName := pkg.Metadata.Name

	deployedPackage = &types.DeployedPackage{
		Name:               packageName,
		CLIVersion:         config.CLIVersion,
		Data:               pkg,
		DeployedComponents: components,
		ConnectStrings:     connectStrings,
	}

	packageData, err := json.Marshal(deployedPackage)
	if err != nil {
		return nil, err
	}

	// Update the package secret
	deployedPackageSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.ZarfPackagePrefix + packageName,
			Namespace: ZarfNamespaceName,
			Labels: map[string]string{
				ZarfManagedByLabel:   "zarf",
				ZarfPackageInfoLabel: packageName,
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"data": packageData,
		},
	}
	updatedSecret, err := func() (*corev1.Secret, error) {
		secret, err := c.Clientset.CoreV1().Secrets(deployedPackageSecret.Namespace).Create(ctx, deployedPackageSecret, metav1.CreateOptions{})
		if err != nil && !kerrors.IsAlreadyExists(err) {
			return nil, err
		}
		if err == nil {
			return secret, nil
		}
		secret, err = c.Clientset.CoreV1().Secrets(deployedPackageSecret.Namespace).Update(ctx, deployedPackageSecret, metav1.UpdateOptions{})
		if err != nil {
			return nil, err
		}
		return secret, nil
	}()
	if err != nil {
		return nil, fmt.Errorf("failed to record package deployment in secret '%s'", deployedPackageSecret.Name)
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
	policy := autoscalingV2.MinChangePolicySelect
	hpa.Spec.Behavior.ScaleDown.SelectPolicy = &policy
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
	policy := autoscalingV2.DisabledPolicySelect
	hpa.Spec.Behavior.ScaleDown.SelectPolicy = &policy
	_, err = c.Clientset.AutoscalingV2().HorizontalPodAutoscalers(hpa.Namespace).Update(ctx, hpa, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}
