// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package v1alpha1

import (
	"github.com/zarf-dev/zarf/src/api/generic"
)

// ConvertToHub converts a v1alpha1.ZarfPackage to the internal hub type.
func ConvertToHub(in ZarfPackage) generic.ZarfPackage {
	return generic.ZarfPackage{
		Metadata: ConvertMetadataToHub(in.Metadata),
	}
}

// ConvertFromHub converts the internal hub type to v1alpha1.ZarfPackage.
func ConvertFromHub(in generic.ZarfPackage) ZarfPackage {
	return ZarfPackage{
		Metadata: ConvertMetadataFromHub(in.Metadata),
	}
}

// ConvertMetadataToHub converts v1alpha1 metadata to the hub type.
func ConvertMetadataToHub(in ZarfMetadata) generic.ZarfMetadata {
	out := generic.ZarfMetadata{
		Name:                   in.Name,
		Description:            in.Description,
		Version:                in.Version,
		Uncompressed:           in.Uncompressed,
		Architecture:           in.Architecture,
		AggregateChecksum:      in.AggregateChecksum,
		Annotations:            in.Annotations,
		AllowNamespaceOverride: in.AllowNamespaceOverride,
	}

	// Convert YOLO (v1alpha1) to Airgap (hub/v1beta1)
	// YOLO=true means Airgap=false, YOLO=false means Airgap=true
	if in.YOLO {
		airgap := false
		out.Airgap = &airgap
	} else {
		airgap := true
		out.Airgap = &airgap
	}

	// Store removed fields in private fields for backwards compatibility
	out.Url = in.URL
	// out.SetImage(in.Image)
	// out.SetAuthors(in.Authors)
	// out.SetDocumentation(in.Documentation)
	// out.SetSource(in.Source)
	// out.SetVendor(in.Vendor)

	return out
}

// ConvertMetadataFromHub converts the hub type to v1alpha1 metadata.
func ConvertMetadataFromHub(in generic.ZarfMetadata) ZarfMetadata {
	out := ZarfMetadata{
		Name:                   in.Name,
		Description:            in.Description,
		Version:                in.Version,
		Uncompressed:           in.Uncompressed,
		Architecture:           in.Architecture,
		AggregateChecksum:      in.AggregateChecksum,
		Annotations:            in.Annotations,
		AllowNamespaceOverride: in.AllowNamespaceOverride,
	}

	// Convert Airgap (hub/v1beta1) to YOLO (v1alpha1)
	// Airgap=false means YOLO=true, Airgap=true means YOLO=false
	if in.Airgap != nil {
		out.YOLO = !*in.Airgap
	}

	// Restore removed fields from private fields
	out.URL = in.Url
	// out.Image = in.GetImage()
	// out.Authors = in.GetAuthors()
	// out.Documentation = in.GetDocumentation()
	// out.Source = in.GetSource()
	// out.Vendor = in.GetVendor()

	return out
}
