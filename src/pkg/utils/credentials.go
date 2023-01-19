package utils

import (
	"fmt"
	"strings"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/pterm/pterm"
)

// Display credentials in a table
func PrintCredentialTable(state types.ZarfState, componentsToDeploy []types.DeployedComponent) {
	if len(componentsToDeploy) == 0 {
		componentsToDeploy = append(componentsToDeploy, types.DeployedComponent{Name: "logging"}, types.DeployedComponent{Name: "git-server"})
	}

	pterm.Println()
	loginTableHeader := pterm.TableData{
		{"     Application", "Username", "Password", "Connect"},
	}

	loginTable := pterm.TableData{}
	if state.RegistryInfo.InternalRegistry {
		loginTable = append(loginTable, pterm.TableData{{"     Registry", state.RegistryInfo.PushUsername, state.RegistryInfo.PushPassword, "zarf connect registry"}}...)
	}

	for _, component := range componentsToDeploy {
		// Show message if including logging stack
		if component.Name == "logging" {
			loginTable = append(loginTable, pterm.TableData{{"     Logging", "zarf-admin", state.LoggingSecret, "zarf connect logging"}}...)
		}
		// Show message if including git-server
		if component.Name == "git-server" {
			loginTable = append(loginTable, pterm.TableData{
				{"     Git", state.GitServer.PushUsername, state.GitServer.PushPassword, "zarf connect git"},
				{"     Git (read-only)", state.GitServer.PullUsername, state.GitServer.PullPassword, "zarf connect git"},
			}...)
		}
	}

	if len(loginTable) > 0 {
		loginTable = append(loginTableHeader, loginTable...)
		_ = pterm.DefaultTable.WithHasHeader().WithData(loginTable).Render()
	}
}

// Display credentials for a single component
func PrintComponentCredential(state types.ZarfState, componentName string) {
	switch strings.ToLower(componentName) {
	case "logging":
		message.Note("Logging credentials (username: zarf-admin):")
		fmt.Println(state.LoggingSecret)
	case "git":
		message.Note("Git Server push password (username: " + state.GitServer.PushUsername + "):")
		fmt.Println(state.GitServer.PushPassword)
	case "git-readonly":
		message.Note("Git Server (read-only) password (username: " + state.GitServer.PullUsername + "):")
		fmt.Println(state.GitServer.PullPassword)
	case "registry":
		message.Note("Image Registry password (username: " + state.RegistryInfo.PushUsername + "):")
		fmt.Println(state.RegistryInfo.PushPassword)
	default:
		message.Warn("Unknown component: " + componentName)
	}
}
