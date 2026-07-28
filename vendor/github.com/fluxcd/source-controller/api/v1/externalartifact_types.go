/*
Copyright 2025 The Flux authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fluxcd/pkg/apis/meta"
)

// ExternalArtifactKind is the string representation of the ExternalArtifact.
const ExternalArtifactKind = "ExternalArtifact"

// ExternalArtifactSpec defines the desired state of ExternalArtifact
type ExternalArtifactSpec struct {
	// SourceRef points to the Kubernetes custom resource for
	// which the artifact is generated.
	// +optional
	SourceRef *meta.NamespacedObjectKindReference `json:"sourceRef,omitempty"`
}

// ExternalArtifactStatus defines the observed state of ExternalArtifact
type ExternalArtifactStatus struct {
	// Artifact represents the output of an ExternalArtifact reconciliation.
	// +optional
	Artifact *meta.Artifact `json:"artifact,omitempty"`

	// Conditions holds the conditions for the ExternalArtifact.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// GetConditions returns the status conditions of the object.
func (in *ExternalArtifact) GetConditions() []metav1.Condition {
	return in.Status.Conditions
}

// SetConditions sets the status conditions on the object.
func (in *ExternalArtifact) SetConditions(conditions []metav1.Condition) {
	in.Status.Conditions = conditions
}

// GetArtifact returns the latest Artifact from the ExternalArtifact if
// present in the status sub-resource.
func (in *ExternalArtifact) GetArtifact() *meta.Artifact {
	return in.Status.Artifact
}

// GetRequeueAfter returns the duration after which the ExternalArtifact
// must be reconciled again.
func (in *ExternalArtifact) GetRequeueAfter() time.Duration {
	return time.Minute
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description=""
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description=""
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message",description=""
// +kubebuilder:printcolumn:name="Source",type="string",JSONPath=".spec.sourceRef.name",description=""

// ExternalArtifact is the Schema for the external artifacts API
type ExternalArtifact struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ExternalArtifactSpec   `json:"spec,omitempty"`
	Status ExternalArtifactStatus `json:"status,omitempty"`
}

// ExternalArtifactList contains a list of ExternalArtifact
// +kubebuilder:object:root=true
type ExternalArtifactList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ExternalArtifact `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ExternalArtifact{}, &ExternalArtifactList{})
}
