// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var crioBootstrapMinimumVersion = semver.MustParse("1.35.0")

func logBootstrapRuntimeDiagnostics(ctx context.Context, c *cluster.Cluster, injectorAddress string) {
	l := logger.From(ctx)
	nodes, err := c.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		l.Debug("unable to inspect node container runtimes for CRI-O bootstrap compatibility", "error", err)
		return
	}

	sort.Slice(nodes.Items, func(i, j int) bool {
		return nodes.Items[i].Name < nodes.Items[j].Name
	})

	logDetectedContainerRuntimes(ctx, nodes.Items)
	warnCRIOBootstrapCompatibility(ctx, nodes.Items, injectorAddress)
}

func logDetectedContainerRuntimes(ctx context.Context, nodes []corev1.Node) {
	l := logger.From(ctx)
	if !l.Enabled(ctx, slog.LevelDebug) {
		return
	}

	runtimes := make([]string, 0, len(nodes))
	for _, node := range nodes {
		runtimeVersion := node.Status.NodeInfo.ContainerRuntimeVersion
		if runtimeVersion != "" {
			runtimes = append(runtimes, fmt.Sprintf("%s (%s)", node.Name, runtimeVersion))
		}
	}
	l.Debug("detected node container runtimes", "runtimes", strings.Join(runtimes, ", "))
}

func warnCRIOBootstrapCompatibility(ctx context.Context, nodes []corev1.Node, injectorAddress string) {
	matching := make([]string, 0)
	for _, node := range nodes {
		runtimeVersion := node.Status.NodeInfo.ContainerRuntimeVersion
		versionString, isCRIO := strings.CutPrefix(runtimeVersion, "cri-o://")
		if !isCRIO {
			continue
		}
		version, err := semver.NewVersion(versionString)
		if err == nil && !version.LessThan(crioBootstrapMinimumVersion) {
			matching = append(matching, fmt.Sprintf("%s (%s)", node.Name, runtimeVersion))
		}
	}
	if len(matching) == 0 {
		return
	}

	logger.From(ctx).Warn(fmt.Sprintf(
		"CRI-O 1.35+ detected on nodes %s. Zarf's temporary bootstrap injector at %s serves HTTP, while affected CRI-O registry policies may attempt HTTPS. If bootstrap image pulls fail, configure this endpoint in the node runtime's registries.conf or use an external registry.",
		strings.Join(matching, ", "), injectorAddress,
	))
}
