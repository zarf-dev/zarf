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
			loginTable = append(loginTable, pterm.TableData{{"     Logging", "zarf-admin", state.LoggingSecret, "zarf connect logging", loggingKey}}...)
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
		message.Note("Logging credentials (username: zarf-admin):")
		fmt.Println(state.LoggingSecret)
	case gitKey:
		message.Note("Git Server push password (username: " + state.GitServer.PushUsername + "):")
		fmt.Println(state.GitServer.PushPassword)
	case gitReadKey:
		message.Note("Git Server (read-only) password (username: " + state.GitServer.PullUsername + "):")
		fmt.Println(state.GitServer.PullPassword)
	case artifactKey:
		message.Note("Artifact Server token (username: " + state.ArtifactServer.PushUsername + "):")
		fmt.Println(state.ArtifactServer.PushToken)
	case registryKey:
		message.Note("Image Registry password (username: " + state.RegistryInfo.PushUsername + "):")
		fmt.Println(state.RegistryInfo.PushPassword)
	case registryReadKey:
		message.Note("Image Registry (read-only) password (username: " + state.RegistryInfo.PullUsername + "):")
		fmt.Println(state.RegistryInfo.PullPassword)
	default:
		message.Warn("Unknown component: " + componentName)
	}
}
