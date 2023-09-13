// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/k8s"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/exec"
	"github.com/defenseunicorns/zarf/src/types"
	corev1 "k8s.io/api/core/v1"
)

// HandleDataInjection waits for the target pod(s) to come up and inject the data into them
// todo:  this currently requires kubectl but we should have enough k8s work to make this native now.
func (c *Cluster) HandleDataInjection(wg *sync.WaitGroup, data types.ZarfDataInjection, componentPath types.ComponentPaths, dataIdx int) {
	defer wg.Done()

	injectionCompletionMarker := filepath.Join(componentPath.DataInjections, config.GetDataInjectionMarker())
	if err := utils.WriteFile(injectionCompletionMarker, []byte("🦄")); err != nil {
		message.WarnErrf(err, "Unable to create the data injection completion marker")
		return
	}

	tarCompressFlag := ""
	if data.Compress {
		tarCompressFlag = "-z"
	}

	// Pod filter to ensure we only use the current deployment's pods
	podFilterByInitContainer := func(pod corev1.Pod) bool {
		// Look everywhere in the pod for a matching data injection marker
		return strings.Contains(message.JSONValue(pod), config.GetDataInjectionMarker())
	}

	// Get the OS shell to execute commands in
	shell, shellArgs := exec.GetOSShell(types.ZarfComponentActionShell{Windows: "cmd"})

	if _, _, err := exec.Cmd(shell, shellArgs, "tar --version"); err != nil {
		message.WarnErr(err, "Unable to execute tar on this system.  Please ensure it is installed and on your $PATH.")
		return
	}

iterator:
	// The eternal loop because some data injections can take a very long time
	for {
		message.Debugf("Attempting to inject data into %s", data.Target)
		source := filepath.Join(componentPath.DataInjections, filepath.Base(data.Target.Path))
		if utils.InvalidPath(source) {
			// The path is likely invalid because of how we compose OCI components, add an index suffix to the filename
			source = filepath.Join(componentPath.DataInjections, strconv.Itoa(dataIdx), filepath.Base(data.Target.Path))
			if utils.InvalidPath(source) {
				message.Warnf("Unable to find the data injection source path %s", source)
				return
			}
		}

		target := k8s.PodLookup{
			Namespace: data.Target.Namespace,
			Selector:  data.Target.Selector,
			Container: data.Target.Container,
		}

		// Wait until the pod we are injecting data into becomes available
		pods := c.WaitForPodsAndContainers(target, podFilterByInitContainer)
		if len(pods) < 1 {
			continue
		}

		// Inject into all the pods
		for _, pod := range pods {
			// Try to use the embedded kubectl if we can
			zarfBinPath, err := utils.GetFinalExecutablePath()
			kubectlBinPath := "kubectl"
			if err != nil {
				message.Warnf("Unable to get the zarf executable path, falling back to host kubectl: %s", err)
			} else {
				kubectlBinPath = fmt.Sprintf("%s tools kubectl", zarfBinPath)
			}
			kubectlCmd := fmt.Sprintf("%s exec -i -n %s %s -c %s ", kubectlBinPath, data.Target.Namespace, pod.Name, data.Target.Container)

			// Note that each command flag is separated to provide the widest cross-platform tar support
			tarCmd := fmt.Sprintf("tar -c %s -f -", tarCompressFlag)
			untarCmd := fmt.Sprintf("tar -x %s -v -f - -C %s", tarCompressFlag, data.Target.Path)

			// Must create the target directory before trying to change to it for untar
			mkdirCmd := fmt.Sprintf("%s -- mkdir -p %s", kubectlCmd, data.Target.Path)
			if err := exec.CmdWithPrint(shell, shellArgs, mkdirCmd); err != nil {
				message.Warnf("Unable to create the data injection target directory %s in pod %s", data.Target.Path, pod.Name)
				continue iterator
			}

			cpPodCmd := fmt.Sprintf("%s -C %s . | %s -- %s",
				tarCmd,
				source,
				kubectlCmd,
				untarCmd,
			)

			// Do the actual data injection
			if err := exec.CmdWithPrint(shell, shellArgs, cpPodCmd); err != nil {
				message.Warnf("Error copying data into the pod %#v: %#v\n", pod.Name, err)
				continue iterator
			}

			// Leave a marker in the target container for pods to track the sync action
			cpPodCmd = fmt.Sprintf("%s -C %s %s | %s -- %s",
				tarCmd,
				componentPath.DataInjections,
				config.GetDataInjectionMarker(),
				kubectlCmd,
				untarCmd,
			)

			if err := exec.CmdWithPrint(shell, shellArgs, cpPodCmd); err != nil {
				message.Warnf("Error saving the zarf sync completion file after injection into pod %#v\n", pod.Name)
				continue iterator
			}
		}

		// Do not look for a specific container after injection in case they are running an init container
		podOnlyTarget := k8s.PodLookup{
			Namespace: data.Target.Namespace,
			Selector:  data.Target.Selector,
		}

		// Block one final time to make sure at least one pod has come up and injected the data
		// Using only the pod as the final selector because we don't know what the container name will be
		// Still using the init container filter to make sure we have the right running pod
		_ = c.WaitForPodsAndContainers(podOnlyTarget, podFilterByInitContainer)

		// Cleanup now to reduce disk pressure
		_ = os.RemoveAll(source)

		// Return to stop the loop
		return
	}
}
