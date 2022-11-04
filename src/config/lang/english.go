//go:build !alt_language

// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package lang contains the language strings for english used by Zarf
// Alternative languages can be created by duplicating this file and changing the build tag to "//go:build alt_language && <language>"
package lang

// All language strings should be in the form of a constant
// The constants should be grouped by the top level package they are used in (or common)
// The format should be <PachaName><Err/Debug/Info><ShortDescription>
// Include sprintf formatting directives in the string if needed
const (
	ErrUnmarshal = "failed to unmarshal file: %w"
)

// Zarf Agent messages
// These are only seen in the Kubernetes logs
const (
	AgentErrStart    = "Failed to start the web server"
	AgentErrShutdown = "unable to properly shutdown the web server"

	AgentInfoPort     = "Server running in port: %s"
	AgentInfoShutdown = "Shutdown gracefully..."

	AgentHooksErrGetState      = "failed to load zarf state from file: %w"
	AgentHooksErrHostnameMatch = "failed to complete hostname matching: %w"

	AgentHooksDebugGitURL    = "Using the url of (%s) to mutate the flux repository"
	AgentHooksDebugGitMutate = "original git URL of (%s) got mutated to (%s)"

	AgentHooksErrImageSwap = "Unable to swap the host for (%s)"

	
)
