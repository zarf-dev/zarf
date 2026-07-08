// Copyright 2020 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0
//
// The testutil package houses utility function for testing.

package testutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/cli-utils/pkg/object"
	"sigs.k8s.io/cli-utils/pkg/object/dependson"
)

// OwningInventoryKey is the annotation key indicating the inventory owning an object.
// This is a copy of inventory.OwningInventoryKey to avoid dependency cycle.
const OwningInventoryKey = "config.k8s.io/owning-inventory"

var codec = scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)

// Unstructured translates the passed object config string into an
// object in Unstructured format. The mutators modify the config
// yaml before returning the object.
func Unstructured(t *testing.T, manifest string, mutators ...Mutator) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	err := runtime.DecodeInto(codec, []byte(manifest), u)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	return Mutate(u, mutators...)
}

// Mutate executes the specified Mutators on the specified object.
func Mutate(u *unstructured.Unstructured, mutators ...Mutator) *unstructured.Unstructured {
	for _, m := range mutators {
		m.Mutate(u)
	}
	return u
}

// Mutator inteface defines a function to update an object
// while translating it unto Unstructured format from yaml config.
type Mutator interface {
	Mutate(u *unstructured.Unstructured)
}

// ToIdentifier translates object yaml config into ObjMetadata.
func ToIdentifier(t *testing.T, manifest string) object.ObjMetadata {
	obj := Unstructured(t, manifest)
	return object.ObjMetadata{
		GroupKind: obj.GetObjectKind().GroupVersionKind().GroupKind(),
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(), // If cluster-scoped, empty namespace string
	}
}

// AddOwningInv returns a Mutator which adds the passed inv string
// as the owning inventory annotation.
func AddOwningInv(t *testing.T, inv string) Mutator {
	return addOwningInvMutator{
		t:   t,
		inv: inv,
	}
}

// owningInvMutator encapsulates the fields necessary to modify
// an object by adding the owning inventory annotation. This
// structure implements the Mutator interface.
type addOwningInvMutator struct {
	t   *testing.T
	inv string
}

// Mutate updates the passed object by adding the owning
// inventory annotation. Needed to implement the Mutator interface.
func (a addOwningInvMutator) Mutate(u *unstructured.Unstructured) {
	annos, found, err := unstructured.NestedStringMap(u.Object, "metadata", "annotations")
	if !assert.NoError(a.t, err) {
		a.t.FailNow()
	}
	if !found {
		annos = make(map[string]string)
	}
	annos[OwningInventoryKey] = a.inv
	err = unstructured.SetNestedStringMap(u.Object, annos, "metadata", "annotations")
	if !assert.NoError(a.t, err) {
		a.t.FailNow()
	}
}

// DeleteOwningInv returns a Mutator which deletes the passed inv string
// from the owning inventory annotation.
func DeleteOwningInv(t *testing.T, inv string) Mutator {
	return deleteOwningInvMutator{
		t:   t,
		inv: inv,
	}
}

// deleteOwningInvMutator encapsulates the fields necessary to modify
// an object by deleting the owning inventory annotation. This
// structure implements the Mutator interface.
type deleteOwningInvMutator struct {
	t   *testing.T
	inv string
}

// Mutate updates the passed object by deleting the owning
// inventory annotation. Needed to implement the Mutator interface.
func (a deleteOwningInvMutator) Mutate(u *unstructured.Unstructured) {
	annos, found, err := unstructured.NestedStringMap(u.Object, "metadata", "annotations")
	if !assert.NoError(a.t, err) {
		a.t.FailNow()
	}
	if !found {
		annos = make(map[string]string)
	}
	if !assert.Equal(a.t, a.inv, annos[OwningInventoryKey]) {
		a.t.FailNow()
	}
	delete(annos, OwningInventoryKey)
	if len(annos) > 0 {
		err = unstructured.SetNestedStringMap(u.Object, annos, "metadata", "annotations")
		if !assert.NoError(a.t, err) {
			a.t.FailNow()
		}
	} else {
		unstructured.RemoveNestedField(u.Object, "metadata", "annotations")
	}
}

// AddDependsOn returns a testutil.Mutator which adds the passed objects as a
// depends-on annotation to the object which is mutated. Multiple objects
// passed in means multiple depends on objects in the annotation separated
// by a comma.
func AddDependsOn(t *testing.T, deps ...object.ObjMetadata) Mutator {
	return dependsOnMutator{
		t:    t,
		deps: dependson.DependencySet(deps),
	}
}

// dependsOnMutator encapsulates fields for adding depends-on annotation
// to a test object. Implements the Mutator interface.
type dependsOnMutator struct {
	t    *testing.T
	deps dependson.DependencySet
}

// Mutate writes a depends-on annotation on the supplied object. The value of
// the annotation is a set of dependencies referencing the dependsOnMutator's
// depObjs.
func (d dependsOnMutator) Mutate(u *unstructured.Unstructured) {
	err := dependson.WriteAnnotation(u, d.deps)
	if !assert.NoError(d.t, err) {
		d.t.FailNow()
	}
}
