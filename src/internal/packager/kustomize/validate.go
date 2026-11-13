// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package kustomize

import (
	"errors"
	"strings"

	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	"sigs.k8s.io/yaml"
)

// ValidatePatch reports whether Kustomize can load an inline patch with an optional target. It builds a
// kustomization that only holds the patch, so failures that depend on the patched resources are not reported.
func ValidatePatch(patch string, target *types.Selector) error {
	k, err := yaml.Marshal(types.Kustomization{Patches: []types.Patch{{Patch: patch, Target: target}}})
	if err != nil {
		return err
	}
	// Build in a subdirectory: on Windows the in-memory root is `\`, so it cannot be found as "/".
	fs := filesys.MakeFsInMemory()
	if err := fs.WriteFile("/validate/kustomization.yaml", k); err != nil {
		return err
	}
	_, err = RunKustomizer(fs, "/validate", krusty.MakeDefaultOptions())
	switch {
	case err == nil:
		return nil
	case target == nil && strings.Contains(err.Error(), "no resource matches strategic merge patch"):
		// An untargeted strategic merge patch must match a resource, and there are none here.
		return nil
	case strings.HasPrefix(err.Error(), "trouble configuring builtin") && errors.Unwrap(err) != nil:
		// Drop krusty's prefix, which only repeats the generated kustomization.
		return errors.Unwrap(err)
	}
	return err
}
