// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package v1beta1 holds the definition of the v1beta1 Zarf Package
package v1beta1

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
)

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

	betaPkg.APIVersion = ApiVersion

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
			oldUrl := alphaPkg.Components[i].Charts[j].URL
			if helpers.IsOCIURL(oldUrl) {
				betaPkg.Components[i].Charts[j].OCI.Url = oldUrl
			} else if strings.HasSuffix(oldUrl, ".git") {
				betaPkg.Components[i].Charts[j].Git.Url = oldUrl
				betaPkg.Components[i].Charts[j].Git.Path = alphaPkg.Components[i].Charts[j].GitPath
			} else {
				betaPkg.Components[i].Charts[j].Helm.Url = oldUrl
				betaPkg.Components[i].Charts[j].Helm.RepoName = alphaPkg.Components[i].Charts[j].RepoName
			}

			betaPkg.Components[i].Charts[j].Local.Path = alphaPkg.Components[i].Charts[j].LocalPath
		}
	}

	return betaPkg, nil
}
