package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	v1ac "k8s.io/client-go/applyconfigurations/core/v1"

	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/aggregator"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/collector"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/event"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
	"sigs.k8s.io/cli-utils/pkg/kstatus/watcher"
	"sigs.k8s.io/cli-utils/pkg/object"

	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
)

const (
	timeout     = 60 * time.Second
	packageFile = "zarf-init-amd64-v0.67.0-11-g0b411ed2.tar.zst"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	fmt.Println("ConfigMap Test Tool with Zarf Injection")
	fmt.Println("=========================================")

	// Get current directory
	workDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting working directory: %v\n", err)
		os.Exit(1)
	}

	packagePath := filepath.Join(workDir, packageFile)

	// Extract package
	fmt.Println("✓ Package extracted")
	pkgLayout, err := layout.LoadFromTar(ctx, packagePath, layout.PackageLayoutOptions{})
	if err != nil {
		fmt.Printf("Error loading package: %v\n", err)
		os.Exit(1)
	}

	// Connect to cluster
	fmt.Println("\nConnecting to Kubernetes cluster...")
	c, err := cluster.NewWithWait(ctx)
	if err != nil {
		fmt.Printf("Error connecting to cluster: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Connected successfully!")

	pkgName := "configmap-test"
	// Start injection
	fmt.Println("\nStarting Zarf injection...")
	cwd, _ := os.Getwd()
	_, _, err = c.CreateInjectorConfigMaps(ctx, cwd, pkgLayout.GetImageDirPath(), []string{"library/registry:3.0.0", "alpine/socat:1.8.0.3"}, pkgName)
	if err != nil {
		fmt.Printf("Error starting injection: %v\n", err)
		os.Exit(1)
	}

	// Stop injection (this cleans up the injector resources)
	fmt.Println("\nStopping Zarf injection...")
	if err := c.StopInjection(ctx); err != nil {
		fmt.Printf("Error stopping injection: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Injection stopped and cleaned up")

	// Create registry ConfigMap in kube-public
	fmt.Println("\nCreating registry ConfigMap in kube-public...")
	registryCM, err := createRegistryConfigMap(ctx, c)
	if err != nil {
		fmt.Printf("Error creating registry ConfigMap: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Registry ConfigMap created: %s\n", registryCM.Name)

	// Run health check on registry ConfigMap
	fmt.Println("\nRunning health check on registry ConfigMap...")
	objMeta := configMapToObjMetadata(registryCM)
	if err := waitForReady(ctx, c.Watcher, []object.ObjMetadata{objMeta}); err != nil {
		fmt.Printf("Error during health check: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Health check passed! ConfigMap is ready.")

	// Success summary
	fmt.Println("\n=========================================")
	fmt.Println("Success! Zarf injection completed and ConfigMap verified.")
	fmt.Println("- Zarf injector resources created and cleaned up")
	fmt.Println("- 1 registry ConfigMap in namespace: kube-public (still in cluster)")
}

// createRegistryConfigMap creates the local-registry-hosting ConfigMap in kube-public
func createRegistryConfigMap(ctx context.Context, c *cluster.Cluster) (*corev1.ConfigMap, error) {
	registryData := `host: "127.0.0.1:5001"
help: "https://github.com/zarf-dev/zarf"`

	cm := v1ac.ConfigMap("local-registry-hosting", "kube-public").
		WithData(map[string]string{
			"localRegistryHosting.v1": registryData,
		})

	appliedCM, err := c.Clientset.CoreV1().ConfigMaps("kube-public").Apply(ctx, cm,
		metav1.ApplyOptions{Force: true, FieldManager: "configmap-test"})
	if err != nil {
		return nil, err
	}

	return appliedCM, nil
}

// configMapToObjMetadata converts a ConfigMap to ObjMetadata for health checking
func configMapToObjMetadata(cm *corev1.ConfigMap) object.ObjMetadata {
	return object.ObjMetadata{
		Name:      cm.Name,
		Namespace: cm.Namespace,
		GroupKind: schema.GroupKind{
			Group: "",
			Kind:  "ConfigMap",
		},
	}
}

// waitForReady waits for the given objects to reach ready status
// Copied from /home/austin/code/zarf/src/internal/healthchecks/healthchecks.go:62-104
func waitForReady(ctx context.Context, sw watcher.StatusWatcher, objs []object.ObjMetadata) error {
	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	eventCh := sw.Watch(cancelCtx, objs, watcher.Options{})
	statusCollector := collector.NewResourceStatusCollector(objs)
	done := statusCollector.ListenWithObserver(eventCh, collector.ObserverFunc(
		func(statusCollector *collector.ResourceStatusCollector, _ event.Event) {
			rss := []*event.ResourceStatus{}
			for _, rs := range statusCollector.ResourceStatuses {
				if rs == nil {
					continue
				}
				rss = append(rss, rs)
			}
			desired := status.CurrentStatus
			if aggregator.AggregateStatus(rss, desired) == desired {
				cancel()
				return
			}
		}),
	)
	<-done

	if statusCollector.Error != nil {
		return statusCollector.Error
	}

	// Only check parent context error, otherwise we would error when desired status is achieved.
	if ctx.Err() != nil {
		errs := []error{}
		for _, id := range objs {
			rs := statusCollector.ResourceStatuses[id]
			if rs.Status != status.CurrentStatus {
				errs = append(errs, fmt.Errorf("%s: %s not ready, status is %s: additional info: %s", rs.Identifier.Name, rs.Identifier.GroupKind.Kind, rs.Status, rs.String()))
			}
		}
		errs = append(errs, ctx.Err())
		return errors.Join(errs...)
	}

	return nil
}
