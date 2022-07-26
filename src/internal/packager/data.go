package packager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/defenseunicorns/zarf/src/internal/k8s"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

// Wait for the target pod(s) to come up and inject the data into them
// todo:  this currently requires kubectl but we should have enough k8s work to make this native now
func handleDataInjection(wg *sync.WaitGroup, data types.ZarfDataInjection, componentPath componentPaths) {
	message.Debugf("packager.handleDataInjections(%#v, %#v, %#v)", wg, data, componentPath)
	defer wg.Done()

	injectionCompletionMarker := filepath.Join(componentPath.dataInjections, ".zarf-sync-complete")
	if err := utils.WriteFile(injectionCompletionMarker, []byte("🦄")); err != nil {
		message.Errorf(err, "Unable to create the data injection completion marker")
		return
	}

	timeout := time.After(15 * time.Minute)
	for {
		// delay check 2 seconds
		time.Sleep(2 * time.Second)

		message.Debugf("Attempting to inject data into %s", data.Target)

		select {

		// on timeout abort
		case <-timeout:
			message.Warnf("data injection into target %s timed out\n", data.Target.Namespace)
			return

		default:
			source := filepath.Join(componentPath.dataInjections, filepath.Base(data.Target.Path))

			// Wait until the pod we are injecting data into becomes available
			pods := k8s.WaitForPodsAndContainers(data.Target, true)
			if len(pods) < 1 {
				continue
			}

			// Inject into all the pods
			for _, pod := range pods {
				cpPodExec := fmt.Sprintf("tar c -C %s . | kubectl exec -i -n %s %s -c %s -- tar xvf - -C %s",
					source,
					data.Target.Namespace,
					pod,
					data.Target.Container,
					data.Target.Path,
				)

				// Do the actual data injection
				_, _, err := utils.ExecCommandWithContext(context.TODO(), true, "sh", "-c", cpPodExec)
				if err != nil {
					message.Warnf("Error copying data into the pod %#v: %#v\n", pod, err)
					continue
				} else {
					// Leave a marker in the target container for pods to track the sync action
					cpPodExec := fmt.Sprintf("tar c -C %s .zarf-sync-complete | kubectl exec -i -n %s %s -c %s -- tar xvf - -C %s",
						componentPath.dataInjections,
						data.Target.Namespace,
						pod,
						data.Target.Container,
						data.Target.Path,
					)
					_, _, err = utils.ExecCommandWithContext(context.TODO(), true, "sh", "-c", cpPodExec)
					if err != nil {
						message.Warnf("Error saving the zarf sync completion file after injection into pod %#v\n", pod)
					}
				}
			}

			// Cleanup now to reduce disk pressure
			_ = os.RemoveAll(source)

			// Return to stop the loop
			return
		}
	}
}
