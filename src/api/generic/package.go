// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package convert holds the internal hub types for Zarf package conversions.
// These types are used solely for converting between different API versions.
package generic

// ZarfPackage is the hub type for converting between package versions.
type ZarfPackage struct {
	Metadata   ZarfMetadata
	Components []ZarfComponent
}

// ZarfMetadata lists information about the current ZarfPackage.
// This is the internal hub representation used for converting between API versions.
type ZarfMetadata struct {
	// Common fields present in all versions
	Name                   string
	Description            string
	Version                string
	Uncompressed           bool
	Architecture           string
	AggregateChecksum      string
	Annotations            map[string]string
	AllowNamespaceOverride *bool

	// Airgap represents the canonical form (v1beta1+)
	// In v1alpha1, this is called YOLO and the value is inverted
	// Airgap=true means YOLO=false, Airgap=false means YOLO=true
	Airgap *bool

	// Fields below are removed in v1beta1 but kept for backwards compatibility
	// These are stored as private fields so they don't appear in the v1beta1 schema

	// url (removed in v1beta1, migrated to annotations["url"])
	Url string

	// image (removed in v1beta1, migrated to annotations["image"])
	Image string

	// authors (removed in v1beta1, migrated to annotations["authors"])
	Authors string

	// documentation (removed in v1beta1, migrated to annotations["documentation"])
	Documentation string

	// source (removed in v1beta1, migrated to annotations["source"])
	Source string

	// vendor (removed in v1beta1, migrated to annotations["vendor"])
	Vendor string
}

type ZarfComponent struct {
	// The name of the component.
	Name string `json:"name" jsonschema:"pattern=^[a-z0-9][a-z0-9\\-]*$"`

	// Message to include during package deploy describing the purpose of this component.
	Description string `json:"description,omitempty"`

	// Determines the default Y/N state for installing this component on package deploy.
	Default bool `json:"default,omitempty"`

	// Do not prompt user to install this component.
	Required *bool `json:"required,omitempty"`

	// [Deprecated] Create a user selector field based on all components in the same group. This will be removed in Zarf v1.0.0. Consider using 'only.flavor' instead.
	DeprecatedGroup string `json:"group,omitempty" jsonschema:"deprecated=true"`
	// List of OCI images to include in the package.
	Images []string `json:"images,omitempty"`

	// List of git repos to include in the package.
	Repos []string `json:"repos,omitempty"`
}
