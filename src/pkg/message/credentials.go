// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package message provides a rich set of functions for displaying messages to the user.
package message

import (
	"fmt"
	"log/slog"
	"os"
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

// PrintCredentialTable displays credentials in a table
func PrintCredentialTable(state *types.ZarfState, componentsToDeploy []types.DeployedComponent) {
	if len(componentsToDeploy) == 0 {
		componentsToDeploy = []types.DeployedComponent{{Name: "git-server"}}
	}

	// Pause the logfile's output to avoid credentials being printed to the log file
	if logFile != nil {
		logFile.Pause()
		defer logFile.Resume()
	}

	loginData := [][]string{}
	if state.RegistryInfo.IsInternal() {
		loginData = append(loginData,
			[]string{"Registry", state.RegistryInfo.PushUsername, string(state.RegistryInfo.PushPassword), "zarf connect registry", RegistryKey},
			[]string{"Registry (read-only)", state.RegistryInfo.PullUsername, string(state.RegistryInfo.PullPassword), "zarf connect registry", RegistryReadKey},
		)
	}

	for _, component := range componentsToDeploy {
		// Show message if including git-server
		if component.Name == "git-server" {
			loginData = append(loginData,
				[]string{"Git", state.GitServer.PushUsername, string(state.GitServer.PushPassword), "zarf connect git", GitKey},
				[]string{"Git (read-only)", state.GitServer.PullUsername, string(state.GitServer.PullPassword), "zarf connect git", GitReadKey},
				[]string{"Artifact Token", state.ArtifactServer.PushUsername, string(state.ArtifactServer.PushToken), "zarf connect git", ArtifactKey},
			)
		}
	}

	if len(loginData) > 0 {
		header := []string{"Application", "Username", "Password", "Connect", "Get-Creds Key"}
		Table(header, loginData)
	}
}

// PrintComponentCredential displays credentials for a single component
func PrintComponentCredential(state *types.ZarfState, componentName string) {
	switch strings.ToLower(componentName) {
	case GitKey:
		Notef("Git Server push password (username: %s):", state.GitServer.PushUsername)
		fmt.Println(state.GitServer.PushPassword)
	case GitReadKey:
		Notef("Git Server (read-only) password (username: %s):", state.GitServer.PullUsername)
		fmt.Println(state.GitServer.PullPassword)
	case ArtifactKey:
		Notef("Artifact Server token (username: %s):", state.ArtifactServer.PushUsername)
		fmt.Println(state.ArtifactServer.PushToken)
	case RegistryKey:
		Notef("Image Registry password (username: %s):", state.RegistryInfo.PushUsername)
		fmt.Println(state.RegistryInfo.PushPassword)
	case RegistryReadKey:
		Notef("Image Registry (read-only) password (username: %s):", state.RegistryInfo.PullUsername)
		fmt.Println(state.RegistryInfo.PullPassword)
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

	// IRL this would be made somewhere else
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	for _, service := range services {
		HorizontalRule()

		switch service {
		case RegistryKey:
			oR := oldState.RegistryInfo
			nR := newState.RegistryInfo
			logger.Info("Registry information", compareStrings("URL Address", oR.Address, nR.Address),
				compareStrings("Push Username", oR.PushUsername, nR.PushUsername), comparePasswords("Push Password", oR.PushPassword, nR.PushPassword),
				compareStrings("Pull Username", oR.PullUsername, nR.PullUsername), comparePasswords("Push Password", oR.PullPassword, nR.PullPassword))
		case GitKey:
			oG := oldState.GitServer
			nG := newState.GitServer
			logger.Info("Git Server info", compareStrings("URL Address", oG.Address, nG.Address),
				compareStrings("Push Username", oG.PushUsername, nG.PushUsername), comparePasswords("Push Password", oG.PushPassword, nG.PushPassword),
				compareStrings("Pull Username", oG.PullUsername, nG.PullUsername), comparePasswords("Push Password", oG.PullPassword, nG.PullPassword))
		case ArtifactKey:
			oA := oldState.ArtifactServer
			nA := newState.ArtifactServer
			Title("Artifact Server", "the information used to interact with Zarf's Artifact Server")
			pterm.Println()
			logger.Info("Artifact info",
				compareStrings("URL Address", oA.Address, nA.Address),
				compareStrings("Push Username", oA.PushUsername, nA.PushUsername),
				comparePasswords("Push Token", oA.PushToken, nA.PushToken))
			// case AgentKey:
			// 	oT := oldState.AgentTLS
			// 	nT := newState.AgentTLS
			// 	Title("Agent TLS", "the certificates used to connect to Zarf's Agent")
			// 	pterm.Println()
			// 	pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Certificate Authority"), compareStrings(string(oT.CA), string(nT.CA), true))
			// 	pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Public Certificate"), compareStrings(string(oT.Cert), string(nT.Cert), true))
			// 	pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Private Key"), compareStrings(string(oT.Key), string(nT.Key), true))
		}
	}

	pterm.Println()
}

func compareStrings(attribute string, old string, new string) slog.Attr {
	if new == old {
		return slog.String(attribute, "(unchanged) "+old)
	}
	return slog.Group(attribute, "old", old, "new", new)
}

func comparePasswords(attribute string, old types.Password, new types.Password) slog.Attr {
	if new == old {
		return slog.Any(attribute, old)
	}
	return slog.Group(attribute, "old", old, "new", new)
}
