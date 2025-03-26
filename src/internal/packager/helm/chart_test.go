// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helm contains operations for working with helm charts.
package helm

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/internal/packager/template"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/types"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/kubectl/pkg/scheme"
)

func TestChartInstall(t *testing.T) {
	// FIXME
	t.Skip()
	ctx := context.Background()
	timeout := time.Second * 10
	chartPath := filepath.Join("testdata", "template", "simple-chart")
	zarfChart := v1alpha1.ZarfChart{
		Name:      "simple-chart",
		Version:   "1.0.0",
		LocalPath: chartPath,
	}
	tmpdir := t.TempDir()
	err := PackageChart(ctx, zarfChart, tmpdir, tmpdir)
	require.NoError(t, err)
	vc := template.GetZarfVariableConfig(ctx)
	vc.SetVariable("image", "nginx:1.0.0", false, false, v1alpha1.RawVariableType)
	vc.SetVariable("port", "8080", false, false, v1alpha1.RawVariableType)
	chart, values, err := LoadChartData(zarfChart, tmpdir, tmpdir, nil)
	require.NoError(t, err)
	dynamicFake := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
	c := &cluster.Cluster{
		Clientset:     fake.NewClientset(),
		DynamicClient: dynamicFake,
	}

	state := &types.ZarfState{
		GitServer: types.GitServerInfo{
			Address:      "https://git-server.com",
			PushUsername: "push-user",
			PushPassword: "push-password",
			PullPassword: "pull-password",
			PullUsername: "pull-user",
		},
		RegistryInfo: types.RegistryInfo{
			PullUsername: "pull-user",
			PushUsername: "push-user",
			PullPassword: "pull-password",
			PushPassword: "push-password",
			Address:      "127.0.0.1:30001",
			NodePort:     30001,
		},
		Distro: "test",
	}
	helmOpts := InstallUpgradeOpts{
		AdoptExistingResources: false,
		VariableConfig:         vc,
		State:                  state,
		Cluster:                c,
		// Needs testcase
		AirgapMode: true,
		Timeout:    timeout,
		Retries:    3,
	}
	connectStrings, releaseName, err := InstallOrUpgradeChart(ctx, zarfChart, chart, values, helmOpts)
	require.NoError(t, err)
	require.Empty(t, connectStrings)
	fmt.Println(releaseName)
}
