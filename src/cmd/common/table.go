// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package common

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/fatih/color"
	"github.com/pterm/pterm"

	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/types"
)

// Table prints a padded table containing the specified header and data
func Table(header []string, data [][]string) {
	pterm.Println()

	// To avoid side effects make copies of the header and data before adding padding
	headerCopy := make([]string, len(header))
	copy(headerCopy, header)
	dataCopy := make([][]string, len(data))
	copy(dataCopy, data)
	if len(headerCopy) > 0 {
		headerCopy[0] = fmt.Sprintf("     %s", headerCopy[0])
	}

	table := pterm.TableData{
		headerCopy,
	}

	for _, row := range dataCopy {
		if len(row) > 0 {
			row[0] = fmt.Sprintf("     %s", row[0])
		}
		table = append(table, pterm.TableData{row}...)
	}

	//nolint:errcheck // never returns an error
	pterm.DefaultTable.WithHasHeader().WithData(table).Render()
}

// PrintFindings prints the findings in the LintError as a table.
func PrintFindings(lintErr *lint.LintError) {
	mapOfFindingsByPath := lint.GroupFindingsByPath(lintErr.Findings, lintErr.PackageName)
	for _, findings := range mapOfFindingsByPath {
		lintData := [][]string{}
		for _, finding := range findings {
			sevColor := color.FgWhite
			switch finding.Severity {
			case lint.SevErr:
				sevColor = color.FgRed
			case lint.SevWarn:
				sevColor = color.FgYellow
			}

			lintData = append(lintData, []string{
				colorWrap(string(finding.Severity), sevColor),
				colorWrap(finding.YqPath, color.FgCyan),
				finding.ItemizedDescription(),
			})
		}
		var packagePathFromUser string
		if helpers.IsOCIURL(findings[0].PackagePathOverride) {
			packagePathFromUser = findings[0].PackagePathOverride
		} else {
			packagePathFromUser = filepath.Join(lintErr.BaseDir, findings[0].PackagePathOverride)
		}
		message.Notef("Linting package %q at %s", findings[0].PackageNameOverride, packagePathFromUser)
		Table([]string{"Type", "Path", "Message"}, lintData)
	}
}

func colorWrap(str string, attr color.Attribute) string {
	if !message.ColorEnabled() || str == "" {
		return str
	}
	return fmt.Sprintf("\x1b[%dm%s\x1b[0m", attr, str)
}

// PrintConnectStringTable prints a table of connect strings.
func PrintConnectStringTable(connectStrings types.ConnectStrings) {
	if len(connectStrings) == 0 {
		return
	}

	connectData := [][]string{}
	for name, connect := range connectStrings {
		name = fmt.Sprintf("zarf connect %s", name)
		connectData = append(connectData, []string{name, connect.Description})
	}
	header := []string{"Connect Command", "Description"}
	Table(header, connectData)
}

// PrintCredentialTable displays credentials in a table
func PrintCredentialTable(state *types.ZarfState, componentsToDeploy []types.DeployedComponent) {
	if len(componentsToDeploy) == 0 {
		componentsToDeploy = []types.DeployedComponent{{Name: "git-server"}}
	}

	// Pause the logfile's output to avoid credentials being printed to the log file
	// if logFile != nil {
	// 	logFile.Pause()
	// 	defer logFile.Resume()
	// }

	loginData := [][]string{}
	if state.RegistryInfo.IsInternal() {
		loginData = append(loginData,
			[]string{"Registry", state.RegistryInfo.PushUsername, state.RegistryInfo.PushPassword, "zarf connect registry", cluster.RegistryKey},
			[]string{"Registry (read-only)", state.RegistryInfo.PullUsername, state.RegistryInfo.PullPassword, "zarf connect registry", cluster.RegistryReadKey},
		)
	}

	for _, component := range componentsToDeploy {
		// Show message if including git-server
		if component.Name == "git-server" {
			loginData = append(loginData,
				[]string{"Git", state.GitServer.PushUsername, state.GitServer.PushPassword, "zarf connect git", cluster.GitKey},
				[]string{"Git (read-only)", state.GitServer.PullUsername, state.GitServer.PullPassword, "zarf connect git", cluster.GitReadKey},
				[]string{"Artifact Token", state.ArtifactServer.PushUsername, state.ArtifactServer.PushToken, "zarf connect git", cluster.ArtifactKey},
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
	case cluster.GitKey:
		message.Notef("Git Server push password (username: %s):", state.GitServer.PushUsername)
		fmt.Println(state.GitServer.PushPassword)
	case cluster.GitReadKey:
		message.Notef("Git Server (read-only) password (username: %s):", state.GitServer.PullUsername)
		fmt.Println(state.GitServer.PullPassword)
	case cluster.ArtifactKey:
		message.Notef("Artifact Server token (username: %s):", state.ArtifactServer.PushUsername)
		fmt.Println(state.ArtifactServer.PushToken)
	case cluster.RegistryKey:
		message.Notef("Image Registry password (username: %s):", state.RegistryInfo.PushUsername)
		fmt.Println(state.RegistryInfo.PushPassword)
	case cluster.RegistryReadKey:
		message.Notef("Image Registry (read-only) password (username: %s):", state.RegistryInfo.PullUsername)
		fmt.Println(state.RegistryInfo.PullPassword)
	default:
		message.Warn("Unknown component: " + componentName)
	}
}

// PrintCredentialUpdates displays credentials that will be updated
func PrintCredentialUpdates(oldState *types.ZarfState, newState *types.ZarfState, services []string) {
	for _, service := range services {
		message.HorizontalRule()

		switch service {
		case cluster.RegistryKey:
			oR := oldState.RegistryInfo
			nR := newState.RegistryInfo
			message.Title("Registry", "the information used to interact with Zarf's container image registry")
			pterm.Println()
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("URL Address"), compareStrings(oR.Address, nR.Address, false))
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Push Username"), compareStrings(oR.PushUsername, nR.PushUsername, false))
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Push Password"), compareStrings(oR.PushPassword, nR.PushPassword, true))
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Pull Username"), compareStrings(oR.PullUsername, nR.PullUsername, false))
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Pull Password"), compareStrings(oR.PullPassword, nR.PullPassword, true))
		case cluster.GitKey:
			oG := oldState.GitServer
			nG := newState.GitServer
			message.Title("Git Server", "the information used to interact with Zarf's GitOps Git Server")
			pterm.Println()
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("URL Address"), compareStrings(oG.Address, nG.Address, false))
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Push Username"), compareStrings(oG.PushUsername, nG.PushUsername, false))
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Push Password"), compareStrings(oG.PushPassword, nG.PushPassword, true))
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Pull Username"), compareStrings(oG.PullUsername, nG.PullUsername, false))
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Pull Password"), compareStrings(oG.PullPassword, nG.PullPassword, true))
		case cluster.ArtifactKey:
			oA := oldState.ArtifactServer
			nA := newState.ArtifactServer
			message.Title("Artifact Server", "the information used to interact with Zarf's Artifact Server")
			pterm.Println()
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("URL Address"), compareStrings(oA.Address, nA.Address, false))
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Push Username"), compareStrings(oA.PushUsername, nA.PushUsername, false))
			pterm.Printfln("    %s: %s", pterm.Bold.Sprint("Push Token"), compareStrings(oA.PushToken, nA.PushToken, true))
		case cluster.AgentKey:
			oT := oldState.AgentTLS
			nT := newState.AgentTLS
			message.Title("Agent TLS", "the certificates used to connect to Zarf's Agent")
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
