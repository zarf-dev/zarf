// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

const plainDeployment = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: plain
spec:
  template:
    spec:
      containers:
        - name: app
          image: ghcr.io/zarf-dev/zarf/agent:v0.68.1
      initContainers:
        - name: init
          image: docker.io/library/alpine:latest
`

const listedDeployment = `apiVersion: v1
kind: DeploymentList
items:
  - apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: in-list
    spec:
      template:
        spec:
          containers:
            - name: app
              image: ghcr.io/zarf-dev/zarf/agent:v0.68.1
          initContainers:
            - name: init
              image: docker.io/library/alpine:latest
`

// scanImages splits and scans a manifest the way FindImages composes those steps, so that a
// document holding a list is measured through the same pipeline as one holding a resource.
func scanImages(t *testing.T, manifest string) (matched, maybe map[string]bool) {
	t.Helper()

	ctx := testutil.TestContext(t)
	objs, err := utils.SplitYAML([]byte(manifest))
	require.NoError(t, err)
	objs = scannableResources(objs)

	matched, maybe = map[string]bool{}, map[string]bool{}
	for _, obj := range objs {
		matched, maybe, err = processUnstructuredImages(ctx, obj, matched, maybe)
		require.NoError(t, err)
	}
	return matched, maybe
}

func TestProcessUnstructuredImagesReadsInsideLists(t *testing.T) {
	t.Parallel()

	// a workload is the same workload whether it is its own document or an item of a list, so the
	// images found in it must not depend on the wrapper it arrived in
	plainMatched, plainMaybe := scanImages(t, plainDeployment)
	listMatched, listMaybe := scanImages(t, listedDeployment)

	// the plain document is read as the workload it is, so both of its images are confirmed
	require.Equal(t, map[string]bool{
		"ghcr.io/zarf-dev/zarf/agent:v0.68.1": true,
		"docker.io/library/alpine:latest":     true,
	}, plainMatched)

	require.Equal(t, plainMatched, listMatched, "images inside a list must be confirmed matches, not guesses")
	require.Equal(t, plainMaybe, listMaybe)
}

func TestFindImagesInListKinds(t *testing.T) {
	t.Parallel()

	// the whole command over manifests that hold their resources in lists: the images belong under
	// images:, not under the possible-images bucket a user has to confirm by hand.
	//
	// the two busybox tags are the helm test boundary: :1.36 is annotated on an item inside a plain
	// list, which helm installs, so it belongs here; :1.37 sits in a list that is itself the test,
	// which helm never installs, so it must not
	scans, err := FindImages(testutil.TestContext(t), "./testdata/find-images/list-kinds", FindImagesOptions{SkipCosign: true})
	require.NoError(t, err)

	require.Equal(t, []ComponentImageScan{
		{
			ComponentName: "baseline",
			Matches: []string{
				"docker.io/library/alpine:latest",
				"docker.io/library/busybox:1.36",
				"ghcr.io/zarf-dev/flux-chart:1.2.3",
				"ghcr.io/zarf-dev/zarf/agent:v0.68.1",
			},
		},
	}, scans)
}

const plainOCIRepo = `apiVersion: source.toolkit.fluxcd.io/v1beta2
kind: OCIRepository
metadata:
  name: plain
spec:
  url: oci://ghcr.io/zarf-dev/flux-chart
  ref:
    tag: 1.2.3
`

const listedOCIRepo = `apiVersion: v1
kind: OCIRepositoryList
items:
  - apiVersion: source.toolkit.fluxcd.io/v1beta2
    kind: OCIRepository
    metadata:
      name: in-list
    spec:
      url: oci://ghcr.io/zarf-dev/flux-chart
      ref:
        tag: 1.2.3
`

func TestProcessUnstructuredImagesReadsOCIRepositoriesInsideLists(t *testing.T) {
	t.Parallel()

	// an OCIRepository keeps its reference in spec.url and spec.ref.tag, which only the typed case
	// assembles. no regex looks at url, so a wrapped one is not even guessed at
	plainMatched, _ := scanImages(t, plainOCIRepo)
	listMatched, _ := scanImages(t, listedOCIRepo)

	require.Equal(t, map[string]bool{"ghcr.io/zarf-dev/flux-chart:1.2.3": true}, plainMatched)
	require.Equal(t, plainMatched, listMatched)
}

func TestScannableResourcesUnwrapsNestedLists(t *testing.T) {
	t.Parallel()

	// a list is free to hold another list, and helm's own flatten keeps going, so this has to as
	// well or the inner wrapper reaches the scan as the one resource it appears to be
	const nested = `apiVersion: v1
kind: List
items:
  - apiVersion: v1
    kind: DeploymentList
    items:
      - apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: deep
        spec:
          template:
            spec:
              containers:
                - name: app
                  image: docker.io/library/alpine:latest
`

	matched, _ := scanImages(t, nested)
	require.Equal(t, map[string]bool{"docker.io/library/alpine:latest": true}, matched)
}

func TestScannableResourcesKeepsDocumentsThatAreNotListsOfResources(t *testing.T) {
	t.Parallel()

	// a custom resource may keep anything in an items array, so scanning it as one document keeps
	// the answer the regex fallback already gave instead of failing the whole scan
	const widget = `apiVersion: example.com/v1
kind: Widget
metadata:
  name: has-items
spec:
  image: docker.io/library/alpine:latest
items:
  - alpha
  - beta
`

	_, maybe := scanImages(t, widget)
	require.Equal(t, map[string]bool{"docker.io/library/alpine:latest": true}, maybe)
}

func TestScannableResourcesHelmTestBoundary(t *testing.T) {
	t.Parallel()

	const markedList = `apiVersion: v1
kind: List
metadata:
  annotations:
    helm.sh/hook: test
items:
  - apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: inner
    spec:
      template:
        spec:
          containers:
            - name: t
              image: docker.io/library/busybox:1.36
`

	const markedItem = `apiVersion: v1
kind: List
items:
  - apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: inner
      annotations:
        helm.sh/hook: test
    spec:
      template:
        spec:
          containers:
            - name: t
              image: docker.io/library/busybox:1.36
`

	// helm reads the hook annotation off each document it renders, so where the annotation sits
	// decides what is installed, and with it what has to be found. the fuzzy pass runs for every
	// kind, so a confirmed image is a possible one too, and empty maybes mean nothing was scanned
	busybox := map[string]bool{"docker.io/library/busybox:1.36": true}
	for _, tc := range []struct {
		name            string
		manifest        string
		expectedMatched map[string]bool
		expectedMaybe   map[string]bool
	}{
		{name: "list is the test", manifest: markedList, expectedMatched: map[string]bool{}, expectedMaybe: map[string]bool{}},
		{name: "item in a plain list is annotated", manifest: markedItem, expectedMatched: busybox, expectedMaybe: busybox},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			matched, maybe := scanImages(t, tc.manifest)
			require.Equal(t, tc.expectedMatched, matched)
			require.Equal(t, tc.expectedMaybe, maybe)
		})
	}
}
