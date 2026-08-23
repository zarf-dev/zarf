/*
Copyright 2021 The Flux authors

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

package meta

import "strings"

// ObjectWithDependencies describes a Kubernetes resource object with dependencies.
// +k8s:deepcopy-gen=false
type ObjectWithDependencies interface {
	// GetDependsOn returns a DependencyReference list the object depends on.
	GetDependsOn() []DependencyReference
}

// MakeDependsOn parses a list of dependency strings into DependencyReference
// objects. Each dependency string can be in one of the following formats:
//   - "name" - a dependency in the same namespace with no CEL expression
//   - "namespace/name" - a dependency in a specific namespace
//   - "name@readyExpr" - a dependency with a CEL readiness expression
//   - "namespace/name@readyExpr" - a dependency in a specific namespace with a CEL expression
//
// The @ symbol is used to separate the resource reference from the CEL expression.
// Note that @ cannot be part of resource names or namespaces per Kubernetes naming conventions:
// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/
// For CEL expression syntax, see:
// https://github.com/google/cel-spec/blob/master/doc/langdef.md
func MakeDependsOn(deps []string) []DependencyReference {
	refs := make([]DependencyReference, 0, len(deps))
	for _, dep := range deps {
		ref := DependencyReference{}

		// Split off the CEL ready expression if present.
		if idx := strings.Index(dep, "@"); idx != -1 {
			ref.ReadyExpr = dep[idx+1:]
			dep = dep[:idx]
		}

		// Split the namespace/name.
		if parts := strings.SplitN(dep, "/", 2); len(parts) == 2 {
			ref.Namespace = parts[0]
			ref.Name = parts[1]
		} else {
			ref.Name = dep
		}

		refs = append(refs, ref)
	}
	return refs
}
