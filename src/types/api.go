// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package types contains all the types used by Zarf.
package types

import (
	"k8s.io/client-go/tools/clientcmd/api"
)

// RestAPI is the struct that is used to marshal/unmarshal the top-level API objects.
type RestAPI struct {
	ZarfPackage                   ZarfPackage                   `json:"zarfPackage"`
	ZarfState                     ZarfState                     `json:"zarfState"`
	ZarfCommonOptions             ZarfCommonOptions             `json:"zarfCommonOptions"`
	ZarfCreateOptions             ZarfCreateOptions             `json:"zarfCreateOptions"`
	ZarfDeployOptions             ZarfDeployOptions             `json:"zarfDeployOptions"`
	ZarfInitOptions               ZarfInitOptions               `json:"zarfInitOptions"`
	ConnectStrings                ConnectStrings                `json:"connectStrings"`
	ClusterSummary                ClusterSummary                `json:"clusterSummary"`
	DeployedPackage               DeployedPackage               `json:"deployedPackage"`
	APIZarfPackage                APIZarfPackage                `json:"apiZarfPackage"`
	APIZarfDeployPayload          APIZarfDeployPayload          `json:"apiZarfDeployPayload"`
	APIZarfPackageConnection      APIDeployedPackageConnection  `json:"apiZarfPackageConnection"`
	APIDeployedPackageConnections APIDeployedPackageConnections `json:"apiZarfPackageConnections"`
	APIConnections                APIConnections                `json:"apiConnections"`
	APIExplorer                   APIExplorer                   `json:"apiExplorer"`
}

// ClusterSummary contains the summary of a cluster for the API.
type ClusterSummary struct {
	Reachable   bool        `json:"reachable"`
	HasZarf     bool        `json:"hasZarf"`
	Distro      string      `json:"distro"`
	ZarfState   ZarfState   `json:"zarfState"`
	K8sRevision string      `json:"k8sRevision"`
	RawConfig   *api.Config `json:"rawConfig"`
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

// APIConnections represents all of the existing connections
type APIConnections map[string]APIDeployedPackageConnections

// APIDeployedPackageConnections represents all of the connections for a deployed package
type APIDeployedPackageConnections []APIDeployedPackageConnection

// APIDeployedPackageConnection represents a single connection from a deployed package
type APIDeployedPackageConnection struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

type APIExplorer struct {
	Dir   string            `json:"dir"`
	Files []APIExplorerFile `json:"files"`
}

type APIExplorerFile struct {
	IsDir bool   `json:"isDir"`
	Path  string `json:"path"`
}
