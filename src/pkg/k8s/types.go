// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package k8s provides a client for interacting with a Kubernetes cluster.
package k8s

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Log is a ... @JPERRY
type Log func(string, ...any)

// Labels is a ... @JPERRY
type Labels map[string]string

// K8s is a ... @JPERRY
type K8s struct {
	Clientset  *kubernetes.Clientset
	RestConfig *rest.Config
	Log        Log
	Labels     Labels
}

// PodLookup ... @JPERRY
type PodLookup struct {
	Namespace string `json:"namespace" jsonschema:"description=The namespace to target for data injection"`
	Selector  string `json:"selector" jsonschema:"description=The K8s selector to target for data injection"`
	Container string `json:"container" jsonschema:"description=The container to target for data injection"`
}

// GeneratedPKI ... @JPERRY
type GeneratedPKI struct {
	CA   []byte `json:"ca"`
	Cert []byte `json:"cert"`
	Key  []byte `json:"key"`
}
