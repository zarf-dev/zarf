// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package wait provides functions for waiting on Kubernetes resources and network endpoints.
package wait

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/utils/exec"
)

// isJSONPathWaitType checks if the condition is a JSONPath or condition.
func isJSONPathWaitType(condition string) bool {
	return len(condition) >= 2 && (condition[0] == '{' || condition[1] == '{') && strings.Contains(condition, "=") && strings.Contains(condition, "}")
}

// unsafeShellCharsRegex matches any character that is NOT a letter, digit, underscore (/w) or shell safe special characters
var unsafeShellCharsRegex = regexp.MustCompile(`[^\w@%+=:,./-]`)

// Source: https://github.com/alessio/shellescape/blob/v1.6.0/shellescape.go#L30-L42
// SPDX-License-Identifier: MIT
// Minor edits: Simplified for use case
func shellQuote(s string) string {
	if len(s) == 0 {
		return "''"
	}
	if unsafeShellCharsRegex.MatchString(s) {
		return fmt.Sprintf("'%s'", strings.ReplaceAll(s, "'", "'\"'\"'"))
	}
	return s
}

// ForResource waits for a Kubernetes resource to meet the specified condition using kubectl wait.
func ForResource(ctx context.Context, kind, identifier, condition, namespace string, timeout time.Duration) error {
	l := logger.From(ctx)
	waitInterval := time.Second

	// Type of wait, condition or JSONPath
	var waitType string

	// Check if waitType is JSONPath or condition
	if isJSONPathWaitType(condition) {
		waitType = "jsonpath="
		// Strip any existing shell quotes before re-quoting for the shell command
		condition = strings.ReplaceAll(condition, "'", "")
		condition = shellQuote(condition)
	} else {
		waitType = "condition="
	}

	// Get the Zarf command configuration.
	zarfCommand, err := utils.GetFinalExecutableCommand()
	if err != nil {
		return fmt.Errorf("could not locate the current Zarf binary path: %w", err)
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
	if namespace != "" {
		namespaceFlag = fmt.Sprintf("-n %s", namespace)
		namespaceMsg = fmt.Sprintf(" in namespace %s", namespace)
	}

	// Setup the spinner messages.
	conditionMsg := fmt.Sprintf("waiting for %s%s to be %s.", path.Join(kind, identifierMsg), namespaceMsg, condition)
	existMsg := fmt.Sprintf("waiting for %s%s to exist.", path.Join(kind, identifierMsg), namespaceMsg)
	completedMsg := fmt.Sprintf("wait for %s%s complete.", path.Join(kind, identifierMsg), namespaceMsg)

	// Get the OS shell to execute commands in
	shell, shellArgs := exec.GetOSShell(v1alpha1.Shell{Windows: "cmd"})

	l.Info(existMsg)
	for {
		// Delay the check for 1 second
		time.Sleep(waitInterval)

		select {
		case <-expired:
			return errors.New("wait timed out")
		case <-ctx.Done():
			return errors.New("received interrupt")

		default:
			// Check if the resource exists.
			l.Debug("checking resource existence", "namespace", namespaceFlag, "kind", kind, "identifier", identifier)
			zarfKubectlGet := fmt.Sprintf("%s tools kubectl get %s %s %s", zarfCommand, namespaceFlag, kind, identifier)
			cmd := append(shellArgs, zarfKubectlGet)
			stdout, stderr, err := exec.Cmd(shell, cmd...)
			l.Debug("cmd done", "cmd", cmd, "stdout", stdout, "stderr", stderr, "error", err)
			if err != nil {
				if strings.Contains(stderr, "connect: connection refused") {
					l.Info("api server unavailable")
					continue
				}
				// otherwise just log and retry
				l.Info("resource error", "error", err)
				continue
			}

			resourceNotFound := strings.Contains(stderr, "No resources found")
			if resourceNotFound {
				l.Debug("resource not found", "error", err)
				continue
			}

			// If only checking for existence, exit here.
			switch condition {
			case "", "exist", "exists":
				return nil
			}

			l.Info(conditionMsg)
			// Wait for the resource to meet the given condition.
			zarfKubectlWait := fmt.Sprintf("%s tools kubectl wait %s %s %s --for %s%s --timeout=%ds",
				zarfCommand, namespaceFlag, kind, identifier, waitType, condition, int(timeout.Seconds()))

			// If there is an error, log it and try again.
			waitCmd := append(shellArgs, zarfKubectlWait)
			waitStdout, waitStderr, err := exec.Cmd(shell, waitCmd...)
			l.Debug("wait done", "cmd", waitCmd, "stdout", waitStdout, "stderr", waitStderr, "error", err)
			if err != nil {
				l.Debug("wait error", "error", err)
				continue
			}

			// And just like that, success!
			l.Info(completedMsg)
			return nil
		}
	}
}

// ForNetwork waits for a network endpoint to respond.
func ForNetwork(ctx context.Context, protocol, address, condition string, timeout time.Duration) error {
	waitInterval := time.Second
	return forNetwork(ctx, protocol, address, condition, timeout, waitInterval)
}

func forNetwork(ctx context.Context, protocol string, address string, condition string, timeout time.Duration, waitInterval time.Duration) error {
	l := logger.From(ctx)
	expired := time.After(timeout)

	condition = strings.ToLower(condition)
	if condition == "" {
		condition = "success"
	}

	// Create an HTTP client with a per-request timeout that is slightly shorter than our wait-interval to prevent
	// hanging on slow or unresponsive servers.
	httpClient := &http.Client{
		Timeout: waitInterval - (time.Millisecond * 5),
	}

	delay := 100 * time.Millisecond

	for {
		// Delay the check for 100ms the first time and then the wait interval after that
		time.Sleep(delay)
		delay = waitInterval

		select {
		case <-expired:
			return errors.New("wait timed out")
		case <-ctx.Done():
			return errors.New("received interrupt")
		default:
			switch protocol {
			case "http", "https":
				// Handle HTTP and HTTPS endpoints.
				url := fmt.Sprintf("%s://%s", protocol, address)

				// Default to checking for a 2xx response.
				if condition == "success" {
					// Try to get the URL and check the status code.
					resp, err := httpClient.Get(url)
					if err != nil {
						l.Debug(err.Error())
						continue
					}
					err = resp.Body.Close()
					if err != nil {
						l.Debug(err.Error())
					}

					// If the status code is not in the 2xx range, try again.
					if resp.StatusCode < 200 || resp.StatusCode > 299 {
						l.Debug("did not receive 2xx status code", "response_code", resp.StatusCode)
						continue
					}

					// Success, break out of the switch statement.
					break
				}

				// Convert the condition to an int and check if it's a valid HTTP status code.
				code, err := strconv.Atoi(condition)
				if err != nil {
					return fmt.Errorf("http status code %s is not an integer: %w", condition, err)
				}
				if http.StatusText(code) == "" {
					return errors.New("http status code is unknown")
				}

				// Try to get the URL and check the status code.
				resp, err := httpClient.Get(url)
				if err != nil {
					l.Debug(err.Error())
					continue
				}
				err = resp.Body.Close()
				if err != nil {
					l.Debug(err.Error())
				}

				if resp.StatusCode != code {
					l.Debug("did not receive expected status code", "expected", code, "actual", resp.StatusCode)
					continue
				}
			default:
				// Fallback to any generic protocol using net.Dial
				conn, err := net.Dial(protocol, address)
				if err != nil {
					l.Debug(err.Error())
					continue
				}
				err = conn.Close()
				if err != nil {
					l.Debug(err.Error())
					continue
				}
			}

			// Yay, we made it!
			return nil
		}
	}
}
