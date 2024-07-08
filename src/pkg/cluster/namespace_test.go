// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cluster

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdoptZarfManagedLabels(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"foo":      "bar",
		AgentLabel: "ignore",
	}
	adoptedLabels := AdoptZarfManagedLabels(labels)
	expectedLabels := map[string]string{
		"foo":              "bar",
		ZarfManagedByLabel: "zarf",
	}
	require.Equal(t, expectedLabels, adoptedLabels)
}
