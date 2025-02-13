// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package message provides a rich set of functions for displaying messages to the user.
package message

import (
	"fmt"
	"strings"

	"github.com/pterm/pterm"
	"github.com/zarf-dev/zarf/src/types"
)

// Common constants for printing credentials
const (
	RegistryKey     = "registry"
	RegistryReadKey = "registry-readonly"
	GitKey          = "git"
	GitReadKey      = "git-readonly"
	ArtifactKey     = "artifact"
	AgentKey        = "agent"
)

// PrintComponentCredential displays credentials for a single component
func PrintComponentCredential(state *types.ZarfState, componentName string) {
	switch strings.ToLower(componentName) {
	case GitKey:
		Notef("Git Server push password (username: %s):", state.GitServer.PushUsername)
	case GitReadKey:
		Notef("Git Server (read-only) password (username: %s):", state.GitServer.PullUsername)
	case ArtifactKey:
		Notef("Artifact Server token (username: %s):", state.ArtifactServer.PushUsername)
	case RegistryKey:
		Notef("Image Registry password (username: %s):", state.RegistryInfo.PushUsername)
	case RegistryReadKey:
		Notef("Image Registry (read-only) password (username: %s):", state.RegistryInfo.PullUsername)
	default:
		Warn("Unknown component: " + componentName)
	}
}

// PrintCredentialUpdates displays credentials that will be updated
func PrintCredentialUpdates(oldState *types.ZarfState, newState *types.ZarfState, services []string) {
	// Pause the logfile's output to avoid credentials being printed to the log file
	if logFile != nil {
		logFile.Pause()
		defer logFile.Resume()
	}

	for _, service := range services {
		HorizontalRule()

		switch service {
		case RegistryKey:
			oR := oldState.RegistryInfo
			nR := newState.RegistryInfo
			Title("Registry", "the information used to interact with Zarf's container image registry")
			pterm.Println()
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("URL Address"), compareStrings(oR.Address, nR.Address, false))
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Push Username"), compareStrings(oR.PushUsername, nR.PushUsername, false))
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Push Password"), compareStrings(oR.PushPassword, nR.PushPassword, true))
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Pull Username"), compareStrings(oR.PullUsername, nR.PullUsername, false))
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Pull Password"), compareStrings(oR.PullPassword, nR.PullPassword, true))
		case GitKey:
			oG := oldState.GitServer
			nG := newState.GitServer
			Title("Git Server", "the information used to interact with Zarf's GitOps Git Server")
			pterm.Println()
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("URL Address"), compareStrings(oG.Address, nG.Address, false))
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Push Username"), compareStrings(oG.PushUsername, nG.PushUsername, false))
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Push Password"), compareStrings(oG.PushPassword, nG.PushPassword, true))
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Pull Username"), compareStrings(oG.PullUsername, nG.PullUsername, false))
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Pull Password"), compareStrings(oG.PullPassword, nG.PullPassword, true))
		case ArtifactKey:
			oA := oldState.ArtifactServer
			nA := newState.ArtifactServer
			Title("Artifact Server", "the information used to interact with Zarf's Artifact Server")
			pterm.Println()
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("URL Address"), compareStrings(oA.Address, nA.Address, false))
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Push Username"), compareStrings(oA.PushUsername, nA.PushUsername, false))
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Push Token"), compareStrings(oA.PushToken, nA.PushToken, true))
		case AgentKey:
			oT := oldState.AgentTLS
			nT := newState.AgentTLS
			Title("Agent TLS", "the certificates used to connect to Zarf's Agent")
			pterm.Println()
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Certificate Authority"), compareStrings(string(oT.CA), string(nT.CA), true))
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Public Certificate"), compareStrings(string(oT.Cert), string(nT.Cert), true))
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Private Key"), compareStrings(string(oT.Key), string(nT.Key), true))
		}
	}

	pterm.Println()
}

func compareStrings(curent string, updated string, secret bool) string {
	if updated == curent {
		if secret {
			return "**sanitized** (unchanged)"
		}
		return fmt.Sprintf("%s (unchanged)", curent)
	}
	if secret {
		return fmt.Sprintf("%s -> %s", pterm.FgRed.Sprint("**existing (sanitized)**"), pterm.FgGreen.Sprint("**replacement (sanitized)**"))
	}
	return fmt.Sprintf("%s -> %s", pterm.FgRed.Sprint(curent), pterm.FgGreen.Sprint(updated))
}
