// Copyright 2021 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0
//

package dependson

import (
	"sigs.k8s.io/cli-utils/pkg/object"
)

// DependencySet is a set of object references.
// When testing equality, order is not importent.
type DependencySet object.ObjMetadataSet

// Equal returns true if the ObjMetadata sets are equivalent, ignoring order.
// Fulfills Equal interface from github.com/google/go-cmp
func (a DependencySet) Equal(b DependencySet) bool {
	return object.ObjMetadataSet(a).Equal(object.ObjMetadataSet(b))
}
