// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package types contains all the types used by Zarf.
package types

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TranslateAlphaPackage translates a v1alpha1.ZarfPackage to a v1beta1.ZarfPackage
func TranslateAlphaPackage(alphaPkg v1alpha1.ZarfPackage) (ZarfPackage, error) {
	var zarfPackage ZarfPackage

	// This will set all the fields that are common between v1alpha1 and v1beta1
	jsonData, err := json.Marshal(alphaPkg)
	if err != nil {
		return ZarfPackage{}, fmt.Errorf("failed to marshal v1alpha1 object: %w", err)
	}

	err = json.Unmarshal(jsonData, &zarfPackage)
	if err != nil {
		return ZarfPackage{}, fmt.Errorf("failed to unmarshal JSON to v1beta1 object: %w", err)
	}

	zarfPackage.APIVersion = APIVersion

	zarfPackage.Metadata.Annotations = make(map[string]string)
	if alphaPkg.Metadata.Description != "" {
		zarfPackage.Metadata.Annotations["description"] = alphaPkg.Metadata.Description
	}
	if alphaPkg.Metadata.URL != "" {
		zarfPackage.Metadata.Annotations["url"] = alphaPkg.Metadata.URL
	}
	if alphaPkg.Metadata.Image != "" {
		zarfPackage.Metadata.Annotations["image"] = alphaPkg.Metadata.Image
	}
	if alphaPkg.Metadata.Authors != "" {
		zarfPackage.Metadata.Annotations["authors"] = alphaPkg.Metadata.Authors
	}
	if alphaPkg.Metadata.Documentation != "" {
		zarfPackage.Metadata.Annotations["documentation"] = alphaPkg.Metadata.Documentation
	}
	if alphaPkg.Metadata.Source != "" {
		zarfPackage.Metadata.Annotations["source"] = alphaPkg.Metadata.Source
	}
	if alphaPkg.Metadata.Vendor != "" {
		zarfPackage.Metadata.Annotations["vendor"] = alphaPkg.Metadata.Vendor
	}

	if alphaPkg.Metadata.YOLO {
		zarfPackage.Metadata.Airgap = helpers.BoolPtr(false)
	}

	zarfPackage.Build.AggregateChecksum = alphaPkg.Metadata.AggregateChecksum

	for i := range zarfPackage.Components {
		zarfPackage.Components[i].Optional = helpers.BoolPtr(!alphaPkg.Components[i].IsRequired())
		for j := range zarfPackage.Components[i].Charts {
			oldURL := alphaPkg.Components[i].Charts[j].URL
			if helpers.IsOCIURL(oldURL) {
				zarfPackage.Components[i].Charts[j].OCI.URL = oldURL
			} else if strings.HasSuffix(oldURL, ".git") {
				zarfPackage.Components[i].Charts[j].Git.URL = oldURL
				zarfPackage.Components[i].Charts[j].Git.Path = alphaPkg.Components[i].Charts[j].GitPath
			} else {
				zarfPackage.Components[i].Charts[j].Helm.URL = oldURL
				zarfPackage.Components[i].Charts[j].Helm.RepoName = alphaPkg.Components[i].Charts[j].RepoName
			}
			zarfPackage.Components[i].Charts[j].Local.Path = alphaPkg.Components[i].Charts[j].LocalPath
			zarfPackage.Components[i].Charts[j].Wait = helpers.BoolPtr(!alphaPkg.Components[i].Charts[j].NoWait)
		}

		for j := range zarfPackage.Components[i].Manifests {
			zarfPackage.Components[i].Manifests[j].Wait = helpers.BoolPtr(!alphaPkg.Components[i].Manifests[j].NoWait)
		}
		zarfPackage.Components[i].Actions.OnCreate = transformActionSet(zarfPackage.Components[i].Actions.OnCreate, alphaPkg.Components[i].Actions.OnCreate)
		zarfPackage.Components[i].Actions.OnDeploy = transformActionSet(zarfPackage.Components[i].Actions.OnDeploy, alphaPkg.Components[i].Actions.OnDeploy)
		zarfPackage.Components[i].Actions.OnRemove = transformActionSet(zarfPackage.Components[i].Actions.OnRemove, alphaPkg.Components[i].Actions.OnRemove)
	}

	return zarfPackage, nil
}

func transformActionSet(betaActions ZarfComponentActionSet, alphaActions v1alpha1.ZarfComponentActionSet) ZarfComponentActionSet {
	if alphaActions.Defaults.MaxTotalSeconds != 0 {
		betaActions.Defaults.Timeout = &v1.Duration{Duration: time.Duration(alphaActions.Defaults.MaxTotalSeconds) * time.Second}
	}
	betaActions.Defaults.Retries = alphaActions.Defaults.MaxRetries

	betaActions.After = transformActions(betaActions.After, alphaActions.After)
	betaActions.Before = transformActions(betaActions.Before, alphaActions.Before)
	betaActions.OnFailure = transformActions(betaActions.OnFailure, alphaActions.OnFailure)
	betaActions.OnSuccess = transformActions(betaActions.OnSuccess, alphaActions.OnSuccess)

	return betaActions
}

func transformActions(betaActions []ZarfComponentAction, alphaActions []v1alpha1.ZarfComponentAction) []ZarfComponentAction {
	for i := range betaActions {
		if alphaActions[i].MaxTotalSeconds != nil && *alphaActions[i].MaxTotalSeconds != 0 {
			betaActions[i].Timeout = &v1.Duration{Duration: time.Duration(*alphaActions[i].MaxTotalSeconds) * time.Second}
		}

		if alphaActions[i].MaxRetries != nil {
			betaActions[i].Retries = *alphaActions[i].MaxRetries
		}
	}
	return betaActions
}

// TranslateBetaPackage translates a v1alpha1.ZarfPackage to a v1beta1.ZarfPackage
func TranslateBetaPackage(alphaPkg v1beta1.ZarfPackage) (ZarfPackage, error) {
	var zarfPackage ZarfPackage

	// v1beta1 is a subset of types.ZarfPackage so this is all that is required
	jsonData, err := json.Marshal(alphaPkg)
	if err != nil {
		return ZarfPackage{}, fmt.Errorf("failed to marshal v1alpha1 object: %w", err)
	}

	err = json.Unmarshal(jsonData, &zarfPackage)
	if err != nil {
		return ZarfPackage{}, fmt.Errorf("failed to unmarshal JSON to v1beta1 object: %w", err)
	}
	return zarfPackage, nil
}
