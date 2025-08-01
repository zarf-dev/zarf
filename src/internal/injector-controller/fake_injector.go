// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package injectorcontroller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
)

// FakeInjectionExecutor is a fake implementation of InjectionExecutor for testing
type FakeInjectionExecutor struct {
	RunInjectionCalled bool
	RunWithOwnerCalled bool
	LastOwnerPod       *corev1.Pod
}

// NewFakeInjectionExecutor creates a new fake implementation
func NewFakeInjectionExecutor() *FakeInjectionExecutor {
	return &FakeInjectionExecutor{}
}

// Run records that it was called
func (f *FakeInjectionExecutor) Run(_ context.Context, _ *corev1.Pod) error {
	f.RunInjectionCalled = true
	return nil
}

// RunWithOwner records that it was called with an owner
func (f *FakeInjectionExecutor) RunWithOwner(_ context.Context, _ *corev1.Pod, owner *corev1.Pod) error {
	f.RunWithOwnerCalled = true
	f.LastOwnerPod = owner
	return nil
}
