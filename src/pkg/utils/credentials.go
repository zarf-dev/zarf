// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic utility functions.
package utils

import (
	"fmt"
	"os"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/pterm/pterm"
)

const (
	loggingUsername = "zarf-admin"

	registryKey     = "registry"
	registryReadKey = "registry-readonly"
	gitKey          = "git"
	gitReadKey      = "git-readonly"
	artifactKey     = "artifact"
	loggingKey      = "logging"
)

// PrintCredentialTable displays credentials in a table
func PrintCredentialTable(state types.ZarfState, componentsToDeploy []types.DeployedComponent) {
	if len(componentsToDeploy) == 0 {
		componentsToDeploy = []types.DeployedComponent{{Name: "logging"}, {Name: "git-server"}}
	}

	// Set output to os.Stderr to avoid creds being printed in logs
	pterm.SetDefaultOutput(os.Stderr)

	pterm.Println()
	loginTableHeader := pterm.TableData{
		{"     Application", "Username", "Password", "Connect", "Get-Creds Key"},
	}

	loginTable := pterm.TableData{}
	if state.RegistryInfo.InternalRegistry {
		loginTable = append(loginTable, pterm.TableData{
			{"     Registry", state.RegistryInfo.PushUsername, state.RegistryInfo.PushPassword, "zarf connect registry", registryKey},
			{"     Registry (read-only)", state.RegistryInfo.PullUsername, state.RegistryInfo.PullPassword, "zarf connect registry", registryReadKey},
		}...)
	}

	for _, component := range componentsToDeploy {
		// Show message if including logging stack
		if component.Name == "logging" {
			loginTable = append(loginTable, pterm.TableData{{"     Logging", loggingUsername, state.LoggingSecret, "zarf connect logging", loggingKey}}...)
		}
		// Show message if including git-server
		if component.Name == "git-server" {
			loginTable = append(loginTable, pterm.TableData{
				{"     Git", state.GitServer.PushUsername, state.GitServer.PushPassword, "zarf connect git", gitKey},
				{"     Git (read-only)", state.GitServer.PullUsername, state.GitServer.PullPassword, "zarf connect git", gitReadKey},
				{"     Artifact Token", state.ArtifactServer.PushUsername, state.ArtifactServer.PushToken, "zarf connect git", artifactKey},
			}...)
		}
	}

	if len(loginTable) > 0 {
		loginTable = append(loginTableHeader, loginTable...)
		_ = pterm.DefaultTable.WithHasHeader().WithData(loginTable).Render()
	}

	// Restore the log file if it was specified
	if !config.SkipLogFile {
		message.UseLogFile()
	}
}

// PrintComponentCredential displays credentials for a single component
func PrintComponentCredential(state types.ZarfState, componentName string) {
	switch strings.ToLower(componentName) {
	case loggingKey:
		message.Notef("Logging credentials (username: %s):", loggingUsername)
		fmt.Println(state.LoggingSecret)
	case gitKey:
		message.Notef("Git Server push password (username: %s):", state.GitServer.PushUsername)
		fmt.Println(state.GitServer.PushPassword)
	case gitReadKey:
		message.Notef("Git Server (read-only) password (username: %s):", state.GitServer.PullUsername)
		fmt.Println(state.GitServer.PullPassword)
	case artifactKey:
		message.Notef("Artifact Server token (username: %s):", state.ArtifactServer.PushUsername)
		fmt.Println(state.ArtifactServer.PushToken)
	case registryKey:
		message.Notef("Image Registry password (username: %s):", state.RegistryInfo.PushUsername)
		fmt.Println(state.RegistryInfo.PushPassword)
	case registryReadKey:
		message.Notef("Image Registry (read-only) password (username: %s):", state.RegistryInfo.PullUsername)
		fmt.Println(state.RegistryInfo.PullPassword)
	default:
		message.Warn("Unknown component: " + componentName)
	}
}

// PrintCredentialUpdates displays credentials that will be updated
func PrintCredentialUpdates(oldState types.ZarfState, newState types.ZarfState, services []string) {
	if len(services) == 0 {
		services = []string{registryKey, gitKey, artifactKey, loggingKey}
	}

	// Set output to os.Stderr to avoid creds being printed in logs
	pterm.SetDefaultOutput(os.Stderr)

	for _, service := range services {

		message.HorizontalRule()

		switch service {
		case registryKey:
			oR := oldState.RegistryInfo
			nR := newState.RegistryInfo
			message.Title("Registry", "the information used to interact with Zarf's container image registry")
			pterm.Println()
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("URL Address"), compareStrings(oR.Address, nR.Address, false))
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Push Username"), compareStrings(oR.PushUsername, nR.PushUsername, false))
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Push Password"), compareStrings(oR.PushPassword, nR.PushPassword, true))
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Pull Username"), compareStrings(oR.PullUsername, nR.PullUsername, false))
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Pull Password"), compareStrings(oR.PullPassword, nR.PullPassword, true))
		case gitKey:
			oG := oldState.GitServer
			nG := newState.GitServer
			message.Title("Git Server", "the information used to interact with Zarf's GitOps Git Server")
			pterm.Println()
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("URL Address"), compareStrings(oG.Address, nG.Address, false))
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Push Username"), compareStrings(oG.PushUsername, nG.PushUsername, false))
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Push Password"), compareStrings(oG.PushPassword, nG.PushPassword, true))
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Pull Username"), compareStrings(oG.PullUsername, nG.PullUsername, false))
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Pull Password"), compareStrings(oG.PullPassword, nG.PullPassword, true))
		case artifactKey:
			oA := oldState.ArtifactServer
			nA := newState.ArtifactServer
			message.Title("Artifact Server", "the information used to interact with Zarf's Artifact Server")
			pterm.Println()
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("URL Address"), compareStrings(oA.Address, nA.Address, false))
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Push Username"), compareStrings(oA.PushUsername, nA.PushUsername, false))
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Push Token"), compareStrings(oA.PushToken, nA.PushToken, true))
		case loggingKey:
			oL := oldState.LoggingSecret
			nL := newState.LoggingSecret
			message.Title("Logging", "the information used to interact with Zarf's Logging Stack")
			pterm.Println()
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Logging Secret"), compareStrings(oL, nL, true))
		}
	}

	pterm.Println()

	// Restore the log file if it was specified
	if !config.SkipLogFile {
		message.UseLogFile()
	}
}

func compareStrings(old string, new string, secret bool) string {
	if new == "" {
		if secret {
			return fmt.Sprintf("%s -> %s", pterm.FgRed.Sprint("**existing (sanitized)**"), pterm.FgGreen.Sprint("**auto-generated**"))
		}
		return fmt.Sprintf("%s (unchanged)", old)
	}
	if secret {
		return fmt.Sprintf("%s -> %s", pterm.FgRed.Sprint("**existing (sanitized)**"), pterm.FgGreen.Sprint("**provided (sanitized)**"))
	}
	return fmt.Sprintf("%s -> %s", pterm.FgRed.Sprint(old), pterm.FgGreen.Sprint(new))
}
