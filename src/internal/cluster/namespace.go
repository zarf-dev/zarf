// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains zarf-specific cluster management functions
package cluster

import (
	"context"

	"github.com/defenseunicorns/zarf/src/pkg/message"
)

// DeleteNamespace deletes the zarf namespace from the connected cluster.
func (c *Cluster) DeleteZarfNamespace() {
	spinner := message.NewProgressSpinner("Deleting the zarf namespace from this cluster")
	defer spinner.Stop()

	c.Kube.DeleteNamespace(context.TODO(), ZarfNamespace)
}
