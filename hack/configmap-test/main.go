package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	timeout       = 60 * time.Second
	testNamespace = "test-namespace"
)

// getKubeConfig creates a Kubernetes REST config from kubeconfig
func getKubeConfig() (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	return kubeConfig.ClientConfig()
}

// createClientset creates a Kubernetes clientset
func createClientset(config *rest.Config) (*kubernetes.Clientset, error) {
	return kubernetes.NewForConfig(config)
}

func WatcherForConfig(cfg *rest.Config) (watcher.StatusWatcher, error) {
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

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	fmt.Println("k3s ConfigMap test")

	// Get current directory
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	// Create Kubernetes config
	config, err := getKubeConfig()
	if err != nil {
		return fmt.Errorf("getting kubeconfig: %w", err)
	}

	// Create Kubernetes clientset
	clientset, err := createClientset(config)
	if err != nil {
		return fmt.Errorf("creating clientset: %w", err)
	}

	// Create status watcher
	sw, err := WatcherForConfig(config)
	if err != nil {
		return fmt.Errorf("creating watcher: %w", err)
	}

	// Start injection
	fmt.Println("Creating ConfigMaps")
	b, err := os.ReadFile(filepath.Join(workDir, "zarf-injector"))
	if err != nil {
		return err
	}

	_, err = clientset.CoreV1().Namespaces().Apply(ctx, v1ac.Namespace(testNamespace), metav1.ApplyOptions{Force: true, FieldManager: "configmap-test"})
	if err != nil {
		return err
	}

	cm := v1ac.ConfigMap("rust-binary", testNamespace).
		WithBinaryData(map[string][]byte{
			"large-binary": b,
		})
	_, err = clientset.CoreV1().ConfigMaps(*cm.Namespace).Apply(ctx, cm, metav1.ApplyOptions{Force: true, FieldManager: "configmap-test"})
	if err != nil {
		return err
	}

	// Delete the binary configmap
	fmt.Println("Deleting configmap")
	err = clientset.CoreV1().ConfigMaps(testNamespace).Delete(ctx, "rust-binary", metav1.DeleteOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		return err
	}
	// While this selector doesn't actually select anything, it's still needed to
	listOpts := metav1.ListOptions{
		LabelSelector: "nothing-selector",
	}
	err = clientset.CoreV1().ConfigMaps(testNamespace).DeleteCollection(ctx, metav1.DeleteOptions{}, listOpts)
	if err != nil {
		return err
	}

	// Create registry ConfigMap in kube-public
	fmt.Println("Creating separate ConfigMap...")
	registryCM, err := createRegistryConfigMap(ctx, clientset)
	if err != nil {
		return fmt.Errorf("creating registry ConfigMap: %w", err)
	}

	// Run health check on registry ConfigMap
	fmt.Println("Running health check on registry ConfigMap...")
	fmt.Println("current time is", time.Now())
	objMeta := configMapToObjMetadata(registryCM)
	if err := waitForReady(ctx, sw, []object.ObjMetadata{objMeta}); err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	fmt.Println("Health check passed! ConfigMap is ready.")

	return nil
}

// createRegistryConfigMap creates the local-registry-hosting ConfigMap in kube-public
func createRegistryConfigMap(ctx context.Context, clientset *kubernetes.Clientset) (*corev1.ConfigMap, error) {
	registryData := `host: "127.0.0.1:5001"
help: "https://example.com"`

	// Namespace doesn't need to be kube-public, this happens in any namespace
	namespace := "kube-public"

	cm := v1ac.ConfigMap("local-registry-hosting", namespace).
		WithData(map[string]string{
			"localRegistryHosting.v1": registryData,
		})

	appliedCM, err := clientset.CoreV1().ConfigMaps(namespace).Apply(ctx, cm,
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
