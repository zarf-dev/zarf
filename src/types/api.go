// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package types contains all the types used by Zarf.
package types

// RestAPI is the struct that is used to marshal/unmarshal the top-level API objects.
type RestAPI struct {
	ZarfPackage          ZarfPackage          `json:"zarfPackage"`
	ZarfState            ZarfState            `json:"zarfState"`
	ZarfCommonOptions    ZarfCommonOptions    `json:"zarfCommonOptions"`
	ZarfCreateOptions    ZarfCreateOptions    `json:"zarfCreateOptions"`
	ZarfDeployOptions    ZarfDeployOptions    `json:"zarfDeployOptions"`
	ZarfInitOptions      ZarfInitOptions      `json:"zarfInitOptions"`
	ConnectStrings       ConnectStrings       `json:"connectStrings"`
	ClusterSummary       ClusterSummary       `json:"clusterSummary"`
	DeployedPackage      DeployedPackage      `json:"deployedPackage"`
	APIZarfPackage       APIZarfPackage       `json:"apiZarfPackage"`
	APIZarfDeployPayload APIZarfDeployPayload `json:"apiZarfDeployPayload"`
}

// ClusterSummary contains the summary of a cluster for the API.
type ClusterSummary struct {
	Reachable   bool      `json:"reachable"`
	HasZarf     bool      `json:"hasZarf"`
	Distro      string    `json:"distro"`
	ZarfState   ZarfState `json:"zarfState"`
	Host        string    `json:"host"`
	K8sRevision string    `json:"k8sRevision"`
}

// APIZarfPackage represents a ZarfPackage and its path for the API.
type APIZarfPackage struct {
	Path        string      `json:"path"`
	ZarfPackage ZarfPackage `json:"zarfPackage"`
}

// APIZarfDeployPayload represents the needed data to deploy a ZarfPackage/ZarfInit
type APIZarfDeployPayload struct {
	DeployOpts ZarfDeployOptions `json:"deployOpts"`
	InitOpts   *ZarfInitOptions  `json:"initOpts,omitempty"`
}
