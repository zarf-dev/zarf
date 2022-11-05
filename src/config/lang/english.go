//go:build !alt_language

// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package lang contains the language strings for english used by Zarf
// Alternative languages can be created by duplicating this file and changing the build tag to "//go:build alt_language && <language>"
package lang

// All language strings should be in the form of a constant
// The constants should be grouped by the top level package they are used in (or common)
// The format should be <PathName><Err/Info><ShortDescription>
// Debug messages will not be a part of the language strings since they are not intended to be user facing
// Include sprintf formatting directives in the string if needed
const (
	ErrUnmarshal = "failed to unmarshal file: %w"
)

// Zarf Agent messages
// These are only seen in the Kubernetes logs
const (
	AgentInfoWebhookAllowed = "Webhook [%s - %s] - Allowed: %t"
	AgentInfoShutdown       = "Shutdown gracefully..."
	AgentInfoPort           = "Server running in port: %s"

	AgentErrStart                  = "Failed to start the web server"
	AgentErrShutdown               = "unable to properly shutdown the web server"
	AgentErrNilReq                 = "malformed admission review: request is nil"
	AgentErrMarshalResponse        = "unable to marshal the response"
	AgentErrMarshallJSONPatch      = "unable to marshall the json patch"
	AgentErrInvalidType            = "only content type 'application/json' is supported"
	AgentErrInvalidOp              = "invalid operation: %s"
	AgentErrInvalidMethod          = "invalid method only POST requests are allowed"
	AgentErrImageSwap              = "Unable to swap the host for (%s)"
	AgentErrHostnameMatch          = "failed to complete hostname matching: %w"
	AgentErrGetState               = "failed to load zarf state from file: %w"
	AgentErrCouldNotDeserializeReq = "could not deserialize request: %s"
	AgentErrBindHandler            = "Unable to bind the webhook handler"
	AgentErrBadRequest             = "could not read request body: %s"
)
