// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bigbang contains the logic for installing Big Bang and Flux
package bigbang

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/defenseunicorns/pkg/helpers/v2"
	fluxHelmCtrl "github.com/fluxcd/helm-controller/api/v2beta1"
	fluxSrcCtrl "github.com/fluxcd/source-controller/api/v1beta2"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/internal/packager/helm"
	"github.com/zarf-dev/zarf/src/internal/packager/kustomize"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/variables"
	"helm.sh/helm/v3/pkg/chartutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

// Default location for pulling Big Bang.
const (
	bb                   = "bigbang"
	bbRepo               = "https://repo1.dso.mil/big-bang/bigbang.git"
	bbMinRequiredVersion = "1.54.0"
)

var tenMins = metav1.Duration{
	Duration: 10 * time.Minute,
}

func getValuesFromManifest(valuesFileManifest string) (string, error) {
	file, err := os.ReadFile(valuesFileManifest)
	if err != nil {
		return "", err
	}
	var resource unstructured.Unstructured
	if err := yaml.Unmarshal(file, &resource); err != nil {
		return "", err
	}
	if resource.GetKind() != "Secret" && resource.GetKind() != "ConfigMap" {
		return "", errors.New("values manifests must be a Secret or ConfigMap")
	}
	data, found, err := unstructured.NestedStringMap(resource.Object, "data")
	if err != nil || !found {
		data, found, err = unstructured.NestedStringMap(resource.Object, "stringData")
		if err != nil || !found {
			return "", fmt.Errorf("failed to get data from resource: %w", err)
		}
	}
	valuesYaml, found := data["values.yaml"]
	if !found {
		return "", errors.New("values.yaml key must exist in data")
	}
	return valuesYaml, nil
}

// Create creates a Zarf.yaml file for a big bang package
func Create(ctx context.Context, baseDir string, version string, valuesFileManifests []string, skipFlux bool, repo string, airgap bool) error {
	bbComponent := v1alpha1.ZarfComponent{Name: "bigbang", Required: helpers.BoolPtr(true)}
	fluxComponent := v1alpha1.ZarfComponent{Name: "flux", Required: helpers.BoolPtr(true)}
	pkg := v1alpha1.ZarfPackage{
		Kind:       v1alpha1.ZarfPackageConfig,
		APIVersion: v1alpha1.APIVersion,
		Metadata: v1alpha1.ZarfMetadata{
			Name: "bigbang",
			YOLO: !airgap,
		},
		Components: []v1alpha1.ZarfComponent{},
	}

	tmpDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return err
	}
	defer os.Remove(tmpDir)

	valuesFiles := []string{}
	for idx, valuesFile := range valuesFileManifests {
		valuesYaml, err := getValuesFromManifest(valuesFile)
		if err != nil {
			return err
		}
		valuesFilePath := filepath.Join(tmpDir, fmt.Sprintf("values-%d.yaml", idx))
		if err := os.WriteFile(valuesFilePath, []byte(valuesYaml), helpers.ReadWriteUser); err != nil {
			return err
		}
		valuesFiles = append(valuesFiles, valuesFilePath)
	}

	validVersionResponse, err := isValidVersion(version)

	if err != nil {
		return fmt.Errorf("invalid version %s: %w", version, err)
	}
	if !validVersionResponse {
		return fmt.Errorf("version %s must be at least %s", version, bbMinRequiredVersion)
	}

	// By default, we want to deploy flux.
	if !skipFlux {
		fluxBaseDir := filepath.Join(baseDir, "flux")
		err := getFluxManifest(fluxBaseDir, "kustomization.yaml", repo, version)
		if err != nil {
			return err
		}

		err = getFluxManifest(fluxBaseDir, "gotk-components.yaml", repo, version)
		if err != nil {
			return err
		}
		fluxFilePath := filepath.Join(fluxBaseDir, "bb-flux.yaml")
		if err := kustomize.Build(fluxBaseDir, fluxFilePath, true); err != nil {
			return fmt.Errorf("unable to build kustomization: %w", err)
		}

		fluxManifest := v1alpha1.ZarfManifest{
			Name:      "flux-system",
			Namespace: "flux-system",
			Files:     []string{fluxFilePath},
		}

		if airgap {
			images, err := readFluxImages(fluxFilePath)
			if err != nil {
				return nil
			}
			// Add the images to the list of images to be pulled down by Zarf.
			fluxComponent.Images = append(fluxComponent.Images, images...)
		}

		fluxComponent.Manifests = append(fluxComponent.Manifests, fluxManifest)
	}

	bbRepo := fmt.Sprintf("%s@%s", repo, version)

	// Configure helm to pull down the Big Bang chart.
	helmCfg := helm.New(
		v1alpha1.ZarfChart{
			Name:        bb,
			Namespace:   bb,
			URL:         bbRepo,
			Version:     version,
			ValuesFiles: valuesFiles,
			GitPath:     "./chart",
		},
		path.Join(tmpDir, bb),
		path.Join(tmpDir, bb, "values"),
		helm.WithVariableConfig(&variables.VariableConfig{}),
	)

	// Download the chart from Git and save it to a temporary directory.
	err = helmCfg.PackageChartFromGit(ctx, "")
	if err != nil {
		return fmt.Errorf("unable to download Big Bang Chart: %w", err)
	}

	// Template the chart so we can see what GitRepositories are being referenced in the
	// manifests created with the provided Helm.
	template, _, err := helmCfg.TemplateChart(ctx)
	if err != nil {
		return fmt.Errorf("unable to template Big Bang Chart: %w", err)
	}

	// Add the Big Bang repo to the list of repos to be pulled down by Zarf.
	if airgap {
		bbRepo := fmt.Sprintf("%s@%s", repo, version)
		bbComponent.Repos = append(bbComponent.Repos, bbRepo)
	}

	// Parse the template for GitRepository objects and add them to the list of repos to be pulled down by Zarf.
	gitRepos, hrDependencies, hrValues, err := findBBResources(template)
	if err != nil {
		return fmt.Errorf("unable to find Big Bang resources: %w", err)
	}
	if airgap {
		for _, gitRepo := range gitRepos {
			bbComponent.Repos = append(bbComponent.Repos, gitRepo)
		}
	}

	// Generate a list of HelmReleases that need to be deployed in order.
	dependencies := []utils.Dependency{}
	for _, hrDep := range hrDependencies {
		dependencies = append(dependencies, hrDep)
	}
	namespacedHelmReleaseNames, err := utils.SortDependencies(dependencies)
	if err != nil {
		return fmt.Errorf("unable to sort Big Bang HelmReleases: %w", err)
	}

	// Add wait actions for each of the helm releases in generally the order they should be deployed.
	for _, hrNamespacedName := range namespacedHelmReleaseNames {
		hr := hrDependencies[hrNamespacedName]
		healthCheck := v1alpha1.NamespacedObjectKindReference{
			APIVersion: "v1",
			Kind:       "HelmRelease",
			Name:       hr.Metadata.Name,
			Namespace:  hr.Metadata.Namespace,
		}

		// TODO, ask radius method what's going on here

		// In Big Bang the metrics-server is a special case that only deploy if needed.
		// The check it, we need to look for the existence of APIService instead of the HelmRelease, which
		// may not ever be created. See links below for more details.
		// https://repo1.dso.mil/big-bang/bigbang/-/blob/1.54.0/chart/templates/metrics-server/helmrelease.yaml
		// if hr.Metadata.Name == "metrics-server" {
		// 	action.Description = "K8s metric server to exist or be deployed by Big Bang"
		// 	action.Wait.Cluster = &v1alpha1.ZarfComponentActionWaitCluster{
		// 		Kind: "APIService",
		// 		// https://github.com/kubernetes-sigs/metrics-server#compatibility-matrix
		// 		Name: "v1beta1.metrics.k8s.io",
		// 	}
		// }

		bbComponent.HealthChecks = append(bbComponent.HealthChecks, healthCheck)
	}

	t := true
	failureGeneral := []string{
		"get nodes -o wide",
		"get hr -n bigbang",
		"get gitrepo -n bigbang",
		"get pods -A",
	}
	failureDebug := []string{
		"describe hr -n bigbang",
		"describe gitrepo -n bigbang",
		"describe pods -A",
		"describe nodes",
		"get events -A",
	}

	// Add onFailure actions with additional troubleshooting information.
	for _, cmd := range failureGeneral {
		bbComponent.Actions.OnDeploy.OnFailure = append(bbComponent.Actions.OnDeploy.OnFailure, v1alpha1.ZarfComponentAction{
			Cmd: fmt.Sprintf("./zarf tools kubectl %s", cmd),
		})
	}

	for _, cmd := range failureDebug {
		bbComponent.Actions.OnDeploy.OnFailure = append(bbComponent.Actions.OnDeploy.OnFailure, v1alpha1.ZarfComponentAction{
			Mute:        &t,
			Description: "Storing debug information to the log for troubleshooting.",
			Cmd:         fmt.Sprintf("./zarf tools kubectl %s", cmd),
		})
	}

	// Add a pre-remove action to suspend the Big Bang HelmReleases to prevent reconciliation during removal.
	bbComponent.Actions.OnRemove.Before = append(bbComponent.Actions.OnRemove.Before, v1alpha1.ZarfComponentAction{
		Description: "Suspend Big Bang HelmReleases to prevent reconciliation during removal.",
		Cmd:         `./zarf tools kubectl patch helmrelease -n bigbang bigbang --type=merge -p '{"spec":{"suspend":true}}'`,
	})

	// Select the images needed to support the repos for this configuration of Big Bang.
	if airgap {
		for _, hr := range hrDependencies {
			namespacedName := getNamespacedNameFromMeta(hr.Metadata)
			gitRepo := gitRepos[hr.NamespacedSource]
			values := hrValues[namespacedName]

			images, err := findImagesForBBChartRepo(ctx, gitRepo, values)
			if err != nil {
				return fmt.Errorf("unable to find images for chart repo: %w", err)
			}

			bbComponent.Images = append(bbComponent.Images, images...)
		}

		// Make sure the list of images is unique.
		bbComponent.Images = helpers.Unique(bbComponent.Images)
	}

	manifestDir := filepath.Join(baseDir, "manifests")

	err = os.Mkdir(manifestDir, helpers.ReadWriteExecuteUser)
	if err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}

	manifests := []v1alpha1.ZarfManifest{}

	manifest, err := addBigBangManifests(airgap, manifestDir, valuesFileManifests, version, repo)
	if err != nil {
		return err
	}

	// Add the Big Bang manifests to the list of manifests to be pulled down by Zarf.
	manifests = append(manifests, manifest)

	// Prepend the Big Bang manifests to the list of manifests to be pulled down by Zarf.
	// This is done so that the Big Bang manifests are deployed first.
	bbComponent.Manifests = append(manifests, bbComponent.Manifests...)

	pkg.Components = append(pkg.Components, fluxComponent, bbComponent)

	err = utils.WriteYaml(filepath.Join(baseDir, "zarf.yaml"), pkg, helpers.ReadWriteUser)
	if err != nil {
		return err
	}

	return nil
}

// isValidVersion check if the version is 1.54.0 or greater.
func isValidVersion(version string) (bool, error) {
	specifiedVersion, err := semver.NewVersion(version)

	if err != nil {
		return false, err
	}
	minRequiredVersion, err := semver.NewVersion(bbMinRequiredVersion)
	if err != nil {
		return false, err
	}
	// Evaluating pre-releases too
	c, err := semver.NewConstraint(fmt.Sprintf(">= %s-0", minRequiredVersion))
	if err != nil {
		return false, err
	}
	return c.Check(specifiedVersion), nil
}

// findBBResources takes a list of yaml objects (as a string) and
// parses it for GitRepository objects that it then parses
// to return the list of git repos and tags needed.
func findBBResources(t string) (map[string]string, map[string]HelmReleaseDependency, map[string]map[string]interface{}, error) {
	// Break the template into separate resources.
	yamls, err := utils.SplitYAMLToString([]byte(t))
	if err != nil {
		return nil, nil, nil, err
	}

	gitRepos := map[string]string{}
	helmReleaseDeps := map[string]HelmReleaseDependency{}
	helmReleaseValues := map[string]map[string]interface{}{}
	secrets := map[string]corev1.Secret{}
	configMaps := map[string]corev1.ConfigMap{}

	for _, y := range yamls {
		var u unstructured.Unstructured

		if err := yaml.Unmarshal([]byte(y), &u); err != nil {
			return nil, nil, nil, err
		}

		switch u.GetKind() {
		case fluxHelmCtrl.HelmReleaseKind:
			var h fluxHelmCtrl.HelmRelease
			if err := yaml.Unmarshal([]byte(y), &h); err != nil {
				return nil, nil, nil, err
			}

			var deps []string
			for _, d := range h.Spec.DependsOn {
				depNamespacedName := getNamespacedNameFromStr(d.Namespace, d.Name)
				deps = append(deps, depNamespacedName)
			}

			namespacedName := getNamespacedNameFromMeta(h.ObjectMeta)
			srcNamespacedName := getNamespacedNameFromStr(h.Spec.Chart.Spec.SourceRef.Namespace, h.Spec.Chart.Spec.SourceRef.Name)

			helmReleaseDeps[namespacedName] = HelmReleaseDependency{
				Metadata:               h.ObjectMeta,
				NamespacedDependencies: deps,
				NamespacedSource:       srcNamespacedName,
				ValuesFrom:             h.Spec.ValuesFrom,
			}

		case fluxSrcCtrl.GitRepositoryKind:
			var g fluxSrcCtrl.GitRepository
			if err := yaml.Unmarshal([]byte(y), &g); err != nil {
				return nil, nil, nil, err
			}

			if g.Spec.URL != "" {
				ref := "master"
				switch {
				case g.Spec.Reference.Commit != "":
					ref = g.Spec.Reference.Commit
				case g.Spec.Reference.SemVer != "":
					ref = g.Spec.Reference.SemVer
				case g.Spec.Reference.Tag != "":
					ref = g.Spec.Reference.Tag
				case g.Spec.Reference.Branch != "":
					ref = g.Spec.Reference.Branch
				}

				namespacedName := getNamespacedNameFromMeta(g.ObjectMeta)
				gitRepos[namespacedName] = fmt.Sprintf("%s@%s", g.Spec.URL, ref)
			}

		case "Secret":
			var s corev1.Secret
			if err := yaml.Unmarshal([]byte(y), &s); err != nil {
				return nil, nil, nil, err
			}

			namespacedName := getNamespacedNameFromMeta(s.ObjectMeta)
			secrets[namespacedName] = s

		case "ConfigMap":
			var c corev1.ConfigMap
			if err := yaml.Unmarshal([]byte(y), &c); err != nil {
				return nil, nil, nil, err
			}

			namespacedName := getNamespacedNameFromMeta(c.ObjectMeta)
			configMaps[namespacedName] = c
		}
	}

	for _, hr := range helmReleaseDeps {
		namespacedName := getNamespacedNameFromMeta(hr.Metadata)
		values, err := composeValues(hr, secrets, configMaps)
		if err != nil {
			return nil, nil, nil, err
		}
		helmReleaseValues[namespacedName] = values
	}

	return gitRepos, helmReleaseDeps, helmReleaseValues, nil
}

// addBigBangManifests creates the manifests component for deploying Big Bang.
func addBigBangManifests(airgap bool, manifestDir string, valuesFiles []string, version string, repo string) (v1alpha1.ZarfManifest, error) {
	// Create a manifest component that we add to the zarf package for bigbang.
	manifest := v1alpha1.ZarfManifest{
		Name:      bb,
		Namespace: bb,
	}

	// Helper function to marshal and write a manifest and add it to the component.
	addManifest := func(name string, data any) error {
		path := path.Join(manifestDir, name)
		out, err := yaml.Marshal(data)
		if err != nil {
			return err
		}

		if err := os.WriteFile(path, out, helpers.ReadWriteUser); err != nil {
			return err
		}

		manifest.Files = append(manifest.Files, path)
		return nil
	}

	fluxGitRepo, err := manifestGitRepo(version, repo)
	if err != nil {
		return v1alpha1.ZarfManifest{}, err
	}

	// Create the GitRepository manifest.
	if err := addManifest("bb-ext-gitrepository.yaml", fluxGitRepo); err != nil {
		return v1alpha1.ZarfManifest{}, err
	}

	var hrValues []fluxHelmCtrl.ValuesReference

	// Only include the zarf-credentials secret if in airgap mode
	if airgap {
		zarfCredsManifest, err := manifestZarfCredentials(version)
		if err != nil {
			return manifest, err
		}
		// Create the zarf-credentials secret manifest.
		if err := addManifest("bb-ext-zarf-credentials.yaml", zarfCredsManifest); err != nil {
			return manifest, err
		}

		// Create the list of values manifests starting with zarf-credentials.
		hrValues = []fluxHelmCtrl.ValuesReference{{
			Kind: "Secret",
			Name: "zarf-credentials",
		}}
	}

	// Loop through the valuesFrom list and create a manifest for each.s
	for _, valuesFile := range valuesFiles {
		// Get values file name, make sure it's a secret and add it here
		file, err := os.ReadFile(valuesFile)
		if err != nil {
			return v1alpha1.ZarfManifest{}, err
		}
		var resource unstructured.Unstructured
		if err := yaml.Unmarshal(file, &resource); err != nil {
			return v1alpha1.ZarfManifest{}, err
		}

		manifest.Files = append(manifest.Files, valuesFile)

		// Add it to the list of valuesFrom for the HelmRelease
		hrValues = append(hrValues, fluxHelmCtrl.ValuesReference{
			Kind: resource.GetKind(),
			Name: resource.GetName(),
		})
	}

	if err := addManifest("bb-ext-helmrelease.yaml", manifestHelmRelease(hrValues)); err != nil {
		return v1alpha1.ZarfManifest{}, err
	}

	return manifest, nil
}

// findImagesForBBChartRepo finds and returns the images for the Big Bang chart repo
func findImagesForBBChartRepo(ctx context.Context, repo string, values chartutil.Values) (images []string, err error) {
	matches := strings.Split(repo, "@")
	if len(matches) < 2 {
		return images, fmt.Errorf("cannot convert git repo %s to helm chart without a version tag", repo)
	}

	spinner := message.NewProgressSpinner("Discovering images in %s", repo)
	defer spinner.Stop()

	gitPath, err := helm.DownloadChartFromGitToTemp(ctx, repo)
	if err != nil {
		return images, err
	}
	defer os.RemoveAll(gitPath)

	// Set the directory for the chart
	chartPath := filepath.Join(gitPath, "chart")

	images, err = helm.FindAnnotatedImagesForChart(chartPath, values)
	if err != nil {
		return images, err
	}

	spinner.Success()

	return images, err
}
