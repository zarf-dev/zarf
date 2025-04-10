// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"github.com/zarf-dev/zarf/src/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestPackageList(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		outputFormat outputFormat
		file         string
	}{
		{
			name:         "json package list",
			outputFormat: outputJSON,
			file:         "expected.json",
		},
		{
			name:         "yaml package list",
			outputFormat: outputYAML,
			file:         "expected.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			c := &cluster.Cluster{
				Clientset: fake.NewClientset(),
			}

			packages := []types.DeployedPackage{
				{
					Name: "package1",
					Data: v1alpha1.ZarfPackage{
						Metadata: v1alpha1.ZarfMetadata{
							Version: "0.42.0",
						},
					},
					DeployedComponents: []types.DeployedComponent{
						{
							Name: "component1",
						},
						{
							Name: "component2",
						},
					},
				},
				{
					Name: "package2",
					Data: v1alpha1.ZarfPackage{
						Metadata: v1alpha1.ZarfMetadata{
							Version: "1.0.0",
						},
					},
					DeployedComponents: []types.DeployedComponent{
						{
							Name: "component3",
						},
						{
							Name: "component4",
						},
					},
				},
			}

			for _, p := range packages {
				b, err := json.Marshal(p)
				require.NoError(t, err)
				secret := corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      strings.Join([]string{config.ZarfPackagePrefix, p.Name}, ""),
						Namespace: "zarf",
						Labels: map[string]string{
							cluster.ZarfPackageInfoLabel: p.Name,
						},
					},
					Data: map[string][]byte{
						"data": b,
					},
				}
				_, err = c.Clientset.CoreV1().Secrets("zarf").Create(ctx, &secret, metav1.CreateOptions{})
				require.NoError(t, err)
			}
			buf := new(bytes.Buffer)
			listOpts := packageListOptions{
				outputFormat: tt.outputFormat,
				outputWriter: buf,
				cluster:      c,
			}
			err := listOpts.run(ctx)
			require.NoError(t, err)
			b, err := os.ReadFile(filepath.Join("testdata", "package-list", tt.file))
			require.NoError(t, err)
			if tt.outputFormat == outputJSON {
				require.JSONEq(t, string(b), buf.String())
			}
			if tt.outputFormat == outputYAML {
				require.YAMLEq(t, string(b), buf.String())
			}
		})
	}
}

func TestPackageInspectManifests(t *testing.T) {
	t.Parallel()
	lint.ZarfSchema = testutil.LoadSchema(t, "../../zarf.schema.json")
	tmpdir := t.TempDir()
	createOpts := packageCreateOptions{
		confirm: true,
		output:  tmpdir,
	}
	definitionPath := filepath.Join("testdata", "inspect-manifests", "manifest")
	err := createOpts.run(context.Background(), []string{definitionPath})
	require.NoError(t, err)
	buf := new(bytes.Buffer)
	opts := packageInspectManifestsOpts{
		outputWriter: buf,
	}
	packagePath := filepath.Join(tmpdir, fmt.Sprintf("zarf-package-manifests-%s.tar.zst", config.GetArch()))
	err = opts.run(context.Background(), []string{packagePath})
	require.NoError(t, err)
	b, err := os.ReadFile(filepath.Join("testdata", "inspect-manifests", "manifest", "expected.yaml"))
	require.NoError(t, err)
	require.YAMLEq(t, string(b), buf.String())
}
