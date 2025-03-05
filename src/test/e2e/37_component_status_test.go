// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/types"
)

func TestComponentStatus(t *testing.T) {
	t.Log("E2E: Component Status")
	tmpDir := t.TempDir()
	_, _, err := e2e.Zarf(t, "package", "create", "src/test/packages/37-component-status", "-o", tmpDir, "--confirm")
	require.NoError(t, err)
	packageName := fmt.Sprintf("zarf-package-component-status-%s.tar.zst", e2e.Arch)
	path := filepath.Join(tmpDir, packageName)
	// Stop channel getting the zarf state
	stop := make(chan bool)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		deployingSeen := false
		// The package takes 10 seconds to deploy so give an extra 5 seconds before timing out
		ticker := time.NewTicker(15 * time.Second)
		for {
			select {
			case <-ticker.C:
				t.Error("Timed out waiting for package to deploy")
				return
			case <-stop:
				return
			default:
				deployedPackage := types.DeployedPackage{}
				stdOut, _, err := e2e.Kubectl(t, "get", "secret", "zarf-package-component-status", "-n", "zarf", "-o", "jsonpath={.data.data}")
				if err != nil {
					// Wait for the secret to be created and try again
					time.Sleep(2 * time.Second)
					continue
				}
				decoded, err := base64.StdEncoding.DecodeString(stdOut)
				require.NoError(t, err)
				err = json.Unmarshal(decoded, &deployedPackage)
				require.Len(t, deployedPackage.DeployedComponents, 1)
				status := deployedPackage.DeployedComponents[0].Status
				// We expect to see deploying first and then succeeded
				if !deployingSeen {
					require.Equal(t, types.ComponentStatusDeploying, status)
					deployingSeen = true
				} else {
					if status != types.ComponentStatusDeploying {
						require.Equal(t, types.ComponentStatusSucceeded, status)
						break
					}
				}
				require.NoError(t, err)
				time.Sleep(2 * time.Second)
			}
		}
	}()
	stdOut, stdErr, err := e2e.Zarf(t, "package", "deploy", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	close(stop)
	wg.Wait()
	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", "component-status", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}
