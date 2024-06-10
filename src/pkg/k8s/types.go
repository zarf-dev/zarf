// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package k8s provides a client for interacting with a Kubernetes cluster.
package k8s

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/cli-utils/pkg/kstatus/watcher"
)

// Log is a function that logs a message.
type Log func(string, ...any)

// Labels is a map of K8s labels.
type Labels map[string]string

// K8s is a client for interacting with a Kubernetes cluster.
type K8s struct {
	Clientset  kubernetes.Interface
	RestConfig *rest.Config
	Watcher    watcher.StatusWatcher
	Log        Log
}

// PodLookup is a struct for specifying a pod to target for data injection or lookups.
type PodLookup struct {
	Namespace string `json:"namespace" jsonschema:"description=The namespace to target for data injection"`
	Selector  string `json:"selector" jsonschema:"description=The K8s selector to target for data injection"`
	Container string `json:"container" jsonschema:"description=The container to target for data injection"`
}

// PodFilter is a function that returns true if the pod should be targeted for data injection or lookups.
type PodFilter func(pod corev1.Pod) bool

// GeneratedPKI is a struct for storing generated PKI data.
type GeneratedPKI struct {
	CA   []byte `json:"ca"`
	Cert []byte `json:"cert"`
	Key  []byte `json:"key"`
}
