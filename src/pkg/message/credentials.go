// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package message provides a rich set of functions for displaying messages to the user.
package message

import (
	"context"
	"fmt"
	"strings"

	"github.com/pterm/pterm"
	"github.com/zarf-dev/zarf/src/pkg/logger"
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
			[]string{"Registry", state.RegistryInfo.PushUsername, state.RegistryInfo.PushPassword, "zarf connect registry", RegistryKey},
			[]string{"Registry (read-only)", state.RegistryInfo.PullUsername, state.RegistryInfo.PullPassword, "zarf connect registry", RegistryReadKey},
		)
	}

	for _, component := range componentsToDeploy {
		// Show message if including git-server
		if component.Name == "git-server" {
			loginData = append(loginData,
				[]string{"Git", state.GitServer.PushUsername, state.GitServer.PushPassword, "zarf connect git", GitKey},
				[]string{"Git (read-only)", state.GitServer.PullUsername, state.GitServer.PullPassword, "zarf connect git", GitReadKey},
				[]string{"Artifact Token", state.ArtifactServer.PushUsername, state.ArtifactServer.PushToken, "zarf connect git", ArtifactKey},
			)
		}
	}

	if len(loginData) > 0 {
		header := []string{"Application", "Username", "Password", "Connect", "Get-Creds Key"}
		Table(header, loginData)
	}
}

// PrintComponentCredential displays credentials for a single component
func PrintComponentCredential(ctx context.Context, state *types.ZarfState, componentName string) {
	l := logger.From(ctx)
	switch strings.ToLower(componentName) {
	case GitKey:
		Notef("Git Server push password (username: %s):", state.GitServer.PushUsername)
		l.Info("Git server push password", "username", state.GitServer.PushUsername)
		fmt.Println(state.GitServer.PushPassword)
	case GitReadKey:
		Notef("Git Server (read-only) password (username: %s):", state.GitServer.PullUsername)
		l.Info("Git server (read-only) password", "username", state.GitServer.PullUsername)
		fmt.Println(state.GitServer.PullPassword)
	case ArtifactKey:
		Notef("Artifact Server token (username: %s):", state.ArtifactServer.PushUsername)
		l.Info("artifact server token", "username", state.ArtifactServer.PushUsername)
		fmt.Println(state.ArtifactServer.PushToken)
	case RegistryKey:
		Notef("Image Registry password (username: %s):", state.RegistryInfo.PushUsername)
		l.Info("image registry password", "username", state.RegistryInfo.PushUsername)
		fmt.Println(state.RegistryInfo.PushPassword)
	case RegistryReadKey:
		Notef("Image Registry (read-only) password (username: %s):", state.RegistryInfo.PullUsername)
		l.Info("image registry (read-only) password", "username", state.RegistryInfo.PullUsername)
		fmt.Println(state.RegistryInfo.PullPassword)
	default:
		Warn("Unknown component: " + componentName)
		l.Warn("unknown component", "component", componentName)
	}
}

// PrintCredentialUpdates displays credentials that will be updated
func PrintCredentialUpdates(ctx context.Context, oldState *types.ZarfState, newState *types.ZarfState, services []string) {
	// Pause the logfile's output to avoid credentials being printed to the log file
	l := logger.From(ctx)
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
			l.Info("registry URL address", "existing", oR.Address, "replacement", nR.Address)
			l.Info("registry push username", "existing", oR.PushUsername, "replacement", nR.PushUsername)
			l.Info("registry push password (values redacted)", "changed", !(oR.PushPassword == nR.PushPassword))
			l.Info("registry pull username", "existing", oR.PullUsername, "replacement", nR.PullUsername)
			l.Info("registry pull password (values redacted)", "changed", !(oR.PullPassword == nR.PullPassword))
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
			l.Info("Git server URL address", "existing", oG.Address, "replacement", nG.Address)
			l.Info("Git server push username", "existing", oG.PushUsername, "replacement", nG.PushUsername)
			l.Info("Git server push password (values redacted)", "changed", !(oG.PushPassword == nG.PushPassword))
			l.Info("Git server pull username", "existing", oG.PullUsername, "replacement", nG.PullUsername)
			l.Info("Git server pull password (values redacted)", "changed", !(oG.PullPassword == nG.PullPassword))
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
			l.Info("artifact server URL address", "existing", oA.Address, "replacement", nA.Address)
			l.Info("artifact server push username", "existing", oA.PushUsername, "replacement", nA.PushUsername)
			l.Info("artifact server push token (values redacted)", "changed", !(oA.PushToken == nA.PushToken))
			Title("Artifact Server", "the information used to interact with Zarf's Artifact Server")
			pterm.Println()
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("URL Address"), compareStrings(oA.Address, nA.Address, false))
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Push Username"), compareStrings(oA.PushUsername, nA.PushUsername, false))
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Push Token"), compareStrings(oA.PushToken, nA.PushToken, true))
		case AgentKey:
			oT := oldState.AgentTLS
			nT := newState.AgentTLS
			l.Info("agent certificate authority (values redacted)", "changed", !(string(oT.CA) == string(nT.CA)))
			l.Info("agent public certificate (values redacted)", "changed", !(string(oT.Cert) == string(nT.Cert)))
			l.Info("agent private key (values redacted)", "changed", !(string(oT.Key) == string(nT.Key)))
			Title("Agent TLS", "the certificates used to connect to Zarf's Agent")
			pterm.Println()
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Certificate Authority"), compareStrings(string(oT.CA), string(nT.CA), true))
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Public Certificate"), compareStrings(string(oT.Cert), string(nT.Cert), true))
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Private Key"), compareStrings(string(oT.Key), string(nT.Key), true))
		}
	}

	pterm.Println()
}

func compareStrings(old string, new string, secret bool) string {
	if new == old {
		if secret {
			return "**sanitized** (unchanged)"
		}
		return fmt.Sprintf("%s (unchanged)", old)
	}
	if secret {
		return fmt.Sprintf("%s -> %s", pterm.FgRed.Sprint("**existing (sanitized)**"), pterm.FgGreen.Sprint("**replacement (sanitized)**"))
	}
	return fmt.Sprintf("%s -> %s", pterm.FgRed.Sprint(old), pterm.FgGreen.Sprint(new))
}
