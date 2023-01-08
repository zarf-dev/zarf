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
	"sync"
)

// Change terminal colors.
const colorReset = "\x1b[0m"
const colorGreen = "\x1b[32;1m"
const colorCyan = "\x1b[36;1m"
const colorWhite = "\x1b[37;1m"

// Config is a struct for configuring the Cmd function.
type Config struct {
	Print bool
	Dir   string
	Env   []string
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

	if config.Print {
		fmt.Println()
		fmt.Printf("  %s", colorGreen)
		fmt.Print(command + " ")
		fmt.Printf("%s", colorCyan)
		fmt.Printf("%v", args)
		fmt.Printf("%s", colorWhite)
		fmt.Printf("%s", colorReset)
		fmt.Println("")
	}

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = config.Dir
	cmd.Env = append(os.Environ(), config.Env...)

	var stdoutBuf, stderrBuf bytes.Buffer
	stdoutIn, _ := cmd.StdoutPipe()
	stderrIn, _ := cmd.StderrPipe()

	var errStdout, errStderr error
	stdout := io.MultiWriter(os.Stdout, &stdoutBuf)
	stderr := io.MultiWriter(os.Stderr, &stderrBuf)

	if err := cmd.Start(); err != nil {
		return "", "", err
	}

	if config.Print {
		var wg sync.WaitGroup
		wg.Add(1)

		go func() {
			_, errStdout = io.Copy(stdout, stdoutIn)
			wg.Done()
		}()

		_, errStderr = io.Copy(stderr, stderrIn)
		wg.Wait()
	}

	if err := cmd.Wait(); err != nil {
		return "", "", err
	}

	if config.Print {
		if errStdout != nil || errStderr != nil {
			return "", "", errors.New("unable to capture stdOut or stdErr")
		}
	}

	return stdoutBuf.String(), stderrBuf.String(), nil
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
