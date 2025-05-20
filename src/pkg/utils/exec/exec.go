// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package exec provides a wrapper around the os/exec package
package exec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
)

// Config is a struct for configuring the Cmd function.
type Config struct {
	Print          bool
	Dir            string
	Env            []string
	CommandPrinter func(format string, a ...any)
	Stdout         io.Writer
	Stderr         io.Writer
}

// PrintCfg is a helper function for returning a Config struct with Print set to true.
func PrintCfg() Config {
	return Config{Print: true}
}

// Cmd executes a given command with given config.
func Cmd(command string, args ...string) (string, string, error) {
	return CmdWithContext(context.Background(), Config{}, command, args...)
}

// CmdWithPrint executes a given command with given config and prints the command.
func CmdWithPrint(command string, args ...string) error {
	_, _, err := CmdWithContext(context.Background(), PrintCfg(), command, args...)
	return err
}

// CmdWithTesting takes a *testing.T and generates a context the cancels on cleanup
func CmdWithTesting(t *testing.T, config Config, command string, args ...string) (string, string, error) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return CmdWithContext(ctx, config, command, args...)
}

// CmdWithContext executes a given command with given config.
func CmdWithContext(ctx context.Context, config Config, command string, args ...string) (string, string, error) {
	if command == "" {
		return "", "", errors.New("command is required")
	}

	// Set up the command.
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = config.Dir
	cmd.Env = append(os.Environ(), config.Env...)

	// Capture the command outputs.
	cmdStdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", "", fmt.Errorf("failed to capture stdout, error=%w", err)
	}
	cmdStderr, err := cmd.StderrPipe()
	if err != nil {
		return "", "", fmt.Errorf("failed to capture stderr, error=%w", err)
	}

	var (
		stdoutBuf, stderrBuf bytes.Buffer
		errStdout, errStderr error
		wg                   sync.WaitGroup
	)

	stdoutWriters := []io.Writer{
		&stdoutBuf,
	}

	stdErrWriters := []io.Writer{
		&stderrBuf,
	}

	// TODO (@austinabro321) remove config options for stdout/stderr once logger is released
	// as these options seem to have been added specifically for the spinner
	// Add the writers if requested.
	if config.Stdout != nil {
		stdoutWriters = append(stdoutWriters, config.Stdout)
	}

	if config.Stderr != nil {
		stdErrWriters = append(stdErrWriters, config.Stderr)
	}

	// Print to stdout if requested.
	if config.Print {
		stdoutWriters = append(stdoutWriters, os.Stdout)
		stdErrWriters = append(stdErrWriters, os.Stderr)
	}

	// Bind all the writers.
	stdout := io.MultiWriter(stdoutWriters...)
	stderr := io.MultiWriter(stdErrWriters...)

	// If we're printing, print the command.
	if config.Print && config.CommandPrinter != nil {
		config.CommandPrinter("%s %s", command, strings.Join(args, " "))
	}

	// Start the command.
	if err := cmd.Start(); err != nil {
		return "", "", err
	}

	// Add to waitgroup for each goroutine.
	wg.Add(2)

	// Run a goroutine to capture the command's stdout live.
	go func() {
		_, errStdout = io.Copy(stdout, cmdStdout)
		wg.Done()
	}()

	// Run a goroutine to capture the command's stderr live.
	go func() {
		_, errStderr = io.Copy(stderr, cmdStderr)
		wg.Done()
	}()

	// Wait for the goroutines to finish (if any).
	wg.Wait()

	// Abort if there was an error capturing the command's outputs.
	if errStdout != nil {
		return "", "", fmt.Errorf("failed to capture the stdout command output: %w", errStdout)
	}
	if errStderr != nil {
		return "", "", fmt.Errorf("failed to capture the stderr command output: %w", errStderr)
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

// GetOSShell returns the shell and shellArgs based on the current OS
func GetOSShell(shellPref v1alpha1.Shell) (string, []string) {
	var shell string
	var shellArgs []string
	powershellShellArgs := []string{"-Command", "$ErrorActionPreference = 'Stop';"}
	shShellArgs := []string{"-e", "-c"}

	switch runtime.GOOS {
	case "windows":
		shell = "powershell"
		if shellPref.Windows != "" {
			shell = shellPref.Windows
		}

		shellArgs = powershellShellArgs
		if shell == "cmd" {
			// Change shellArgs to /c if cmd is chosen
			shellArgs = []string{"/c"}
		} else if !IsPowershell(shell) {
			// Change shellArgs to -c if a real shell is chosen
			shellArgs = shShellArgs
		}
	case "darwin":
		shell = "sh"
		if shellPref.Darwin != "" {
			shell = shellPref.Darwin
		}

		shellArgs = shShellArgs
		if IsPowershell(shell) {
			// Change shellArgs to -Command if pwsh is chosen
			shellArgs = powershellShellArgs
		}
	case "linux":
		shell = "sh"
		if shellPref.Linux != "" {
			shell = shellPref.Linux
		}

		shellArgs = shShellArgs
		if IsPowershell(shell) {
			// Change shellArgs to -Command if pwsh is chosen
			shellArgs = powershellShellArgs
		}
	default:
		shell = "sh"
		shellArgs = shShellArgs
	}

	return shell, shellArgs
}

// IsPowershell returns whether a shell name is powershell
func IsPowershell(shellName string) bool {
	return shellName == "powershell" || shellName == "pwsh"
}
