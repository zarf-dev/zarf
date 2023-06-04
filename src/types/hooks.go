// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package types

const (
	HookSecretPrefix = "zarf-hook-"
	ECRHook          = "ecr-config"
)

var AllHookNames = []string{ECRHook}

// HookConfig contains information about a hook that should be run during the deployment of a Zarf package.
// NOTE: Hooks are in a pre-alpha status. Use at your own risk!
type HookConfig struct {
	HookName string                 `json:"hookName" jsonschema:"description=Name of the hook"`
	HookData map[string]interface{} `json:"hookData" jsonschema:"description=Generic data map used for the hook. The data is obtained from a secret in the Zarf namespace"`
	// OCIReference string                 `json:"ociReference" jsonschema:"description=Optional OCI reference to the hook image to run"`
	// TODO: @JPERRY Figure out a good way to determine lifecycle running of hooks
}

// TODO: @JPERRY refresh knowledge about RPCs.....
func (h HookConfig) Run() error {

	// ociReference, ok := h.HookData["ociReference"].(string)
	// Download the OCIReference

	// or

	// DO internal things..

	return nil
}
