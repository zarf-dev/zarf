package utils

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

// Change terminal colors
const colorReset = "\x1b[0m"
const colorGreen = "\x1b[32;1m"
const colorCyan = "\x1b[36;1m"
const colorWhite = "\x1b[37;1m"

// ExecCommandWithContext executes a given command with args in the current working directory.
func ExecCommandWithContext(ctx context.Context, showLogs bool, commandName string, args ...string) (string, string, error) {
	return ExecCommandWithContextAndDir(ctx, "", showLogs, commandName, args...)
}

// ExecCommandWithContextAndDir executes a given command with args in the specified directory.
func ExecCommandWithContextAndDir(ctx context.Context, dir string, showLogs bool, commandName string, args ...string) (string, string, error) {
	if showLogs {
		fmt.Println()
		fmt.Printf("  %s", colorGreen)
		fmt.Print(commandName + " ")
		fmt.Printf("%s", colorCyan)
		fmt.Printf("%v", args)
		fmt.Printf("%s", colorWhite)
		fmt.Printf("%s", colorReset)
		fmt.Println("")
	}

	cmd := exec.CommandContext(ctx, commandName, args...)

	env := os.Environ()
	cmd.Env = env
	cmd.Dir = dir

	var stdoutBuf, stderrBuf bytes.Buffer
	stdoutIn, _ := cmd.StdoutPipe()
	stderrIn, _ := cmd.StderrPipe()

	var errStdout, errStderr error
	stdout := io.MultiWriter(os.Stdout, &stdoutBuf)
	stderr := io.MultiWriter(os.Stderr, &stderrBuf)

	if err := cmd.Start(); err != nil {
		return "", "", err
	}

	if showLogs {
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

	if showLogs {
		if errStdout != nil || errStderr != nil {
			return "", "", errors.New("unable to capture stdOut or stdErr")
		}
	}

	return stdoutBuf.String(), stderrBuf.String(), nil
}

func ExecLaunchURL(url string) error {
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
