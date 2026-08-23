// Copyright 2020 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package testutil

import (
	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func NewFakeRESTMapper(gvks ...schema.GroupVersionKind) meta.RESTMapper {
	var groupVersions []schema.GroupVersion
	for _, gvk := range gvks {
		groupVersions = append(groupVersions, gvk.GroupVersion())
	}
	mapper := meta.NewDefaultRESTMapper(groupVersions)
	for _, gvk := range gvks {
		mapper.Add(gvk, meta.RESTScopeNamespace)
	}
	return fakeRESTMapper{
		DefaultRESTMapper:    mapper,
		defaultGroupVersions: groupVersions,
	}
}

type fakeRESTMapper struct {
	*meta.DefaultRESTMapper
	defaultGroupVersions []schema.GroupVersion
}

// Equal returns true if the defaultGroupVersions are equal.
// Implements the "(T) Equal(T) bool" interface for cmp.Equal:
// https://pkg.go.dev/github.com/google/go-cmp/cmp#Equal
func (rm fakeRESTMapper) Equal(other fakeRESTMapper) bool {
	return cmp.Equal(rm.defaultGroupVersions, other.defaultGroupVersions)
}
