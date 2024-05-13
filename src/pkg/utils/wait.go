// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic helper functions.
package utils

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/event"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
	"sigs.k8s.io/cli-utils/pkg/object"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils/exec"
)

// isJSONPathWaitType checks if the condition is a JSONPath or condition.
func isJSONPathWaitType(condition string) bool {
	if len(condition) == 0 || condition[0] != '{' || !strings.Contains(condition, "=") || !strings.Contains(condition, "}") {
		return false
	}

	return true
}

func ExecuteWaitResource(ctx context.Context, gk schema.GroupKind, namespace, name string) error {
	cfg, err := config.GetConfig()
	if err != nil {
		return err
	}
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{})
	if err != nil {
		return err
	}
	identifiers := []object.ObjMetadata{
		{
			GroupKind: gk,
			Namespace: namespace,
			Name:      name,
		},
	}
	poller := polling.NewStatusPoller(mgr.GetClient(), mgr.GetRESTMapper(), polling.Options{})
	events := poller.Poll(ctx, identifiers, polling.PollOptions{PollInterval: 5 * time.Second})

	spinnerMsg := fmt.Sprintf("Waiting for %s to be ready.", path.Join(gk.String(), name))
	spinner := message.NewProgressSpinner(spinnerMsg)
	defer spinner.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case e := <-events:
			if e.Type == event.ErrorEvent {
				return e.Error
			}
			switch e.Resource.Status {
			case status.CurrentStatus:
				spinner.Successf(e.Resource.Message)
				return nil
			case status.InProgressStatus:
				spinner.Updatef(e.Resource.Message)
			default:
				message.Debug(e.Resource.Message)
			}
		}
	}
}

// ExecuteWait executes the wait-for command.
func ExecuteWait(waitTimeout, waitNamespace, condition, kind, identifier string, timeout time.Duration) error {
	// Handle network endpoints.
	switch kind {
	case "http", "https", "tcp":
		waitForNetworkEndpoint(kind, identifier, condition, timeout)
		return nil
	}

	// Type of wait, condition or JSONPath
	var waitType string

	// Check if waitType is JSONPath or condition
	if isJSONPathWaitType(condition) {
		waitType = "jsonpath="
	} else {
		waitType = "condition="
	}

	// Get the Zarf command configuration.
	zarfCommand, err := GetFinalExecutableCommand()
	if err != nil {
		message.Fatal(err, lang.CmdToolsWaitForErrZarfPath)
	}

	identifierMsg := identifier

	// If the identifier contains an equals sign, convert to a label selector.
	if strings.ContainsRune(identifier, '=') {
		identifierMsg = fmt.Sprintf(" with label `%s`", identifier)
		identifier = fmt.Sprintf("-l %s", identifier)
	}

	// Set the timeout for the wait-for command.
	expired := time.After(timeout)

	// Set the custom message for optional namespace.
	namespaceMsg := ""
	namespaceFlag := ""
	if waitNamespace != "" {
		namespaceFlag = fmt.Sprintf("-n %s", waitNamespace)
		namespaceMsg = fmt.Sprintf(" in namespace %s", waitNamespace)
	}

	// Setup the spinner messages.
	conditionMsg := fmt.Sprintf("Waiting for %s%s%s to be %s.", kind, identifierMsg, namespaceMsg, condition)
	existMsg := fmt.Sprintf("Waiting for %s%s to exist.", path.Join(kind, identifierMsg), namespaceMsg)
	spinner := message.NewProgressSpinner(existMsg)

	// Get the OS shell to execute commands in
	shell, shellArgs := exec.GetOSShell(exec.Shell{Windows: "cmd"})

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
			zarfKubectlGet := fmt.Sprintf("%s tools kubectl get %s %s %s", zarfCommand, namespaceFlag, kind, identifier)
			stdout, stderr, err := exec.Cmd(shell, append(shellArgs, zarfKubectlGet)...)
			if err != nil {
				message.Debug(stdout, stderr, err)
				continue
			}

			resourceNotFound := strings.Contains(stderr, "No resources found") && identifier == ""
			if resourceNotFound {
				message.Debug(stdout, stderr, err)
				continue
			}

			// If only checking for existence, exit here.
			switch condition {
			case "", "exist", "exists":
				spinner.Success()
				return nil
			}

			spinner.Updatef(conditionMsg)
			// Wait for the resource to meet the given condition.
			zarfKubectlWait := fmt.Sprintf("%s tools kubectl wait %s %s %s --for %s%s --timeout=%s",
				zarfCommand, namespaceFlag, kind, identifier, waitType, condition, waitTimeout)

			// If there is an error, log it and try again.
			if stdout, stderr, err := exec.Cmd(shell, append(shellArgs, zarfKubectlWait)...); err != nil {
				message.Debug(stdout, stderr, err)
				continue
			}

			// And just like that, success!
			spinner.Successf(conditionMsg)
			return nil
		}
	}
}

// waitForNetworkEndpoint waits for a network endpoint to respond.
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

					// Success, break out of the switch statement.
					break
				}

				// Convert the condition to an int and check if it's a valid HTTP status code.
				code, err := strconv.Atoi(condition)
				if err != nil || http.StatusText(code) == "" {
					message.Fatal(err, lang.CmdToolsWaitForErrConditionString)
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
