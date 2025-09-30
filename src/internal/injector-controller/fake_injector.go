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
}

// NewFakeInjectionExecutor creates a new fake implementation
func NewFakeInjectionExecutor() *FakeInjectionExecutor {
	return &FakeInjectionExecutor{}
}

// Run records that it was called
func (f *FakeInjectionExecutor) Run(_ context.Context, _ *corev1.Pod, _ *corev1.Pod) error {
	f.RunInjectionCalled = true
	return nil
}
