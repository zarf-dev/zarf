// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"testing"
)

func TestValues(t *testing.T) {
	t.Log("E2E: Package values")
	// TODO(mkcp): e2e fill in the rest of this
	myPkg := "todo-arm64"
	myValues := "myValues.yaml"
	args := []string{"package", "create", myPkg, "-f", myValues}
	_, _, _ = e2e.Zarf(t, args...)
}
