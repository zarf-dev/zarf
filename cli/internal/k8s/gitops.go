package k8s

// Mostly taken from https://github.com/argoproj/gitops-engine/blob/master/agent/main.go

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"io/ioutil"
	"strings"
	"time"

	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/engine"
	"github.com/argoproj/gitops-engine/pkg/sync"
	"github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/message"
	"github.com/go-logr/logr"
	"github.com/go-logr/logr/funcr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
)

const (
	annotationGCMark = "gitops-agent.argoproj.io/gc-mark"
)

type resourceInfo struct {
	gcMark string
}

type settings struct {
	path string
}

func (syncSettings *settings) getGCMark(key kube.ResourceKey) string {
	h := sha256.New()
	_, _ = h.Write([]byte(syncSettings.path))
	_, _ = h.Write([]byte(strings.Join([]string{key.Group, key.Kind, key.Name}, "/")))
	return "sha256." + base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

func (syncSettings *settings) parseManifests(componentImages []string, spinner *message.Spinner) ([]*unstructured.Unstructured, error) {
	message.Debugf("k8s.parseManifests(%v, *message.Spinner)", componentImages)
	spinner.Updatef("Processing kubernetes manifests")

	// Collection of parsed K8s resources
	var k8sResources []*unstructured.Unstructured
	// Track the namespaces found in the manifests
	namespaces := make(map[string]bool)

	manifests := ProcessYamlFilesInPath(syncSettings.path, componentImages)

	for _, manifest := range manifests {
		spinner.Updatef("Processing manifest file %s", manifest)
		// Load the file contents
		data, err := ioutil.ReadFile(manifest)
		if err != nil {
			spinner.Fatalf(err, "Unable to read the manifest file")
		}
		// Split the k8s resources
		items, err := kube.SplitYAML(data)
		if err != nil {
			spinner.Fatalf(err, "Error splitting the yaml file into individual sections")
		}

		// Append resources to the list
		k8sResources = append(k8sResources, items...)
	}

	var filteredK8sResources []*unstructured.Unstructured

	// Iterate first to capture all namespaces
	for _, resource := range k8sResources {
		// Add the namespace to the map if it does not exist
		namespace := resource.GetNamespace()
		if !namespaces[namespace] {
			namespaces[namespace] = true
		}

		// Special treatment for "Kind: List", move it back to regular objects
		// https://github.com/traefik/traefik-helm-chart/pull/508
		if resource.GetKind() == "List" {
			spinner.Debugf("handling kind: list object")
			_ = resource.EachListItem(func(item runtime.Object) error {
				castItem := item.(*unstructured.Unstructured)
				filteredK8sResources = append(filteredK8sResources, castItem)
				return nil
			})
		} else {
			filteredK8sResources = append(filteredK8sResources, resource)
		}
	}

	// Iterate over each namespace to generate image pull creds
	for namespace := range namespaces {
		generatedSecret := GenerateRegistryPullCreds(namespace)
		// Convert to unstructured to match the expected type
		convertedResource, err := kube.ToUnstructured(generatedSecret)
		if err != nil {
			spinner.Fatalf(err, "Unable to generate a registry secret for the namespace %s", namespace)
		}
		// Push the list of K8s resources for gitops engine to manage
		filteredK8sResources = append(filteredK8sResources, convertedResource)
	}

	// Track annotations to help Gitops Engine know what's already managed by it
	for idx := range filteredK8sResources {
		namespace := filteredK8sResources[idx].GetNamespace()
		name := filteredK8sResources[idx].GetName()
		spinner.Updatef("Marking Kubernetes resources %s/%s for deployment", namespace, name)
		annotations := filteredK8sResources[idx].GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[annotationGCMark] = syncSettings.getGCMark(kube.GetResourceKey(filteredK8sResources[idx]))
		filteredK8sResources[idx].SetAnnotations(annotations)
	}

	return filteredK8sResources, nil
}

func GitopsProcess(path string, namespace string, component config.ZarfComponent) {
	message.Debugf("k8s.GitopsProcess(%s, %s, %v)", path, namespace, component)
	spinner := message.NewProgressSpinner("Processing manifests for %s", path)
	defer spinner.Stop()

	klog.SetLogger(GenerateLogShim())
	syncSettings := settings{path}
	restConfig := getRestConfig()
	revision := time.Now().Format(time.RFC3339Nano)

	spinner.Updatef("Loading cluster state")
	clusterCache := cache.NewClusterCache(restConfig,
		cache.SetPopulateResourceInfoHandler(func(un *unstructured.Unstructured, isRoot bool) (info interface{}, cacheManifest bool) {
			// store gc mark of every resource
			gcMark := un.GetAnnotations()[annotationGCMark]
			info = &resourceInfo{gcMark: un.GetAnnotations()[annotationGCMark]}
			// cache resources that have that mark to improve performance
			cacheManifest = gcMark != ""
			return
		}),
	)

	gitOpsEngine := engine.NewEngine(restConfig, clusterCache)
	ctx, done := context.WithCancel(context.Background())

	_, err := gitOpsEngine.Run()
	if err != nil {
		spinner.Fatalf(err, "Error syncing cluster state")
	}

	attempt := 0
	for {
		attempt++

		spinner.Updatef("Attempt %d of 20 to sync", attempt)

		if attempt > 20 {
			spinner.Fatalf(nil, "Unable to complete manifest deployment")
			break
		}

		target, err := syncSettings.parseManifests(component.Images, spinner)
		if err != nil {
			spinner.Errorf(err, "Failed to parse target state")
			time.Sleep(3 * time.Second)
			continue
		}

		result, err := gitOpsEngine.Sync(
			ctx,
			target,
			func(r *cache.Resource) bool {
				return r.Info.(*resourceInfo).gcMark == syncSettings.getGCMark(r.ResourceKey())
			},
			revision,
			namespace,
			sync.WithPrune(true),
			sync.WithNamespaceCreation(true, func(un *unstructured.Unstructured) bool {
				return true
			}),
		)

		if err != nil {
			spinner.Errorf(err, "Failed to synchronize cluster state")
			time.Sleep(3 * time.Second)
			continue
		}

		hasFailure := false
		for _, res := range result {
			message.Debugf("%s: %s", res.ResourceKey.String(), res.Message)
			if res.Status == common.ResultCodeSyncFailed {
				message.Debug(res.Status)
				message.Debug(err)
				hasFailure = true
				break
			}
		}

		if hasFailure {
			spinner.Debugf("Sleeping for 3 seconds")
			time.Sleep(3 * time.Second)
			continue
		} else {
			break
		}

	}

	done()
	spinner.Success()

}

func GenerateLogShim() logr.Logger {
	return funcr.New(func(prefix, args string) {
		message.Debug(args)
	}, funcr.Options{})
}
