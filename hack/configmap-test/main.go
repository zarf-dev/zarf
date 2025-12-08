package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	v1ac "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/aggregator"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/collector"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/event"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
	"sigs.k8s.io/cli-utils/pkg/kstatus/watcher"
	"sigs.k8s.io/cli-utils/pkg/object"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

const (
	testNamespace  = "configmap-test"
	timeout        = 60 * time.Second
	delayBetweenCM = 250 * time.Millisecond
	configMapCount = 30
	dataSizeMB     = 1024 * 1024 // 1MB
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	clientset, cfg, err := connectToCluster()
	if err != nil {
		fmt.Printf("Error connecting to cluster: %v\n", err)
		os.Exit(1)
	}

	// Create watcher
	sw, err := createWatcher(cfg)
	if err != nil {
		fmt.Printf("Error creating watcher: %v\n", err)
		os.Exit(1)
	}

	// Ensure test namespace exists
	fmt.Printf("\nEnsuring namespace '%s' exists...\n", testNamespace)
	if err := ensureNamespace(ctx, clientset, testNamespace); err != nil {
		fmt.Printf("Error creating namespace: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Namespace ready.")

	// Create 30 test ConfigMaps
	fmt.Printf("\nCreating %d test ConfigMaps with %dMB random data each...\n", configMapCount, dataSizeMB/(1024*1024))
	if err := createTestConfigMaps(ctx, clientset, testNamespace, configMapCount); err != nil {
		fmt.Printf("Error creating test ConfigMaps: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ All test ConfigMaps created successfully!")

	time.Sleep(10 * time.Second)

	// Create registry ConfigMap in kube-public
	fmt.Println("\nCreating registry ConfigMap in kube-public...")
	registryCM, err := createRegistryConfigMap(ctx, clientset)
	if err != nil {
		fmt.Printf("Error creating registry ConfigMap: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Registry ConfigMap created: %s\n", registryCM.Name)

	// Run health check on registry ConfigMap
	fmt.Println("\nRunning health check on registry ConfigMap...")
	objMeta := configMapToObjMetadata(registryCM)
	if err := waitForReady(ctx, sw, []object.ObjMetadata{objMeta}); err != nil {
		fmt.Printf("Error during health check: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Health check passed! ConfigMap is ready.")

	// Success summary
	fmt.Println("\n========================================")
	fmt.Println("Success! All ConfigMaps created and verified.")
	fmt.Printf("- %d test ConfigMaps in namespace: %s\n", configMapCount, testNamespace)
	fmt.Println("- 1 registry ConfigMap in namespace: kube-public")
	fmt.Println("\nConfigMaps have been left in the cluster (no cleanup performed).")
}

// connectToCluster returns a Kubernetes client and rest config
func connectToCluster() (kubernetes.Interface, *rest.Config, error) {
	loader := clientcmd.NewDefaultClientConfigLoadingRules()
	clientCfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loader, nil)
	cfg, err := clientCfg.ClientConfig()
	if err != nil {
		return nil, nil, err
	}
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, nil, err
	}
	return clientset, cfg, nil
}

// createWatcher returns a status watcher for the given Kubernetes configuration
func createWatcher(cfg *rest.Config) (watcher.StatusWatcher, error) {
	dynamicClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	httpClient, err := rest.HTTPClientFor(cfg)
	if err != nil {
		return nil, err
	}
	restMapper, err := apiutil.NewDynamicRESTMapper(cfg, httpClient)
	if err != nil {
		return nil, err
	}
	sw := watcher.NewDefaultStatusWatcher(dynamicClient, restMapper)
	return sw, nil
}

// generateRandomData creates a byte slice of random data
func generateRandomData(size int) ([]byte, error) {
	data := make([]byte, size)
	_, err := rand.Read(data)
	if err != nil {
		return nil, fmt.Errorf("failed to generate random data: %w", err)
	}
	return data, nil
}

// ensureNamespace creates the namespace if it doesn't exist
func ensureNamespace(ctx context.Context, clientset kubernetes.Interface, namespace string) error {
	_, err := clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err == nil {
		return nil // Namespace already exists
	}
	if !kerrors.IsNotFound(err) {
		return err // Actual error occurred
	}

	// Create the namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
	_, err = clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	return err
}

// createTestConfigMaps creates count ConfigMaps with random data
func createTestConfigMaps(ctx context.Context, clientset kubernetes.Interface, namespace string, count int) error {
	for i := 0; i < count; i++ {
		// Generate random data
		data, err := generateRandomData(dataSizeMB)
		if err != nil {
			return fmt.Errorf("error generating random data for ConfigMap %d: %w", i, err)
		}

		// Create ConfigMap
		name := fmt.Sprintf("test-configmap-%02d", i)
		cm := v1ac.ConfigMap(name, namespace).
			WithBinaryData(map[string][]byte{
				"data": data,
			})

		_, err = clientset.CoreV1().ConfigMaps(namespace).Apply(ctx, cm,
			metav1.ApplyOptions{Force: true, FieldManager: "configmap-test"})
		if err != nil {
			return fmt.Errorf("error creating ConfigMap %s: %w", name, err)
		}

		// Print progress
		if (i+1)%5 == 0 || i == count-1 {
			fmt.Printf("[%d/%d] Created %s\n", i+1, count, name)
		}

		// Delay between ConfigMaps
		if i < count-1 {
			time.Sleep(delayBetweenCM)
		}
	}
	return nil
}

// createRegistryConfigMap creates the local-registry-hosting ConfigMap in kube-public
func createRegistryConfigMap(ctx context.Context, clientset kubernetes.Interface) (*corev1.ConfigMap, error) {
	registryData := `host: "127.0.0.1:5001"
help: "https://github.com/zarf-dev/zarf"`

	cm := v1ac.ConfigMap("local-registry-hosting", "kube-public").
		WithData(map[string]string{
			"localRegistryHosting.v1": registryData,
		})

	appliedCM, err := clientset.CoreV1().ConfigMaps("kube-public").Apply(ctx, cm,
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
