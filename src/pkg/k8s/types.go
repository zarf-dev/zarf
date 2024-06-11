// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package k8s provides a client for interacting with a Kubernetes cluster.
package k8s

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/cli-utils/pkg/kstatus/watcher"
)

// Log is a function that logs a message.
type Log func(string, ...any)

// K8s is a client for interacting with a Kubernetes cluster.
type K8s struct {
	Clientset  kubernetes.Interface
	RestConfig *rest.Config
	Watcher    watcher.StatusWatcher
	Log        Log
}

// GeneratedPKI is a struct for storing generated PKI data.
type GeneratedPKI struct {
	CA   []byte `json:"ca"`
	Cert []byte `json:"cert"`
	Key  []byte `json:"key"`
}
