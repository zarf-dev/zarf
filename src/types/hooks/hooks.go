// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package hooks

import "fmt"

// List out all known hooks (these are internal hooks)
const (
	HookSecretPrefix   = "zarf-hook"
	ECRRepositoryHook  = "zarf-hook-ecr-repository"
	ECRCredentialsHook = "zarf-hook-ecr-credentials"
)

// HookLifecycle defines when a hook should be run. The executing order of hooks are not guaranteed.
// NOTE: BeforeComponent hooks will always run before any 'beforeAction' component hooks.
// NOTE: AfterComponent hooks will always run after all 'afterAction' component hooks.
type HookLifecycle string

// Constants for hook lifecycle management
const (
	BeforePackage HookLifecycle = "before-package"
	AfterPackage  HookLifecycle = "after-package"

	BeforeComponent HookLifecycle = "before-component"
	AfterComponent  HookLifecycle = "after-component"
)

// HookConfig contains information about a hook that should be run during the deployment of a Zarf package.
// NOTE: Hooks are in a pre-alpha status. Use at your own risk!
type HookConfig struct {
	HookName     string                 `json:"hookName" jsonschema:"description=Name of the hook"`
	Internal     bool                   `json:"internal" jsonschema:"description=Internal hooks are run by Zarf itself, not by a plugin"`
	Lifecycle    HookLifecycle          `json:"lifecycle" jsonschema:"description=Lifecycle of the hook"`
	HookData     map[string]interface{} `json:"hookData" jsonschema:"description=Generic data map used for the hook. The data is obtained from a secret in the Zarf namespace"`
	OCIReference string                 `json:"ociReference" jsonschema:"description=Optional OCI reference to the hook image to run"`
}

// Run will execute the hook via downloading the OCI image and running it. Data passed into the hook will be provided the image via gRPC.
func (h HookConfig) Run() error {

	return fmt.Errorf("not implemented")
}
