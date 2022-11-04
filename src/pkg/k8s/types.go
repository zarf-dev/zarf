// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package k8s provides a client for interacting with a Kubernetes cluster.
package k8s

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Log func(string, ...any)

type Labels map[string]string

type K8s struct {
	Clientset  *kubernetes.Clientset
	RestConfig *rest.Config
	Log        Log
	Labels     Labels
}

type PodLookup struct {
	Namespace string `json:"namespace" jsonschema:"description=The namespace to target for data injection"`
	Selector  string `json:"selector" jsonschema:"description=The K8s selector to target for data injection"`
	Container string `json:"container" jsonschema:"description=The container to target for data injection"`
}

type GeneratedPKI struct {
	CA   []byte `json:"ca"`
	Cert []byte `json:"cert"`
	Key  []byte `json:"key"`
}
