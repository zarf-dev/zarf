// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package helm

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/kube/fake"
	"helm.sh/helm/v4/pkg/release"
	releasecommon "helm.sh/helm/v4/pkg/release/common"
	releasev1 "helm.sh/helm/v4/pkg/release/v1"
	"helm.sh/helm/v4/pkg/storage"
	"helm.sh/helm/v4/pkg/storage/driver"
)

func TestHelmGetReturnsLatestRevision(t *testing.T) {
	t.Parallel()

	actionConfig := testActionConfig(t, 2, 1, 3)
	latest, err := action.NewGet(actionConfig).Run("agent")
	require.NoError(t, err)

	latestAccessor, err := release.NewAccessor(latest)
	require.NoError(t, err)
	require.Equal(t, 3, latestAccessor.Version())
}

func testActionConfig(t *testing.T, revisions ...int) *action.Configuration {
	t.Helper()

	actionConfig := action.NewConfiguration()
	actionConfig.KubeClient = &fake.PrintingKubeClient{Out: io.Discard, LogOutput: io.Discard}
	actionConfig.Releases = storage.Init(driver.NewMemory())
	for _, revision := range revisions {
		err := actionConfig.Releases.Create(&releasev1.Release{
			Name:      "agent",
			Namespace: "zarf",
			Version:   revision,
			Info:      &releasev1.Info{Status: releasecommon.StatusDeployed},
		})
		require.NoError(t, err)
	}
	return actionConfig
}
