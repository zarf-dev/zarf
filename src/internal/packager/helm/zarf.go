// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helm contains operations for working with helm charts.
package helm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/zarf-dev/zarf/src/pkg/state"

	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart"
	"helm.sh/helm/v4/pkg/release"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/cli-utils/pkg/object"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/internal/healthchecks"
	"github.com/zarf-dev/zarf/src/internal/packager/template"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/pkg/utils"
)

// UpdateZarfRegistryValues updates the Zarf registry deployment with the new state values
func UpdateZarfRegistryValues(ctx context.Context, opts InstallUpgradeOptions) error {
	pkgs, err := opts.Cluster.GetDeployedZarfPackages(ctx)
	if err != nil {
		return fmt.Errorf("error getting init package: %w", err)
	}
	initPkgName := findInitPackageWithComponent(pkgs, "zarf-registry")
	if initPkgName == "" {
		return fmt.Errorf("error finding init package with zarf-registry component")
	}
	opts.PkgName = initPkgName
	pushUser, err := utils.GetHtpasswdString(opts.State.RegistryInfo.PushUsername, opts.State.RegistryInfo.PushPassword)
	if err != nil {
		return fmt.Errorf("error generating htpasswd string: %w", err)
	}
	pullUser, err := utils.GetHtpasswdString(opts.State.RegistryInfo.PullUsername, opts.State.RegistryInfo.PullPassword)
	if err != nil {
		return fmt.Errorf("error generating htpasswd string: %w", err)
	}
	registryValues := map[string]interface{}{
		"secrets": map[string]interface{}{
			"htpasswd": fmt.Sprintf("%s\n%s", pushUser, pullUser),
		},
	}
	chart := v1alpha1.ZarfChart{
		Namespace:   "zarf",
		ReleaseName: "zarf-docker-registry",
	}

	err = UpdateReleaseValues(ctx, chart, registryValues, opts)
	if err != nil {
		return fmt.Errorf("error updating the release values: %w", err)
	}

	objs := []object.ObjMetadata{
		{
			GroupKind: schema.GroupKind{
				Group: "apps",
				Kind:  "Deployment",
			},
			Namespace: "zarf",
			Name:      "zarf-docker-registry",
		},
	}
	waitCtx, waitCancel := context.WithTimeout(ctx, 60*time.Second)
	defer waitCancel()
	err = healthchecks.WaitForReady(waitCtx, opts.Cluster.Watcher, objs)
	if err != nil {
		return err
	}
	return nil
}

// UpdateZarfAgentValues updates the Zarf agent deployment with the new state values
func UpdateZarfAgentValues(ctx context.Context, opts InstallUpgradeOptions) error {
	l := logger.From(ctx)

	pkgs, err := opts.Cluster.GetDeployedZarfPackages(ctx)
	if err != nil {
		return fmt.Errorf("error getting init package: %w", err)
	}
	initPkgName := findInitPackageWithComponent(pkgs, "zarf-agent")
	if initPkgName == "" {
		return fmt.Errorf("error finding init package with zarf-agent component")
	}
	opts.PkgName = initPkgName
	deployment, err := opts.Cluster.Clientset.AppsV1().Deployments(state.ZarfNamespaceName).Get(ctx, "agent-hook", metav1.GetOptions{})
	if err != nil {
		return err
	}
	agentImage, err := transform.ParseImageRef(deployment.Spec.Template.Spec.Containers[0].Image)
	if err != nil {
		return err
	}

	// In the event the registry is external and includes subpaths
	// we will remove the subpath from the agent path
	registry := opts.State.RegistryInfo.Address
	parts := strings.Split(registry, "/")
	subPath := strings.Join(parts[1:], "/")
	if subPath != "" {
		agentImage.Path = strings.TrimPrefix(agentImage.Path, fmt.Sprintf("%s/", subPath))
	}

	actionConfig, err := createActionConfig(ctx, state.ZarfNamespaceName)
	if err != nil {
		return err
	}

	// List the releases to find the current agent release name.
	listClient := action.NewList(actionConfig)
	releases, err := listClient.Run()
	if err != nil {
		return fmt.Errorf("unable to list helm releases: %w", err)
	}

	// Ensure we find the release - otherwise this can return without an error and not do anything
	found := false
	for _, releaser := range releases {
		rel, err := release.NewAccessor(releaser)
		if err != nil {
			return err
		}

		// Update the Zarf Agent release with the new values
		// Before the Zarf agent was converted to a Helm chart, the name could differ depending on the name of the init package
		// To stay backwards compatible with these package , we exclude the package name section of the release name
		// FIXME: make sure this is right

		chartAcc, err := chart.NewAccessor(rel.Chart())
		if err != nil {
			return err
		}
		if strings.Contains(chartAcc.Name(), "zarf-agent-zarf-agent") {
			found = true
			chart := v1alpha1.ZarfChart{
				Namespace:   "zarf",
				ReleaseName: rel.Name(),
			}
			opts.VariableConfig.SetConstants([]v1alpha1.Constant{
				{
					Name:  "AGENT_IMAGE",
					Value: agentImage.Path,
				},
				{
					Name:  "AGENT_IMAGE_TAG",
					Value: agentImage.Tag,
				},
			})
			applicationTemplates, err := template.GetZarfTemplates(ctx, "zarf-agent", opts.State)
			if err != nil {
				return fmt.Errorf("error setting up the templates: %w", err)
			}
			opts.VariableConfig.SetApplicationTemplates(applicationTemplates)

			err = UpdateReleaseValues(ctx, chart, map[string]interface{}{}, opts)
			if err != nil {
				return fmt.Errorf("error updating the release values: %w", err)
			}
		}
	}

	if !found {
		return fmt.Errorf("unable to find the Zarf Agent release")
	}

	// Trigger a rolling update for the TLS secret update to take effect.
	// https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#updating-a-deployment
	l.Info("performing a rolling update for the Zarf Agent deployment")

	// Re-fetch the agent deployment before we update since the resourceVersion has changed after updating the Helm release values.
	// Avoids this error: https://github.com/kubernetes/kubernetes/issues/28149
	deployment, err = opts.Cluster.Clientset.AppsV1().Deployments(state.ZarfNamespaceName).Get(ctx, "agent-hook", metav1.GetOptions{})
	if err != nil {
		return err
	}
	if deployment.Spec.Template.Annotations == nil {
		deployment.Spec.Template.Annotations = map[string]string{}
	}
	deployment.Spec.Template.Annotations["zarf.dev/restartedAt"] = time.Now().UTC().Format(time.RFC3339)
	_, err = opts.Cluster.Clientset.AppsV1().Deployments(state.ZarfNamespaceName).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	objs := []object.ObjMetadata{
		{
			GroupKind: schema.GroupKind{
				Group: "apps",
				Kind:  "Deployment",
			},
			Namespace: state.ZarfNamespaceName,
			Name:      "agent-hook",
		},
	}
	waitCtx, waitCancel := context.WithTimeout(ctx, 60*time.Second)
	defer waitCancel()
	err = healthchecks.WaitForReady(waitCtx, opts.Cluster.Watcher, objs)
	if err != nil {
		return err
	}
	return nil
}

func findInitPackageWithComponent(pkgs []state.DeployedPackage, componentName string) string {
	for _, pkg := range pkgs {
		if pkg.Data.Kind == v1alpha1.ZarfInitConfig {
			for _, c := range pkg.Data.Components {
				if c.Name == componentName {
					return pkg.Name
				}
			}
		}
	}
	return ""
}
