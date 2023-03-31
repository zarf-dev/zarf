// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package tools contains the CLI commands for Zarf.
package tools

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/exec"
	"github.com/spf13/cobra"

	// Import to initialize client auth plugins.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var (
	waitTimeout   string
	waitNamespace string
)

var waitForCmd = &cobra.Command{
	Use:     "wait-for {KIND|PROTOCOL} {NAME|SELECTOR|URI} {CONDITION|HTTP_CODE}",
	Aliases: []string{"w", "wait"},
	Short:   lang.CmdToolsWaitForShort,
	Long:    lang.CmdToolsWaitForLong,
	Example: `    Wait for Kubernetes resources:
        zarf tools wait-for pod my-pod-name ready -n default                    wait for pod my-pod-name in namespace default to be ready
        zarf tools wait-for p cool-pod-name ready -n cool                       wait for pod (using p alias) cool-pod-name in namespace cool to be ready
        zarf tools wait-for deployment podinfo available -n podinfo             wait for deployment podinfo in namespace podinfo to be available
        zarf tools wait-for pod app=podinfo ready -n podinfo                    wait for pod with label app=podinfo in namespace podinfo to be ready
        zarf tools wait-for svc zarf-docker-registry exists -n zarf             wait for service zarf-docker-registry in namespace zarf to exist
        zarf tools wait-for svc zarf-docker-registry -n zarf                    same as above, except exists is the default condition
        zarf tools wati-for crd addons.k3s.cattle.io                            wait for crd addons.k3s.cattle.io to exist

    Wait for network endpoints:
        zarf tools wait-for http localhost:8080 200                             wait for a 200 response from http://localhost:8080
        zarf tools wait-for tcp localhost:8080                                  wait for a connection to be established on localhost:8080
        zarf tools wait-for https 1.1.1.1 200                                   wait for a 200 response from https://1.1.1.1
        zarf tools wait-for http google.com                                     wait for any 2xx response from http://google.com
        zarf tools wait-for http google.com success                             wait for any 2xx response from http://google.com
  `,
	Args: cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		// Parse the timeout string
		timeout, err := time.ParseDuration(waitTimeout)
		if err != nil {
			message.Fatalf(err, lang.CmdToolsWaitForErrTimeoutString, waitTimeout)
		}

		// Parse the kind type and identifier.
		kind, identifier := args[0], args[1]

		// Condition is optional, default to "exists".
		condition := ""
		if len(args) > 2 {
			condition = args[2]
		}

		// Handle network endpoints.
		switch kind {
		case "http", "https", "tcp":
			waitForNetworkEndpoint(kind, identifier, condition, timeout)
			return
		}

		// Get the Zarf executable path.
		zarfBinPath, err := utils.GetFinalExecutablePath()
		if err != nil {
			message.Fatal(err, lang.CmdToolsWaitForErrZarfPath)
		}

		// If the identifier contains an equals sign, convert to a label selector.
		identifierMsg := fmt.Sprintf("/%s", identifier)
		if strings.ContainsRune(identifier, '=') {
			identifierMsg = fmt.Sprintf(" with label `%s`", identifier)
			identifier = fmt.Sprintf("-l %s", identifier)
		}

		// Set the timeout for the wait-for command.
		expired := time.After(timeout)

		// Set the custom message for optional namespace.
		namespaceMsg := ""
		if waitNamespace != "" {
			namespaceMsg = fmt.Sprintf(" in namespace %s", waitNamespace)
		}

		// Setup the spinner messages.
		conditionMsg := fmt.Sprintf("Waiting for %s%s%s to be %s.", kind, identifierMsg, namespaceMsg, condition)
		existMsg := fmt.Sprintf("Waiting for %s%s%s to exist.", kind, identifierMsg, namespaceMsg)
		spinner := message.NewProgressSpinner(existMsg)
		defer spinner.Stop()

		for {
			// Delay the check for 1 second
			time.Sleep(time.Second)

			select {
			case <-expired:
				message.Fatal(nil, lang.CmdToolsWaitForErrTimeout)

			default:
				spinner.Updatef(existMsg)
				// Check if the resource exists.
				args := []string{"tools", "kubectl", "get", "-n", waitNamespace, kind, identifier}
				if stdout, stderr, err := exec.Cmd(zarfBinPath, args...); err != nil {
					message.Debug(stdout, stderr, err)
					continue
				}

				// If only checking for existence, exit here.
				switch condition {
				case "", "exist", "exists":
					spinner.Success()
					return
				}

				spinner.Updatef(conditionMsg)
				// Wait for the resource to meet the given condition.
				args = []string{"tools", "kubectl", "wait", "-n", waitNamespace,
					kind, identifier, "--for", "condition=" + condition,
					"--timeout=" + waitTimeout}

				// If there is an error, log it and try again.
				if stdout, stderr, err := exec.Cmd(zarfBinPath, args...); err != nil {
					message.Debug(stdout, stderr, err)
					continue
				}

				// And just like that, success!
				spinner.Successf(conditionMsg)
				return
			}
		}
	},
}

func waitForNetworkEndpoint(resource, name, condition string, timeout time.Duration) {
	// Set the timeout for the wait-for command.
	expired := time.After(timeout)

	// Setup the spinner messages.
	condition = strings.ToLower(condition)
	if condition == "" {
		condition = "success"
	}
	spinner := message.NewProgressSpinner("Waiting for network endpoint %s://%s to respond %s.", resource, name, condition)
	defer spinner.Stop()

	delay := 100 * time.Millisecond

	for {
		// Delay the check for 100ms the first time and then 1 second after that.
		time.Sleep(delay)
		delay = time.Second

		select {
		case <-expired:
			message.Fatal(nil, lang.CmdToolsWaitForErrTimeout)

		default:
			switch resource {

			case "http", "https":
				// Handle HTTP and HTTPS endpoints.
				url := fmt.Sprintf("%s://%s", resource, name)

				// Default to checking for a 2xx response.
				if condition == "success" {
					// Try to get the URL and check the status code.
					resp, err := http.Get(url)

					// If the status code is not in the 2xx range, try again.
					if err != nil || resp.StatusCode < 200 || resp.StatusCode > 299 {
						message.Debug(err)
						continue
					}

					// Success, break out of the swtich statement.
					break
				}

				// Convert the condition to an int and check if it's a valid HTTP status code.
				code, err := strconv.Atoi(condition)
				if err != nil || http.StatusText(code) == "" {
					message.Fatalf(err, lang.CmdToolsWaitForErrConditionString, condition)
				}

				// Try to get the URL and check the status code.
				resp, err := http.Get(url)
				if err != nil || resp.StatusCode != code {
					message.Debug(err)
					continue
				}

			default:
				// Fallback to any generic protocol using net.Dial
				conn, err := net.Dial(resource, name)
				if err != nil {
					message.Debug(err)
					continue
				}
				defer conn.Close()
			}

			// Yay, we made it!
			spinner.Success()
			return
		}
	}
}

func init() {
	toolsCmd.AddCommand(waitForCmd)
	waitForCmd.Flags().StringVar(&waitTimeout, "timeout", "5m", lang.CmdToolsWaitForFlagTimeout)
	waitForCmd.Flags().StringVarP(&waitNamespace, "namespace", "n", "", lang.CmdToolsWaitForFlagNamespace)
}
