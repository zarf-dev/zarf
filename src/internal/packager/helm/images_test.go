// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helm contains operations for working with helm charts.
package helm

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"helm.sh/helm/v4/pkg/chart/common"
)

func TestFindAnnotatedImagesForChart(t *testing.T) {
	t.Run("basic chart with annotations", func(t *testing.T) {
		testChartPath := filepath.Join("testdata", "annotations-test", "test-chart")

		// Test with default values (redis.enabled=true, subchart.postgres.enabled=false)
		values := common.Values{
			"redis": map[string]interface{}{
				"enabled": true,
			},
			"subchart": map[string]interface{}{
				"postgres": map[string]interface{}{
					"enabled": false,
				},
			},
		}

		images, err := FindAnnotatedImagesForChart(testChartPath, values)
		require.NoError(t, err)

		// Convert to map for easier testing
		imageMap := make(map[string]bool)
		for _, img := range images {
			imageMap[img] = true
		}

		// Images from parent chart without conditions
		require.True(t, imageMap["docker.io/library/nginx:1.25.0"], "nginx image should be found")
		require.True(t, imageMap["docker.io/library/busybox:1.36"], "busybox image should be found")

		// Image from parent chart with condition (redis.enabled=true)
		require.True(t, imageMap["docker.io/library/redis:7.0"], "redis image should be found when condition is true")

		// Images from subchart
		require.True(t, imageMap["docker.io/library/memcached:1.6"], "memcached image from subchart should be found")

		// Image from subchart with condition (subchart.postgres.enabled=false)
		require.False(t, imageMap["docker.io/library/postgres:15"], "postgres image should NOT be found when condition is false")

		// Verify we got exactly 4 images
		require.Len(t, images, 4, "should find exactly 4 images")
	})

	t.Run("chart with all conditions enabled", func(t *testing.T) {
		testChartPath := filepath.Join("testdata", "annotations-test", "test-chart")

		// Enable all conditional images
		values := common.Values{
			"redis": map[string]interface{}{
				"enabled": true,
			},
			"subchart": map[string]interface{}{
				"postgres": map[string]interface{}{
					"enabled": true,
				},
			},
		}

		images, err := FindAnnotatedImagesForChart(testChartPath, values)
		require.NoError(t, err)

		imageMap := make(map[string]bool)
		for _, img := range images {
			imageMap[img] = true
		}

		// All images should be present
		require.True(t, imageMap["docker.io/library/nginx:1.25.0"], "nginx should be found")
		require.True(t, imageMap["docker.io/library/busybox:1.36"], "busybox should be found")
		require.True(t, imageMap["docker.io/library/redis:7.0"], "redis should be found")
		require.True(t, imageMap["docker.io/library/memcached:1.6"], "memcached should be found")
		require.True(t, imageMap["docker.io/library/postgres:15"], "postgres should be found when condition is true")

		require.Len(t, images, 5, "should find all 5 images")
	})

	t.Run("chart with all conditions disabled", func(t *testing.T) {
		testChartPath := filepath.Join("testdata", "annotations-test", "test-chart")

		// Disable all conditional images
		values := common.Values{
			"redis": map[string]interface{}{
				"enabled": false,
			},
			"subchart": map[string]interface{}{
				"postgres": map[string]interface{}{
					"enabled": false,
				},
			},
		}

		images, err := FindAnnotatedImagesForChart(testChartPath, values)
		require.NoError(t, err)

		imageMap := make(map[string]bool)
		for _, img := range images {
			imageMap[img] = true
		}

		// Only unconditional images should be present
		require.True(t, imageMap["docker.io/library/nginx:1.25.0"], "nginx should be found")
		require.True(t, imageMap["docker.io/library/busybox:1.36"], "busybox should be found")
		require.True(t, imageMap["docker.io/library/memcached:1.6"], "memcached should be found")

		// Conditional images should not be present
		require.False(t, imageMap["docker.io/library/redis:7.0"], "redis should NOT be found when disabled")
		require.False(t, imageMap["docker.io/library/postgres:15"], "postgres should NOT be found when disabled")

		require.Len(t, images, 3, "should find only 3 unconditional images")
	})

	t.Run("chart with empty values", func(t *testing.T) {
		testChartPath := filepath.Join("testdata", "annotations-test", "test-chart")

		// Use empty values - should merge with chart's default values
		values := common.Values{}

		images, err := FindAnnotatedImagesForChart(testChartPath, values)
		require.NoError(t, err)

		imageMap := make(map[string]bool)
		for _, img := range images {
			imageMap[img] = true
		}

		// With chart's default values from parent chart's values.yaml:
		// redis.enabled=true, subchart.postgres.enabled=false
		require.True(t, imageMap["docker.io/library/nginx:1.25.0"], "nginx should be found")
		require.True(t, imageMap["docker.io/library/busybox:1.36"], "busybox should be found")
		require.True(t, imageMap["docker.io/library/redis:7.0"], "redis should be found with default value")
		require.True(t, imageMap["docker.io/library/memcached:1.6"], "memcached should be found")

		// Parent chart values.yaml sets subchart.postgres.enabled=false, so postgres should NOT be found
		require.False(t, imageMap["docker.io/library/postgres:15"], "postgres should NOT be found with parent chart default values")

		require.Len(t, images, 4, "should find 4 images with chart defaults")
	})

	t.Run("chart with missing subchart values", func(t *testing.T) {
		testChartPath := filepath.Join("testdata", "annotations-test", "test-chart")

		// Provide parent values but no subchart values
		// Since we're not overriding subchart values, it will use the parent chart's
		// default values.yaml which sets subchart.postgres.enabled=false
		values := common.Values{
			"redis": map[string]interface{}{
				"enabled": false,
			},
		}

		images, err := FindAnnotatedImagesForChart(testChartPath, values)
		require.NoError(t, err)

		imageMap := make(map[string]bool)
		for _, img := range images {
			imageMap[img] = true
		}

		// Redis should be disabled
		require.False(t, imageMap["docker.io/library/redis:7.0"], "redis should NOT be found when disabled")

		// Subchart values come from parent chart's values.yaml (postgres.enabled=false)
		require.False(t, imageMap["docker.io/library/postgres:15"], "postgres should NOT be found with parent chart default")

		// Should have nginx, busybox, and memcached
		require.Len(t, images, 3, "should find 3 unconditional images")
	})
}

func TestShouldIncludeImage(t *testing.T) {
	t.Run("image without condition", func(t *testing.T) {
		img := ChartImage{
			Name:      "test",
			Image:     "test:latest",
			Condition: "",
		}
		values := common.Values{}

		result := shouldIncludeImage(img, values)
		require.True(t, result, "image without condition should always be included")
	})

	t.Run("image with true condition", func(t *testing.T) {
		img := ChartImage{
			Name:      "test",
			Image:     "test:latest",
			Condition: "feature.enabled",
		}
		values := common.Values{
			"feature": map[string]interface{}{
				"enabled": true,
			},
		}

		result := shouldIncludeImage(img, values)
		require.True(t, result, "image should be included when condition is true")
	})

	t.Run("image with false condition", func(t *testing.T) {
		img := ChartImage{
			Name:      "test",
			Image:     "test:latest",
			Condition: "feature.enabled",
		}
		values := common.Values{
			"feature": map[string]interface{}{
				"enabled": false,
			},
		}

		result := shouldIncludeImage(img, values)
		require.False(t, result, "image should NOT be included when condition is false")
	})

	t.Run("image with missing condition path", func(t *testing.T) {
		img := ChartImage{
			Name:      "test",
			Image:     "test:latest",
			Condition: "nonexistent.path",
		}
		values := common.Values{
			"feature": map[string]interface{}{
				"enabled": true,
			},
		}

		result := shouldIncludeImage(img, values)
		require.False(t, result, "image should NOT be included when condition path doesn't exist")
	})

	t.Run("image with nested condition path", func(t *testing.T) {
		img := ChartImage{
			Name:      "test",
			Image:     "test:latest",
			Condition: "database.postgres.enabled",
		}
		values := common.Values{
			"database": map[string]interface{}{
				"postgres": map[string]interface{}{
					"enabled": true,
				},
			},
		}

		result := shouldIncludeImage(img, values)
		require.True(t, result, "image should be included when nested condition is true")
	})
}
