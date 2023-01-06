// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package types contains all the types used by Zarf.
package types

// ZarfPackage the top-level structure of a Zarf config file.
type ZarfPackage struct {
	Kind       string                `json:"kind" jsonschema:"description=The kind of Zarf package,enum=ZarfInitConfig,enum=ZarfPackageConfig,default=ZarfPackageConfig"`
	Metadata   ZarfMetadata          `json:"metadata,omitempty" jsonschema:"description=Package metadata"`
	Build      ZarfBuildData         `json:"build,omitempty" jsonschema:"description=Zarf-generated package build data"`
	Components []ZarfComponent       `json:"components" jsonschema:"description=List of components to deploy in this package"`
	Variables  []ZarfPackageVariable `json:"variables,omitempty" jsonschema:"description=Variable template values applied on deploy for K8s resources"`
	Constants  []ZarfPackageConstant `json:"constants,omitempty" jsonschema:"description=Constant template values applied on deploy for K8s resources"`
}

// ZarfMetadata lists information about the current ZarfPackage.
type ZarfMetadata struct {
	Name         string `json:"name" jsonschema:"description=Name to identify this Zarf package,pattern=^[a-z0-9\\-]+$"`
	Description  string `json:"description,omitempty" jsonschema:"description=Additional information about this package"`
	Version      string `json:"version,omitempty" jsonschema:"description=Generic string to track the package version by a package author"`
	URL          string `json:"url,omitempty" jsonschema:"description=Link to package information when online"`
	Image        string `json:"image,omitempty" jsonschema:"description=An image URL to embed in this package for future Zarf UI listing"`
	Uncompressed bool   `json:"uncompressed,omitempty" jsonschema:"description=Disable compression of this package"`
	Architecture string `json:"architecture,omitempty" jsonschema:"description=The target cluster architecture of this package"`
	YOLO         bool   `json:"yolo,omitempty" jsonschema:"description=Yaml OnLy Online (YOLO): True enables deploying a Zarf package without first running zarf init against the cluster. This is ideal for connected environments where you want to use existing VCS and container registries."`
}

// ZarfBuildData is written during the packager.Create() operation to track details of the created package.
type ZarfBuildData struct {
	Terminal     string `json:"terminal"`
	User         string `json:"user"`
	Architecture string `json:"architecture"`
	Timestamp    string `json:"timestamp"`
	Version      string `json:"version"`
}

// ZarfPackageVariable are variables that can be used to dynamically template K8s resources.
type ZarfPackageVariable struct {
	Name        string `json:"name" jsonschema:"description=The name to be used for the variable,pattern=^[A-Z_]+$"`
	Description string `json:"description,omitempty" jsonschema:"description=A description of the variable to be used when prompting the user a value"`
	Default     string `json:"default,omitempty" jsonschema:"description=The default value to use for the variable"`
	Prompt      bool   `json:"prompt,omitempty" jsonschema:"description=Whether to prompt the user for input for this variable"`
}

// ZarfPackageConstant are constants that can be used to dynamically template K8s resources.
type ZarfPackageConstant struct {
	Name  string `json:"name" jsonschema:"description=The name to be used for the constant,pattern=^[A-Z_]+$"`
	Value string `json:"value" jsonschema:"description=The value to set for the constant during deploy"`
	// Include a description that will only be displayed during package create/deploy confirm prompts
	Description string `json:"description,omitempty" jsonschema:"description=A description of the constant to explain its purpose on package create or deploy confirmation prompts"`
}
