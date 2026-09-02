// Copyright 2021 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0
//

package dependson

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/cli-utils/pkg/object"
)

const (
	// Number of fields for a cluster-scoped depends-on object value. Example:
	//   rbac.authorization.k8s.io/ClusterRole/my-cluster-role-name
	numFieldsClusterScoped = 3
	// Number of fields for a namespace-scoped depends-on object value. Example:
	//   apps/namespaces/my-namespace/Deployment/my-deployment-name
	numFieldsNamespacedScoped = 5
	// Used to separate multiple depends-on objects.
	annotationSeparator = ","
	// Used to separate the fields for a depends-on object value.
	fieldSeparator  = "/"
	namespacesField = "namespaces"
)

// FormatDependencySet formats the passed dependency set as a string.
//
// Object references are separated by ','.
//
// Returns the formatted DependencySet or an error if unable to format.
func FormatDependencySet(depSet DependencySet) (string, error) {
	var dependsOnStr string
	for i, depObj := range depSet {
		if i > 0 {
			dependsOnStr += annotationSeparator
		}
		objStr, err := FormatObjMetadata(depObj)
		if err != nil {
			return "", fmt.Errorf("failed to format object metadata (index: %d): %w", i, err)
		}
		dependsOnStr += objStr
	}
	return dependsOnStr, nil
}

// ParseDependencySet parses the passed string as a set of object
// references.
//
// Object references are separated by ','.
//
// Returns the parsed DependencySet or an error if unable to parse.
func ParseDependencySet(depsStr string) (DependencySet, error) {
	objs := DependencySet{}
	for i, objStr := range strings.Split(depsStr, annotationSeparator) {
		obj, err := ParseObjMetadata(objStr)
		if err != nil {
			return objs, fmt.Errorf("failed to parse object reference (index: %d): %w", i, err)
		}
		objs = append(objs, obj)
	}
	return objs, nil
}

// FormatObjMetadata formats the passed object metadata as a string.
//
// Object references can have either three fields (cluster-scoped object) or
// five fields (namespace-scoped object).
//
// Fields are separated by '/'.
//
// Examples:
//
//	Cluster-Scoped: <group>/<kind>/<name> (3 fields)
//	Namespaced: <group>/namespaces/<namespace>/<kind>/<name> (5 fields)
//
// Group and namespace may be empty, but name and kind may not.
//
// Returns the formatted ObjMetadata string or an error if unable to format.
func FormatObjMetadata(obj object.ObjMetadata) (string, error) {
	gk := obj.GroupKind
	// group and namespace are allowed to be empty, but name and kind are not
	if gk.Kind == "" {
		return "", fmt.Errorf("invalid object metadata: kind is empty")
	}
	if obj.Name == "" {
		return "", fmt.Errorf("invalid object metadata: name is empty")
	}
	if obj.Namespace != "" {
		return fmt.Sprintf("%s/namespaces/%s/%s/%s", gk.Group, obj.Namespace, gk.Kind, obj.Name), nil
	}
	return fmt.Sprintf("%s/%s/%s", gk.Group, gk.Kind, obj.Name), nil
}

// ParseObjMetadata parses the passed string as a object metadata.
//
// Object references can have either three fields (cluster-scoped object) or
// five fields (namespace-scoped object).
//
// Fields are separated by '/'.
//
// Examples:
//
//	Cluster-Scoped: <group>/<kind>/<name> (3 fields)
//	Namespaced: <group>/namespaces/<namespace>/<kind>/<name> (5 fields)
//
// Group and namespace may be empty, but name and kind may not.
//
// Returns the parsed ObjMetadata or an error if unable to parse.
func ParseObjMetadata(objStr string) (object.ObjMetadata, error) {
	var obj object.ObjMetadata
	var group, kind, namespace, name string
	objStr = strings.TrimSpace(objStr)
	fields := strings.Split(objStr, fieldSeparator)

	if len(fields) != numFieldsClusterScoped && len(fields) != numFieldsNamespacedScoped {
		return obj, fmt.Errorf("expected %d or %d fields, found %d: %q",
			numFieldsClusterScoped, numFieldsNamespacedScoped, len(fields), objStr)
	}

	group = fields[0]
	if len(fields) == 3 {
		kind = fields[1]
		name = fields[2]
	} else {
		if fields[1] != namespacesField {
			return obj, fmt.Errorf("missing %q field: %q", namespacesField, objStr)
		}
		namespace = fields[2]
		kind = fields[3]
		name = fields[4]
	}

	id := object.ObjMetadata{
		Namespace: namespace,
		Name:      name,
		GroupKind: schema.GroupKind{
			Group: group,
			Kind:  kind,
		},
	}
	return id, nil
}
