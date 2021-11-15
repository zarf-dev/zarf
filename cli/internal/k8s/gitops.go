package k8s

// Mostly taken from https://github.com/argoproj/gitops-engine/blob/master/agent/main.go

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	adapter "github.com/bombsimon/logrusr"
	"github.com/go-logr/logr"

	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/engine"
	"github.com/argoproj/gitops-engine/pkg/sync"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
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

func (syncSettings *settings) parseManifests() ([]*unstructured.Unstructured, error) {
	var res []*unstructured.Unstructured

	manifests := utils.RecursiveFileList(syncSettings.path)

	for _, manifest := range manifests {
		if ext := strings.ToLower(filepath.Ext(manifest)); ext == ".yml" || ext == ".yaml" {
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
			res = append(res, items...)

		}
	}

	for i := range res {
		annotations := res[i].GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[annotationGCMark] = syncSettings.getGCMark(kube.GetResourceKey(res[i]))
		res[i].SetAnnotations(annotations)
	}
	return res, nil
}

func GitopsProcess(path string, revision string, namespace string) {

	logContext := logrus.WithField("manifest", path)
	syncSettings := settings{path}
	restConfig := getRestConfig()

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

		target, err := syncSettings.parseManifests()
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
