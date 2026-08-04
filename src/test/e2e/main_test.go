// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/utils/exec"
	"github.com/zarf-dev/zarf/src/test"
	// "github.com/zarf-dev/zarf/src/pkg/utils/exec"
)

var (
	e2e test.ZarfE2ETest //nolint:gochecknoglobals
)

const (
	applianceModeEnvVar     = "APPLIANCE_MODE"
	applianceModeKeepEnvVar = "APPLIANCE_MODE_KEEP"
	skipK8sEnvVar           = "SKIP_K8S"
)

func TestMain(m *testing.M) {
	rootDir, err := filepath.Abs("../../../")
	if err != nil {
		log.Fatal(err)
	}
	if err := os.Chdir(rootDir); err != nil {
		log.Fatal(err)
	}

	e2e.Arch = config.GetArch()
	zarfBinPath, err := filepath.Abs(filepath.Join("build", test.GetCLIName()))
	if err != nil {
		log.Fatal(err)
	}
	e2e.ZarfBinPath = zarfBinPath
	e2e.ApplianceMode = os.Getenv(applianceModeEnvVar) == "true"
	e2e.ApplianceModeKeep = os.Getenv(applianceModeKeepEnvVar) == "true"

	if _, err := os.Stat(e2e.ZarfBinPath); err != nil {
		log.Fatalf("zarf binary %s not found: %v", e2e.ZarfBinPath, err)
	}

	// If APPLIANCE_MODE set to true, warn Mac user and run only non-cluster tests
	if e2e.ApplianceMode && runtime.GOOS != "linux" {
		cfg := logger.Config{
			Level:       logger.Info,
			Format:      logger.FormatConsole,
			Destination: logger.DestinationDefault,
			Color:       true,
		}
		l, err := logger.New(cfg)
		if err != nil {
			log.Fatal(err)
		}
		l.Warn("APPLIANCE_MODE=true is only fully supported on Linux. " +
			"On macOS, only tests that do not require a cluster will run. " +
			"To run with-cluster tests on macOS, create a local cluster first " +
			"and run make test-e2e-with-cluster ARCH=arm64 instead.")
	}

	// If not in appliance mode, check for cluster connectivity
	// If no cluster is found, warn that only without-cluster tests will run
	if !e2e.ApplianceMode {
		_, _, err := exec.CmdWithContext(context.Background(), exec.Config{}, e2e.ZarfBinPath, "tools", "kubectl", "cluster-info")
		if err != nil {
			cfg := logger.Config{
				Level:       logger.Warn,
				Format:      logger.FormatConsole,
				Destination: logger.DestinationDefault,
				Color:       true,
			}
			l, logErr := logger.New(cfg)
			if logErr != nil {
				log.Fatal(logErr)
			}
			l.Warn("No cluster found - with-cluster tests will fail. " +
				"To run only tests that do not require a cluster use: make test-e2e-without-cluster. " +
				"To run with-cluster tests, ensure a valid cluster is running.")
		}
	}

	os.Exit(m.Run())
}
