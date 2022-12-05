// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains zarf-specific cluster management functions
package cluster

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/k8s"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	corev1 "k8s.io/api/core/v1"
)

// Wait for the target pod(s) to come up and inject the data into them
// todo:  this currently requires kubectl but we should have enough k8s work to make this native now
func (c *Cluster) HandleDataInjection(wg *sync.WaitGroup, data types.ZarfDataInjection, componentPath types.ComponentPaths) {
	message.Debugf("packager.handleDataInjections(%#v, %#v, %#v)", wg, data, componentPath)
	defer wg.Done()

	injectionCompletionMarker := filepath.Join(componentPath.DataInjections, config.GetDataInjectionMarker())
	if err := utils.WriteFile(injectionCompletionMarker, []byte("🦄")); err != nil {
		message.Errorf(err, "Unable to create the data injection completion marker")
		return
	}

	tarCompressFlag := ""
	if data.Compress {
		tarCompressFlag = "z"
	}

	// Pod filter to ensure we only use the current deployment's pods
	markerFilter := func(pod corev1.Pod) bool {
		// Loop over all init containers
		for _, container := range pod.Spec.InitContainers {
			// If the container name matches the injection target
			if container.Name == data.Target.Container {
				// Loop over all the containers command arguments
				for _, cmd := range container.Command {
					// If the command argument contains the injection marker, return true
					if strings.Contains(cmd, config.GetDataInjectionMarker()) {
						return true
					}
				}
			}
		}

		return false
	}

iterator:
	// The eternal loop because some data injections can take a very long time
	for {
		message.Debugf("Attempting to inject data into %s", data.Target)
		source := filepath.Join(componentPath.DataInjections, filepath.Base(data.Target.Path))

		target := k8s.PodLookup{
			Namespace: data.Target.Namespace,
			Selector:  data.Target.Selector,
			Container: data.Target.Container,
		}

		// Wait until the pod we are injecting data into becomes available
		pods := c.Kube.WaitForPodsAndContainers(target, markerFilter)
		if len(pods) < 1 {
			continue
		}

		// Inject into all the pods
		for _, pod := range pods {
			kubectlExec := fmt.Sprintf("kubectl exec -i -n %s %s -c %s ", data.Target.Namespace, pod, data.Target.Container)
			tarExec := fmt.Sprintf("tar c%s", tarCompressFlag)
			untarExec := fmt.Sprintf("tar x%svf - -C %s", tarCompressFlag, data.Target.Path)

			// Must create the target directory before trying to change to it for untar
			mkdirExec := fmt.Sprintf("%s -- mkdir -p %s", kubectlExec, data.Target.Path)
			_, _, err := utils.ExecCommandWithContext(context.TODO(), true, "sh", "-c", mkdirExec)
			if err != nil {
				message.Warnf("Unable to create the data injection target directory %s in pod %s", data.Target.Path, pod)
				break iterator
			}

			cpPodExec := fmt.Sprintf("%s -C %s . | %s -- %s",
				tarExec,
				source,
				kubectlExec,
				untarExec,
			)

			// Do the actual data injection
			_, _, err = utils.ExecCommandWithContext(context.TODO(), true, "sh", "-c", cpPodExec)
			if err != nil {
				message.Warnf("Error copying data into the pod %#v: %#v\n", pod, err)
				break iterator
			} else {
				// Leave a marker in the target container for pods to track the sync action
				cpPodExec := fmt.Sprintf("%s -C %s %s | %s -- %s",
					tarExec,
					componentPath.DataInjections,
					config.GetDataInjectionMarker(),
					kubectlExec,
					untarExec,
				)
				_, _, err = utils.ExecCommandWithContext(context.TODO(), true, "sh", "-c", cpPodExec)
				if err != nil {
					message.Warnf("Error saving the zarf sync completion file after injection into pod %#v\n", pod)
				}
			}
		}

		// Do not look for a specific container after injection in case they are running an init container
		podOnlyTarget := k8s.PodLookup{
			Namespace: data.Target.Namespace,
			Selector:  data.Target.Selector,
		}

		// Block one final time to make sure at least one pod has come up and injected the data
		_ = c.Kube.WaitForPodsAndContainers(podOnlyTarget, nil)

		// Cleanup now to reduce disk pressure
		_ = os.RemoveAll(source)

		// Return to stop the loop
		return
	}
}
