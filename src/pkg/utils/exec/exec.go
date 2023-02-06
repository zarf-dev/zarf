// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package exec provides a wrapper around the os/exec package
package exec

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sync"

	"github.com/defenseunicorns/zarf/src/pkg/message"
)

// Change terminal colors.
const colorReset = "\x1b[0m"
const colorGreen = "\x1b[32;1m"
const colorCyan = "\x1b[36;1m"
const colorWhite = "\x1b[37;1m"

// Config is a struct for configuring the Cmd function.
type Config struct {
	Print   bool
	Dir     string
	Env     []string
	Spinner *message.Spinner
}

// PrintCfg is a helper function for returning a Config struct with Print set to true.
func PrintCfg() Config {
	return Config{Print: true}
}

// Cmd executes a given command with given config.
func Cmd(command string, args ...string) (string, string, error) {
	return CmdWithContext(context.TODO(), Config{}, command, args...)
}

// CmdWithPrint executes a given command with given config and prints the command.
func CmdWithPrint(command string, args ...string) error {
	_, _, err := CmdWithContext(context.TODO(), PrintCfg(), command, args...)
	return err
}

// CmdWithContext executes a given command with given config.
func CmdWithContext(ctx context.Context, config Config, command string, args ...string) (string, string, error) {
	if command == "" {
		return "", "", errors.New("command is required")
	}

	// Print the command if requested.
	if config.Print {
		cmdString := fmt.Sprintf("   %s%s %s%v%s%s", colorGreen, command, colorCyan, args, colorWhite, colorReset)
		if config.Spinner != nil {
			config.Spinner.Println("")
			config.Spinner.Println(cmdString)
			config.Spinner.Println("")
		} else {
			fmt.Println("")
			fmt.Println(cmdString)
			fmt.Println("")
		}
	}

	// Set up the command.
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = config.Dir
	cmd.Env = append(os.Environ(), config.Env...)

	// Capture the command outputs.
	cmdStdout, _ := cmd.StdoutPipe()
	cmdStderr, _ := cmd.StderrPipe()

	var stdoutBuf, stderrBuf bytes.Buffer
	stdout := io.MultiWriter(os.Stdout, &stdoutBuf)
	stderr := io.MultiWriter(os.Stderr, &stderrBuf)

	// Start the command.
	if err := cmd.Start(); err != nil {
		return "", "", err
	}

	// If printing live output, copy the command outputs to stdout/stderr.
	if config.Print {
		var errStdout, errStderr error
		var wg sync.WaitGroup

		// Set the wait group to 2 so we wait for both stdout and stderr.
		wg.Add(2)

		// Run a goroutine to capture the command's stdout live.
		go func() {
			if config.Spinner != nil {
				// TODO: (@WSTARR) refactor this into a helper
				scanner := bufio.NewScanner(cmdStdout)
				scanner.Split(bufio.ScanLines)
				for scanner.Scan() {
					line := scanner.Text()
					if config.Spinner != nil {
						config.Spinner.Println(line)
					}

					// TODO: (@WSTARR) better handle errors
					_, errStdout = fmt.Fprintln(&stdoutBuf, line)
				}
			} else {
				_, errStderr = io.Copy(stdout, cmdStderr)
			}
			wg.Done()
		}()

		// Run a goroutine to capture the command's stderr live.
		go func() {
			if config.Spinner != nil {
				// TODO: (@WSTARR) refactor this into a helper
				scanner := bufio.NewScanner(cmdStderr)
				scanner.Split(bufio.ScanLines)
				for scanner.Scan() {
					line := scanner.Text()
					if config.Spinner != nil {
						config.Spinner.Println(line)
					}

					// TODO: (@WSTARR) better handle errors
					_, errStdout = fmt.Fprintln(&stderrBuf, line)
				}
			} else {
				_, errStderr = io.Copy(stderr, cmdStderr)
			}
			wg.Done()
		}()

		// Wait for the goroutines to finish.
		wg.Wait()

		// Print an empty line to act as a separator for the spinner
		if config.Spinner != nil {
			config.Spinner.Println("")
		}

		// Abort if there was an error capturing the command's outputs.
		if errStdout != nil {
			return "", "", fmt.Errorf("failed to capture the stdout command output: %w", errStdout)
		}
		if errStderr != nil {
			return "", "", fmt.Errorf("failed to capture the stderr command output: %w", errStderr)
		}
	}

	// Wait for the command to finish and return the buffered outputs, regardless of whether we printed them.
	return stdoutBuf.String(), stderrBuf.String(), cmd.Wait()
}

// LaunchURL opens a URL in the default browser.
func LaunchURL(url string) error {
	switch runtime.GOOS {
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	}

	return nil
}
