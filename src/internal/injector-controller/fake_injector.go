// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package injectorcontroller

import (
	"context"

	"github.com/zarf-dev/zarf/src/pkg/state"
	"sigs.k8s.io/cli-utils/pkg/object"
)

// FakeInjectionExecutor is a fake implementation of InjectionExecutor for testing
type FakeInjectionExecutor struct {
	RunInjectionCalled  bool
	StopInjectionCalled bool
}

// NewFakeInjectionExecutor creates a new fake implementation
func NewFakeInjectionExecutor() *FakeInjectionExecutor {
	return &FakeInjectionExecutor{}
}

// RunInjection records that it was called
func (f *FakeInjectionExecutor) RunInjection(ctx context.Context, useRegistryProxy bool, payloadCMNames []string, shasum string, ipFamily state.IPFamily) error {
	f.RunInjectionCalled = true
	return nil
}

// WaitForReady does nothing in the fake
func (f *FakeInjectionExecutor) WaitForReady(ctx context.Context, objs []object.ObjMetadata) error {
	return nil
}

// StopInjection records that it was called
func (f *FakeInjectionExecutor) StopInjection(ctx context.Context, useRegistryProxy bool) error {
	f.StopInjectionCalled = true
	return nil
}
