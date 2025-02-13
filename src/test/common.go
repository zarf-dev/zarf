// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"bufio"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"slices"

	"github.com/defenseunicorns/pkg/helpers/v2"
	_ "github.com/distribution/distribution/v3/registry/storage/driver/inmemory" // used for docker test registry
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/utils/exec"
)

// ZarfE2ETest Struct holding common fields most of the tests will utilize.
type ZarfE2ETest struct {
	ZarfBinPath       string
	Arch              string
	ApplianceMode     bool
	ApplianceModeKeep bool
}

// GetLogger returns the default log configuration for the tests.
func GetLogger(t *testing.T) *slog.Logger {
	t.Helper()
	cfg := logger.Config{
		Level:       logger.Info,
		Format:      logger.FormatConsole,
		Destination: logger.DestinationDefault, // Stderr
		Color:       false,
	}
	l, err := logger.New(cfg)
	require.NoError(t, err)
	return l
}

// GetCLIName looks at the OS and CPU architecture to determine which Zarf binary needs to be run.
func GetCLIName() string {
	var binaryName string
	switch runtime.GOOS {
	case "linux":
		binaryName = "zarf"
	case "darwin":
		switch runtime.GOARCH {
		case "arm64":
			binaryName = "zarf-mac-apple"
		default:
			binaryName = "zarf-mac-intel"
		}
	case "windows":
		if runtime.GOARCH == "amd64" {
			binaryName = "zarf.exe"
		}
	}
	return binaryName
}

// Zarf executes a Zarf command.
func (e2e *ZarfE2ETest) Zarf(t *testing.T, args ...string) (_ string, _ string, err error) {
	return e2e.ZarfInDir(t, "", args...)
}

// ZarfInDir executes a Zarf command in specific directory.
func (e2e *ZarfE2ETest) ZarfInDir(t *testing.T, dir string, args ...string) (_ string, _ string, err error) {
	if !slices.Contains(args, "tools") {
		args = append(args, "--log-format=console", "--no-color")
	}
	if !slices.Contains(args, "--tmpdir") && !slices.Contains(args, "tools") {
		tmpdir, err := os.MkdirTemp("", "zarf-")
		if err != nil {
			return "", "", err
		}
		defer func(path string) {
			errRemove := os.RemoveAll(path)
			err = errors.Join(err, errRemove)
		}(tmpdir)
		args = append(args, "--tmpdir", tmpdir)
	}
	if !slices.Contains(args, "--zarf-cache") && !slices.Contains(args, "tools") && os.Getenv("CI") == "true" {
		// We make the cache dir relative to the working directory to make it work on the Windows Runners
		// - they use two drives which filepath.Rel cannot cope with.
		cwd, err := os.Getwd()
		if err != nil {
			return "", "", err
		}
		cacheDir, err := os.MkdirTemp(cwd, "zarf-")
		if err != nil {
			return "", "", err
		}
		args = append(args, "--zarf-cache", cacheDir)
		defer func(path string) {
			errRemove := os.RemoveAll(path)
			err = errors.Join(err, errRemove)
		}(cacheDir)
	}
	cfg := exec.PrintCfg()
	cfg.Dir = dir
	return exec.CmdWithTesting(t, cfg, e2e.ZarfBinPath, args...)
}

// Kubectl executes `zarf tools kubectl ...`
func (e2e *ZarfE2ETest) Kubectl(t *testing.T, args ...string) (string, string, error) {
	tk := []string{"tools", "kubectl"}
	args = append(tk, args...)
	return e2e.Zarf(t, args...)
}

// CleanFiles removes files and directories that have been created during the test.
func (e2e *ZarfE2ETest) CleanFiles(t *testing.T, files ...string) {
	for _, file := range files {
		err := os.RemoveAll(file)
		require.NoError(t, err)
	}
}

// GetMismatchedArch determines what architecture our tests are running on,
// and returns the opposite architecture.
func (e2e *ZarfE2ETest) GetMismatchedArch() string {
	switch e2e.Arch {
	case "arm64":
		return "amd64"
	default:
		return "arm64"
	}
}

// GetZarfVersion returns the current build/zarf version
func (e2e *ZarfE2ETest) GetZarfVersion(t *testing.T) string {
	// Get the version of the CLI
	stdOut, stdErr, err := e2e.Zarf(t, "version")
	require.NoError(t, err, stdOut, stdErr)
	return strings.Trim(stdOut, "\n")
}

// NormalizeYAMLFilenames normalizes YAML filenames / paths across Operating Systems (i.e Windows vs Linux)
func (e2e *ZarfE2ETest) NormalizeYAMLFilenames(input string) string {
	if runtime.GOOS != "windows" {
		return input
	}

	// Match YAML lines that have files in them https://regex101.com/r/C78kRD/1
	fileMatcher := regexp.MustCompile(`^(?P<start>.* )(?P<file>[^:\n]+\/.*)$`)
	scanner := bufio.NewScanner(strings.NewReader(input))

	output := ""
	for scanner.Scan() {
		line := scanner.Text()
		get, err := helpers.MatchRegex(fileMatcher, line)
		if err != nil {
			output += line + "\n"
			continue
		}
		output += fmt.Sprintf("%s\"%s\"\n", get("start"), strings.ReplaceAll(get("file"), "/", "\\\\"))
	}

	return output
}
