// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package k8s provides a client for interacting with a Kubernetes cluster.
package k8s

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Log is a function that logs a message for the application.
type Log func(string, ...any)

// Labels maps a label name to a value.
type Labels map[string]string

// K8s represents a Kubernetes client, with all the necessary information to connect to a cluster.
type K8s struct {
	Clientset  *kubernetes.Clientset
	RestConfig *rest.Config
	Log        Log
	Labels     Labels
}

// PodLookup represents the information needed to lookup a pod (or a container running in the defined pod).
type PodLookup struct {
	Namespace string `json:"namespace" jsonschema:"description=The namespace to target for data injection"`
	Selector  string `json:"selector" jsonschema:"description=The K8s selector to target for data injection"`
	Container string `json:"container" jsonschema:"description=The container to target for data injection"`
}

// GeneratedPKI represents a public key certificate.
type GeneratedPKI struct {
	CA   []byte `json:"ca"`
	Cert []byte `json:"cert"`
	Key  []byte `json:"key"`
}
