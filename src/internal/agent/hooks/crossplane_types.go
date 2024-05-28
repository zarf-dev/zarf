// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package hooks

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Configration, Functions and Providers are definitions of Crossplane resources.
// The Crossplane structs in this file have been partially copied from upstream.
//
// https://github.com/crossplane/crossplane/blob/v1.16.0/apis/pkg/v1/configuration_types.go
// https://github.com/crossplane/crossplane/blob/v1.16.0/apis/pkg/v1beta1/function_types.go
// https://github.com/crossplane/crossplane/blob/v1.16.0/apis/pkg/v1/provider_types.go
//
// There were errors encountered when trying to import crossplane as a Go package.

// A Configuration installs an OCI compatible Crossplane package, extending
// Crossplane with support for new kinds of CompositeResourceDefinitions and
// Compositions.
//
// Read the Crossplane documentation for
// [more information about Configuration packages](https://docs.crossplane.io/latest/concepts/packages).
type Configuration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ConfigurationSpec `json:"spec"`
}

// ConfigurationSpec specifies details about a request to install a
// configuration to Crossplane.
type ConfigurationSpec struct {
	PackageSpec `json:",inline"`
}

// A Provider installs an OCI compatible Crossplane package, extending
// Crossplane with support for new kinds of managed resources.
//
// Read the Crossplane documentation for
// [more information about Providers](https://docs.crossplane.io/latest/concepts/providers).
type Provider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ProviderSpec `json:"spec,omitempty"`
}

// ProviderSpec specifies details about a request to install a provider to
// Crossplane.
type ProviderSpec struct {
	PackageSpec `json:",inline"`
}

// A Function installs an OCI compatible Crossplane package, extending
// Crossplane with support for a new kind of composition function.
//
// Read the Crossplane documentation for
// [more information about Functions](https://docs.crossplane.io/latest/concepts/composition-functions).
type Function struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec FunctionSpec `json:"spec,omitempty"`
}

// FunctionSpec specifies the configuration of a Function.
type FunctionSpec struct {
	PackageSpec `json:",inline"`
}

// PackageSpec specifies the desired state of a Package.
type PackageSpec struct {
	// Package is the name of the package that is being requested.
	Package string `json:"package"`

	// PackagePullSecrets are named secrets in the same namespace that can be used
	// to fetch packages from private registries.
	// +optional
	PackagePullSecrets []corev1.LocalObjectReference `json:"packagePullSecrets,omitempty"`
}
