// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cluster

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/internal/healthchecks"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery/cached/memory"
	fakediscovery "k8s.io/client-go/discovery/fake"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clientfeatures "k8s.io/client-go/features"
	clientfeaturestesting "k8s.io/client-go/features/testing"
	"k8s.io/client-go/restmapper"
	k8stesting "k8s.io/client-go/testing"
	"sigs.k8s.io/cli-utils/pkg/kstatus/watcher"
	"sigs.k8s.io/cli-utils/pkg/object"
)

// slowDiscovery adds latency to discovery calls so concurrent informers overlap.
type slowDiscovery struct {
	*fakediscovery.FakeDiscovery
}

func (s *slowDiscovery) ServerGroups() (*metav1.APIGroupList, error) {
	time.Sleep(20 * time.Millisecond)
	return s.FakeDiscovery.ServerGroups()
}

func (s *slowDiscovery) ServerGroupsWithContext(ctx context.Context) (*metav1.APIGroupList, error) {
	time.Sleep(20 * time.Millisecond)
	return s.FakeDiscovery.ServerGroupsWithContext(ctx)
}

// TestInvalidatingWatcherNewCRDKinds watches one object to build the mapper, registers
// several new kinds, then watches one object of each new kind through the same watcher.
// Every new kind must resolve; a stale mapper delegate leaves some kinds Unknown.
func TestInvalidatingWatcherNewCRDKinds(t *testing.T) {
	// The fake dynamic client does not support streaming list.
	clientfeaturestesting.SetFeatureDuringTest(t, clientfeatures.WatchListClient, false)
	const kinds = 25
	gv := func(i int) schema.GroupVersion {
		return schema.GroupVersion{Group: fmt.Sprintf("kind%d.example.com", i), Version: "v1"}
	}
	for run := range 10 {
		fake := &k8stesting.Fake{Resources: []*metav1.APIResourceList{{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{{Name: "configmaps", Kind: "ConfigMap", Namespaced: true, Verbs: []string{"get", "list", "watch"}}},
		}}}
		listKinds := map[schema.GroupVersionResource]string{{Version: "v1", Resource: "configmaps"}: "ConfigMapList"}
		cm := &unstructured.Unstructured{}
		cm.SetAPIVersion("v1")
		cm.SetKind("ConfigMap")
		cm.SetNamespace("default")
		cm.SetName("first")
		objs := []runtime.Object{cm}
		targets := []object.ObjMetadata{}
		for i := range kinds {
			listKinds[gv(i).WithResource(fmt.Sprintf("thing%ds", i))] = fmt.Sprintf("Thing%dList", i)
			u := &unstructured.Unstructured{}
			u.SetAPIVersion(gv(i).String())
			u.SetKind(fmt.Sprintf("Thing%d", i))
			u.SetName("one")
			objs = append(objs, u)
			targets = append(targets, object.ObjMetadata{GroupKind: schema.GroupKind{Group: gv(i).Group, Kind: fmt.Sprintf("Thing%d", i)}, Name: "one"})
		}
		dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), listKinds, objs...)
		mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(&slowDiscovery{&fakediscovery.FakeDiscovery{Fake: fake}}))
		w := &invalidatingWatcher{StatusWatcher: watcher.NewDefaultStatusWatcher(dyn, mapper), restMapper: mapper}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := healthchecks.WaitForReady(ctx, w, []object.ObjMetadata{{GroupKind: schema.GroupKind{Kind: "ConfigMap"}, Namespace: "default", Name: "first"}})
		cancel()
		require.NoError(t, err)

		for i := range kinds {
			fake.Resources = append(fake.Resources, &metav1.APIResourceList{
				GroupVersion: gv(i).String(),
				APIResources: []metav1.APIResource{{Name: fmt.Sprintf("thing%ds", i), Kind: fmt.Sprintf("Thing%d", i), Verbs: []string{"get", "list", "watch"}}},
			})
		}
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		err = healthchecks.WaitForReady(ctx, w, targets)
		cancel()
		require.NoError(t, err, "run %d", run)
	}
}
