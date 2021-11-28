package k8s

// Mostly taken from https://github.com/argoproj/gitops-engine/blob/master/agent/main.go

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"
	"time"

	adapter "github.com/bombsimon/logrusr"
	"github.com/go-logr/logr"

	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/engine"
	"github.com/argoproj/gitops-engine/pkg/sync"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/git"
	"github.com/defenseunicorns/zarf/cli/internal/images"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

func (syncSettings *settings) parseManifests(componentImages []string) ([]*unstructured.Unstructured, error) {
	// Collection of parsed K8s resources
	var k8sResources []*unstructured.Unstructured
	// Track the namespaces found in the manifests
	namespaces := make(map[string]bool)
	// The target embedded registry to replace in manifests
	registryEndpoint := config.GetEmbeddedRegistryEndpoint()
	// Embedded registry password
	gitSecret := git.GetOrCreateZarfSecret()

	// Create the embedded registry auth token
	zarfHtPassword, err := utils.GetHtpasswdString(config.ZarfGitUser, gitSecret)
	if err != nil {
		logrus.Debug(err)
		logrus.Fatal("Unable to define `htpasswd` string for the Zarf user")
	}
	zarfDockerAuth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", config.ZarfGitUser, gitSecret)))

	// Only pull in yml and yaml files
	pattern := regexp.MustCompile(`(?mi)\.ya?ml$`)
	manifests := utils.RecursiveFileList(syncSettings.path, pattern)

	// Pre-compute all the replacments for the embedded registry
	type ImageSwap struct {
		find    string
		replace string
	}
	var imageSwap []ImageSwap
	for _, image := range componentImages {
		imageSwap = append(imageSwap, ImageSwap{
			find:    image,
			replace: images.SwapHost(image, registryEndpoint),
		})
	}

	for _, manifest := range manifests {
		logrus.WithField("path", manifest).Info("Processing manifest file")
		// Iterate over each imageswap to see if it exists in the manifest
		for _, swap := range imageSwap {
			utils.ReplaceText(manifest, swap.find, swap.replace)
		}
		utils.ReplaceText(manifest, "###ZARF_REGISTRY###", registryEndpoint)
		utils.ReplaceText(manifest, "###ZARF_SECRET###", gitSecret)
		utils.ReplaceText(manifest, "###ZARF_HTPASSWD###", zarfHtPassword)
		utils.ReplaceText(manifest, "###ZARF_DOCKERAUTH###", zarfDockerAuth)

		// Load the file contents
		data, err := ioutil.ReadFile(manifest)
		if err != nil {
			logrus.Fatal(err)
		}
		// Split the k8s resources
		items, err := kube.SplitYAML(data)
		if err != nil {
			logrus.Fatal(err)
		}
		// Append resources to the list
		k8sResources = append(k8sResources, items...)
	}

	// Iterate first to capture all namespaces
	for _, resource := range k8sResources {
		// Add the namespace to the map if it does not exist
		namespace := resource.GetNamespace()
		if !namespaces[namespace] {
			namespaces[namespace] = true
		}
	}

	// Iterate over each namespace to generate image pull creds
	for namespace := range namespaces {
		generatedSecret := GenerateRegistryPullCreds(namespace)
		// Convert to unstructured to match the expected type
		convertedResource, err := kube.ToUnstructured(generatedSecret)
		if err != nil {
			logrus.WithField("namespace", namespace).Fatal("Unable to generate a registry secret for the namespace")
		}
		// Push the list of K8s resources for gitops engine to manage
		k8sResources = append(k8sResources, convertedResource)
	}

	// Track annotations to help Gitops Engine know what's already managed by it
	for idx := range k8sResources {
		annotations := k8sResources[idx].GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[annotationGCMark] = syncSettings.getGCMark(kube.GetResourceKey(k8sResources[idx]))
		k8sResources[idx].SetAnnotations(annotations)
	}

	return k8sResources, nil
}

func GitopsProcess(path string, namespace string, component config.ZarfComponentAppliance) {

	logContext := logrus.WithField("manifest", path)
	syncSettings := settings{path}
	restConfig := getRestConfig()
	revision := time.Now().Format(time.RFC3339Nano)

	logContext.Info("Caching cluster state data")
	clusterCache := cache.NewClusterCache(restConfig,
		cache.SetPopulateResourceInfoHandler(func(un *unstructured.Unstructured, isRoot bool) (info interface{}, cacheManifest bool) {
			// store gc mark of every resource
			gcMark := un.GetAnnotations()[annotationGCMark]
			info = &resourceInfo{gcMark: un.GetAnnotations()[annotationGCMark]}
			// cache resources that has that mark to improve performance
			cacheManifest = gcMark != ""
			return
		}),
	)

	gitOpsEngine := engine.NewEngine(restConfig, clusterCache)
	ctx, done := context.WithCancel(context.Background())

	_, err := gitOpsEngine.Run()
	if err != nil {
		logContext.Debug(err)
		logContext.Fatal("Error syncing cluster state")
	}

	attempt := 0
	for {
		attempt++

		if attempt > 20 {
			logrus.Warn("Unable to complete manifest deployment")
			break
		}

		logrus.Infof("Sync attempt %d of 20", attempt)

		target, err := syncSettings.parseManifests(component.Images)
		if err != nil {
			logrus.Debug(err)
			logrus.Error(err, "Failed to parse target state")
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
			sync.WithLogr(newLogrusLogger(logContext)),
			sync.WithPrune(true),
			sync.WithManifestValidation(true),
			sync.WithNamespaceCreation(true, func(un *unstructured.Unstructured) bool {
				return true
			}),
		)

		if err != nil {
			logrus.Debug(err)
			logrus.Error(err, "Failed to synchronize cluster state")
			time.Sleep(3 * time.Second)
			continue
		}

		for _, res := range result {
			logrus.WithField("result", res.Message).Info(res.ResourceKey.String())
		}

		break
	}

	logContext.Info("Sync operations complete")
	done()

}

// https://github.com/argoproj/argo-cd/blob/a21b0363e39e93982d280722a0eb86c449145766/util/log/logrus.go#L20
func newLogrusLogger(fieldLogger logrus.FieldLogger) logr.Logger {
	return adapter.NewLoggerWithFormatter(fieldLogger, func(val interface{}) string {
		return ""
	})
}
