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
	"time"

	autoscalingV2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1ac "k8s.io/client-go/applyconfigurations/core/v1"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/internal/gitea"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/types"
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

	err = errors.Join(errs...)
	if err != nil {
		return nil, err
	}
	return deployedPackages, nil
}

// GetDeployedPackage gets the metadata information about the package name provided (if it exists in the cluster).
// We determine what packages have been deployed to the cluster by looking for specific secrets in the Zarf namespace.
func (c *Cluster) GetDeployedPackage(ctx context.Context, packageName string) (*types.DeployedPackage, error) {
	secret, err := c.Clientset.CoreV1().Secrets(ZarfNamespaceName).Get(ctx, config.ZarfPackagePrefix+packageName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	deployedPackage := &types.DeployedPackage{}
	err = json.Unmarshal(secret.Data["data"], deployedPackage)
	if err != nil {
		return nil, err
	}
	return deployedPackage, nil
}

// UpdateDeployedPackage updates the deployed package metadata.
func (c *Cluster) UpdateDeployedPackage(ctx context.Context, depPkg types.DeployedPackage) error {
	secretName := config.ZarfPackagePrefix + depPkg.Name
	packageSecretData, err := json.Marshal(depPkg)
	if err != nil {
		return err
	}
	packageSecret := v1ac.Secret(secretName, ZarfNamespaceName).
		WithLabels(map[string]string{
			ZarfManagedByLabel:   "zarf",
			ZarfPackageInfoLabel: depPkg.Name,
		}).WithData(map[string][]byte{
		"data": packageSecretData,
	}).WithType(corev1.SecretTypeOpaque)
	_, err = c.Clientset.CoreV1().Secrets(*packageSecret.Namespace).Apply(ctx, packageSecret, metav1.ApplyOptions{Force: true, FieldManager: "zarf"})
	if err != nil {
		return fmt.Errorf("unable to apply the deployed package secret: %w", err)
	}
	return nil
}

// DeleteDeployedPackage removes the metadata for the deployed package.
func (c *Cluster) DeleteDeployedPackage(ctx context.Context, packageName string) error {
	secretName := config.ZarfPackagePrefix + packageName
	err := c.Clientset.CoreV1().Secrets(ZarfNamespaceName).Delete(ctx, secretName, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

// StripZarfLabelsAndSecretsFromNamespaces removes metadata and secrets from existing namespaces no longer manged by Zarf.
func (c *Cluster) StripZarfLabelsAndSecretsFromNamespaces(ctx context.Context) {
	start := time.Now()
	l := logger.From(ctx)
	spinner := message.NewProgressSpinner("Removing zarf metadata & secrets from existing namespaces not managed by Zarf")
	defer spinner.Stop()
	l.Info("removing zarf metadata & secrets from existing namespaces not managed by Zarf")

	deleteOptions := metav1.DeleteOptions{}
	listOptions := metav1.ListOptions{
		LabelSelector: ZarfManagedByLabel + "=zarf",
	}

	// TODO(mkcp): Remove unnecessary nesting w/ else
	namespaceList, err := c.Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		spinner.Errorf(err, "Unable to get k8s namespaces")
		l.Error("unable to get k8s namespaces", "error", err)
	} else {
		for _, namespace := range namespaceList.Items {
			if _, ok := namespace.Labels[AgentLabel]; ok {
				spinner.Updatef("Removing Zarf Agent label for namespace %s", namespace.Name)
				l.Info("removing Zarf Agent label", "namespace", namespace.Name)
				delete(namespace.Labels, AgentLabel)
				namespaceCopy := namespace
				_, err := c.Clientset.CoreV1().Namespaces().Update(ctx, &namespaceCopy, metav1.UpdateOptions{})
				if err != nil {
					// This is not a hard failure, but we should log it
					spinner.Errorf(err, "Unable to update the namespace labels for %s", namespace.Name)
					l.Warn("unable to update the namespace labels", "namespace", namespace.Name, "error", err)
				}
			}

			spinner.Updatef("Removing Zarf secrets for namespace %s", namespace.Name)
			l.Info("removing Zarf secrets", "namespace", namespace.Name)
			err := c.Clientset.CoreV1().
				Secrets(namespace.Name).
				DeleteCollection(ctx, deleteOptions, listOptions)
			if err != nil {
				spinner.Errorf(err, "Unable to delete secrets from namespace %s", namespace.Name)
				l.Error("unable to delete secrets", "namespace", namespace.Name, "error", err)
			}
		}
	}

	spinner.Success()
	l.Debug("done stripping zarf labels and secrets from namespaces", "duration", time.Since(start))
}

// RecordPackageDeployment saves metadata about a package that has been deployed to the cluster.
func (c *Cluster) RecordPackageDeployment(ctx context.Context, pkg v1alpha1.ZarfPackage, components []types.DeployedComponent) (*types.DeployedPackage, error) {
	packageName := pkg.Metadata.Name

	// TODO: This is done for backwards compatibility and could be removed in the future.
	connectStrings := types.ConnectStrings{}
	for _, comp := range components {
		for _, chart := range comp.InstalledCharts {
			for k, v := range chart.ConnectStrings {
				connectStrings[k] = v
			}
		}
	}

	deployedPackage := &types.DeployedPackage{
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

	packageSecretName := fmt.Sprintf("%s%s", config.ZarfPackagePrefix, packageName)
	deployedPackageSecret := v1ac.Secret(packageSecretName, ZarfNamespaceName).
		WithLabels(map[string]string{
			ZarfManagedByLabel:   "zarf",
			ZarfPackageInfoLabel: packageName,
		}).WithType(corev1.SecretTypeOpaque).
		WithData(map[string][]byte{
			"data": packageData,
		})
	updatedSecret, err := c.Clientset.CoreV1().Secrets(*deployedPackageSecret.Namespace).Apply(ctx, deployedPackageSecret, metav1.ApplyOptions{Force: true, FieldManager: "zarf"})
	if err != nil {
		return nil, fmt.Errorf("failed to record package deployment in secret '%s': %w", *deployedPackageSecret.Name, err)
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

// GetInstalledChartsForComponent returns any installed Helm Charts for the provided package component.
func (c *Cluster) GetInstalledChartsForComponent(ctx context.Context, packageName string, component v1alpha1.ZarfComponent) ([]types.InstalledChart, error) {
	deployedPackage, err := c.GetDeployedPackage(ctx, packageName)
	if err != nil {
		return nil, err
	}

	installedCharts := make([]types.InstalledChart, 0)
	for _, deployedComponent := range deployedPackage.DeployedComponents {
		if deployedComponent.Name == component.Name {
			installedCharts = append(installedCharts, deployedComponent.InstalledCharts...)
		}
	}

	return installedCharts, nil
}

// UpdateInternalArtifactServerToken updates the the artifact server token on the internal gitea server and returns it
func (c *Cluster) UpdateInternalArtifactServerToken(ctx context.Context, oldGitServer types.GitServerInfo) (string, error) {
	tunnel, err := c.NewTunnel(ZarfNamespaceName, SvcResource, ZarfGitServerName, "", 0, ZarfGitServerPort)
	if err != nil {
		return "", err
	}
	_, err = tunnel.Connect(ctx)
	if err != nil {
		return "", err
	}
	defer tunnel.Close()
	tunnelURL := tunnel.HTTPEndpoint()
	giteaClient, err := gitea.NewClient(tunnelURL, oldGitServer.PushUsername, oldGitServer.PushPassword)
	if err != nil {
		return "", err
	}
	var newToken string
	err = tunnel.Wrap(func() error {
		newToken, err = giteaClient.CreatePackageRegistryToken(ctx)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return newToken, nil
}

// UpdateInternalGitServerSecret updates the internal gitea server secrets with the new git server info
func (c *Cluster) UpdateInternalGitServerSecret(ctx context.Context, oldGitServer types.GitServerInfo, newGitServer types.GitServerInfo) error {
	tunnel, err := c.NewTunnel(ZarfNamespaceName, SvcResource, ZarfGitServerName, "", 0, ZarfGitServerPort)
	if err != nil {
		return err
	}
	_, err = tunnel.Connect(ctx)
	if err != nil {
		return err
	}
	defer tunnel.Close()
	tunnelURL := tunnel.HTTPEndpoint()
	giteaClient, err := gitea.NewClient(tunnelURL, oldGitServer.PushUsername, oldGitServer.PushPassword)
	if err != nil {
		return err
	}
	err = tunnel.Wrap(func() error {
		err := giteaClient.UpdateGitUser(ctx, newGitServer.PullUsername, newGitServer.PullPassword)
		if err != nil {
			return err
		}
		err = giteaClient.UpdateGitUser(ctx, newGitServer.PushUsername, newGitServer.PushPassword)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

// InternalGitServerExists checks if the Zarf internal git server exists in the cluster.
func (c *Cluster) InternalGitServerExists(ctx context.Context) (bool, error) {
	_, err := c.Clientset.CoreV1().Services(ZarfNamespaceName).Get(ctx, ZarfGitServerName, metav1.GetOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		return false, err
	}
	return !kerrors.IsNotFound(err), nil
}
