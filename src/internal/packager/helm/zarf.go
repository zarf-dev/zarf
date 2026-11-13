// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helm contains operations for working with helm charts.
package helm

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart"
	"helm.sh/helm/v4/pkg/release"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/cli-utils/pkg/object"

	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/internal/healthchecks"
	"github.com/zarf-dev/zarf/src/internal/packager/postrender"
	"github.com/zarf-dev/zarf/src/internal/packager/template"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/state"
	tmpl "github.com/zarf-dev/zarf/src/pkg/template"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/pkg/utils"
)

// UpdateZarfRegistryValues updates the Zarf registry deployment with the new state values
func UpdateZarfRegistryValues(ctx context.Context, opts InstallUpgradeOptions) error {
	pkgs, err := opts.Cluster.GetDeployedZarfPackages(ctx)
	if err != nil {
		return fmt.Errorf("error getting init package: %w", err)
	}
	deployedPkg, initPkg, err := findPackageWithService(pkgs, api.ServiceRegistry)
	if err != nil {
		return fmt.Errorf("error finding init package with registry service: %w", err)
	}
	if initPkg == nil {
		return fmt.Errorf("error finding init package with registry service")
	}
	opts.PkgName = initPkg.Metadata.Name
	if opts.State.RegistryInfo.RegistryMode == state.RegistryModeProxy {
		err := opts.Cluster.InitRegistryCerts(ctx)
		if err != nil {
			return err
		}
	}
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
	chart := api.Chart{
		Namespace:   "zarf",
		ReleaseName: "zarf-docker-registry",
	}
	opts.PostRenderer, err = deployedPostRenderer(ctx, deployedPkg, *initPkg, api.ServiceRegistry, chart, opts)
	if err != nil {
		return err
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
	deployedPkg, initPkg, err := findPackageWithService(pkgs, api.ServiceAgent)
	if err != nil {
		return fmt.Errorf("error finding init package with agent service: %w", err)
	}
	if initPkg == nil {
		return fmt.Errorf("error finding init package with agent service")
	}
	opts.PkgName = initPkg.Metadata.Name
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
		chartAcc, err := chart.NewAccessor(rel.Chart())
		if err != nil {
			return err
		}
		if strings.Contains(chartAcc.Name(), "zarf-agent-zarf-agent") {
			found = true
			chart := api.Chart{
				Namespace:   "zarf",
				ReleaseName: rel.Name(),
			}
			agentValues := map[string]interface{}{}
			if initPkg.GetAPIVersion() == v1alpha1.APIVersion {
				// Older CLIs rotate credentials through post-rendered placeholders.
				// Preserve those placeholders in Helm's reusable values.
				opts.VariableConfig.SetConstants([]api.Constant{
					{
						Name:  "AGENT_IMAGE",
						Value: agentImage.Path,
					},
					{
						Name:  "AGENT_IMAGE_TAG",
						Value: agentImage.Tag,
					},
				})
			} else {
				agentTLS := opts.State.AgentInfo.TLS
				agentValues = map[string]interface{}{
					"caBundle":       base64.StdEncoding.EncodeToString(agentTLS.CA),
					"mutationPolicy": string(opts.State.AgentInfo.MutationPolicy),
					"secret": map[string]interface{}{
						"tlsCrt": base64.StdEncoding.EncodeToString(agentTLS.Cert),
						"tlsKey": base64.StdEncoding.EncodeToString(agentTLS.Key),
					},
				}
			}
			applicationTemplates, err := template.GetZarfTemplates(ctx, initPkg.GetAPIVersion(), api.ServiceAgent, opts.State)
			if err != nil {
				return fmt.Errorf("error setting up the templates: %w", err)
			}
			opts.VariableConfig.SetApplicationTemplates(applicationTemplates)

			opts.PostRenderer, err = deployedPostRenderer(ctx, deployedPkg, *initPkg, api.ServiceAgent, chart, opts)
			if err != nil {
				return err
			}
			err = UpdateReleaseValues(ctx, chart, agentValues, opts)
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

func findPackageWithService(pkgs []state.DeployedPackage, service api.ServiceName) (*state.DeployedPackage, *api.Package, error) {
	for i, deployedPackage := range pkgs {
		definition, err := deployedPackage.Definition()
		if err != nil {
			return nil, nil, err
		}
		for _, component := range definition.Components {
			if component.Service.Name == service {
				return &pkgs[i], &definition, nil
			}
		}
	}
	return nil, nil, nil
}

// deployedPostRenderer resolves the post-renderers the deployed package declares for release so that re-rendering the
// release keeps their changes. pkg is deployed's decoded definition. It returns nil when there are none.
func deployedPostRenderer(ctx context.Context, deployed *state.DeployedPackage, pkg api.Package, service api.ServiceName, release api.Chart, opts InstallUpgradeOptions) (*postrender.Renderer, error) {
	for _, component := range pkg.Components {
		if component.Service.Name != service {
			continue
		}
		for _, chart := range component.Charts {
			if chart.GetReleaseName() != release.GetReleaseName() || chart.Namespace != release.Namespace {
				continue
			}
			if !postrender.NeedsTemplating(chart) {
				return postrender.New(ctx, chart, nil)
			}
			deployConfig, _ := deployed.GetDeployConfig()
			objs, err := tmpl.NewObjects(deployConfig.Values).WithPackage(pkg).WithVariableConfig(opts.VariableConfig).
				WithState(tmpl.StateAccess{State: opts.State, AccessKeys: component.StateAccess})
			if err != nil {
				return nil, fmt.Errorf("chart %q: error building template objects: %w", chart.Name, err)
			}
			r, err := postrender.New(ctx, chart, objs)
			if err != nil {
				// Templates relying on the package's default values fail here because only deploy-time values are recorded.
				return nil, fmt.Errorf("%w (re-rendering for a credential update only has deploy-time values; package default values are not recorded)", err)
			}
			return r, nil
		}
	}
	return nil, nil
}
