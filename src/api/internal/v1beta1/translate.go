// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package v1beta1

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TranslateAlphaPackage translates a v1alpha1.ZarfPackage to a v1beta1.ZarfPackage
func TranslateAlphaPackage(alphaPkg v1alpha1.ZarfPackage) (ZarfPackage, error) {
	var betaPkg ZarfPackage

	// This will set all the fields that are common between v1alpha1 and v1beta1
	jsonData, err := json.Marshal(alphaPkg)
	if err != nil {
		return ZarfPackage{}, fmt.Errorf("failed to marshal v1alpha1 object: %w", err)
	}

	err = json.Unmarshal(jsonData, &betaPkg)
	if err != nil {
		return ZarfPackage{}, fmt.Errorf("failed to unmarshal JSON to v1beta1 object: %w", err)
	}

	betaPkg.APIVersion = APIVersion

	betaPkg.Metadata.Annotations = make(map[string]string)
	if alphaPkg.Metadata.Description != "" {
		betaPkg.Metadata.Annotations["description"] = alphaPkg.Metadata.Description
	}
	if alphaPkg.Metadata.URL != "" {
		betaPkg.Metadata.Annotations["url"] = alphaPkg.Metadata.URL
	}
	if alphaPkg.Metadata.Image != "" {
		betaPkg.Metadata.Annotations["image"] = alphaPkg.Metadata.Image
	}
	if alphaPkg.Metadata.Authors != "" {
		betaPkg.Metadata.Annotations["authors"] = alphaPkg.Metadata.Authors
	}
	if alphaPkg.Metadata.Documentation != "" {
		betaPkg.Metadata.Annotations["documentation"] = alphaPkg.Metadata.Documentation
	}
	if alphaPkg.Metadata.Source != "" {
		betaPkg.Metadata.Annotations["source"] = alphaPkg.Metadata.Source
	}
	if alphaPkg.Metadata.Vendor != "" {
		betaPkg.Metadata.Annotations["vendor"] = alphaPkg.Metadata.Vendor
	}

	if alphaPkg.Metadata.YOLO {
		betaPkg.Metadata.Airgap = helpers.BoolPtr(false)
	}

	betaPkg.Build.AggregateChecksum = alphaPkg.Metadata.AggregateChecksum

	for i := range betaPkg.Components {
		betaPkg.Components[i].Optional = helpers.BoolPtr(!alphaPkg.Components[i].IsRequired())
		for j := range betaPkg.Components[i].Charts {
			oldURL := alphaPkg.Components[i].Charts[j].URL
			if helpers.IsOCIURL(oldURL) {
				betaPkg.Components[i].Charts[j].OCI.URL = oldURL
			} else if strings.HasSuffix(oldURL, ".git") {
				betaPkg.Components[i].Charts[j].Git.URL = oldURL
				betaPkg.Components[i].Charts[j].Git.Path = alphaPkg.Components[i].Charts[j].GitPath
			} else {
				betaPkg.Components[i].Charts[j].Helm.URL = oldURL
				betaPkg.Components[i].Charts[j].Helm.RepoName = alphaPkg.Components[i].Charts[j].RepoName
			}
			betaPkg.Components[i].Charts[j].Local.Path = alphaPkg.Components[i].Charts[j].LocalPath
			betaPkg.Components[i].Charts[j].Wait = helpers.BoolPtr(!alphaPkg.Components[i].Charts[j].NoWait)
		}

		for j := range betaPkg.Components[i].Manifests {
			betaPkg.Components[i].Manifests[j].Wait = helpers.BoolPtr(!alphaPkg.Components[i].Manifests[j].NoWait)
		}
		betaPkg.Components[i].Actions.OnCreate = transformActionSet(betaPkg.Components[i].Actions.OnCreate, alphaPkg.Components[i].Actions.OnCreate)
		betaPkg.Components[i].Actions.OnDeploy = transformActionSet(betaPkg.Components[i].Actions.OnDeploy, alphaPkg.Components[i].Actions.OnDeploy)
		betaPkg.Components[i].Actions.OnRemove = transformActionSet(betaPkg.Components[i].Actions.OnRemove, alphaPkg.Components[i].Actions.OnRemove)
	}

	return betaPkg, nil
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
