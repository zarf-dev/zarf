// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package v1beta1

import (
	"github.com/zarf-dev/zarf/src/api/generic"
)

// ConvertToGeneric converts a v1beta1.ZarfPackage to the internal hub type.
func ConvertToGeneric(in ZarfPackage) generic.ZarfPackage {
	return generic.ZarfPackage{
		Metadata: ConvertMetadataToHub(in.Metadata),
	}
}

// ConvertFromGeneric converts the internal hub type to v1beta1.ZarfPackage.
func ConvertFromGeneric(in generic.ZarfPackage) ZarfPackage {
	return ZarfPackage{
		Metadata:   ConvertMetadataFromHub(in.Metadata),
		Components: convertComponentsFromGeneric(in),
	}
}

func convertComponentsFromGeneric(in generic.ZarfPackage) []ZarfComponent {
	out := []ZarfComponent{}
	for _, inComp := range in.Components {
		outComp := ZarfComponent{}
		outComp.deprecatedGroup = inComp.DeprecatedGroup
		out = append(out, outComp)
	}
	return out
}

// ConvertMetadataToHub converts v1beta1 metadata to the hub type.
func ConvertMetadataToHub(in ZarfMetadata) generic.ZarfMetadata {
	return generic.ZarfMetadata{
		Name:                   in.Name,
		Description:            in.Description,
		Version:                in.Version,
		Uncompressed:           in.Uncompressed,
		Architecture:           in.Architecture,
		Airgap:                 in.Airgap,
		Annotations:            in.Annotations,
		AllowNamespaceOverride: in.AllowNamespaceOverride,
	}
}

// ConvertMetadataFromHub converts the hub type to v1beta1 metadata.
func ConvertMetadataFromHub(in generic.ZarfMetadata) ZarfMetadata {
	return ZarfMetadata{
		Name:                   in.Name,
		Description:            in.Description,
		Version:                in.Version,
		Uncompressed:           in.Uncompressed,
		Architecture:           in.Architecture,
		Airgap:                 in.Airgap,
		Annotations:            in.Annotations,
		AllowNamespaceOverride: in.AllowNamespaceOverride,
		// Note: Removed fields (url, image, authors, etc.) are not included in v1beta1.
		// They are preserved in the hub type only for v1alpha1 conversions.
	}
}
