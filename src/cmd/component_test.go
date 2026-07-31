// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cmd

import (
	"bytes"
	"context"
	"errors"
	"path"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/registry"

	"github.com/zarf-dev/zarf/src/config"
	componentpackager "github.com/zarf-dev/zarf/src/pkg/packager/component"
)

func TestRootRegistersComponentPublishCommand(t *testing.T) {
	oldV := v
	oldCachePath := config.CommonOptions.CachePath
	oldPlainHTTP := plainHTTP
	oldInsecureSkipTLSVerify := insecureSkipTLSVerify
	v = newTestViper()
	t.Cleanup(func() {
		v = oldV
		config.CommonOptions.CachePath = oldCachePath
		plainHTTP = oldPlainHTTP
		insecureSkipTLSVerify = oldInsecureSkipTLSVerify
	})

	root := NewZarfCommand()

	componentCmd, _, err := root.Find([]string{"component"})
	require.NoError(t, err)
	require.Equal(t, "component", componentCmd.Name())

	publishCmd, _, err := root.Find([]string{"component", "publish"})
	require.NoError(t, err)
	require.Equal(t, "publish", publishCmd.Name())
	require.NotNil(t, publishCmd.Flags().Lookup("flavor"))
	require.NotNil(t, publishCmd.Flags().ShorthandLookup("f"))
	require.NotNil(t, publishCmd.Flags().Lookup("tag"))
	require.NotNil(t, publishCmd.Flags().ShorthandLookup("t"))
	require.NotNil(t, publishCmd.Flags().Lookup("oci-concurrency"))
	require.NotNil(t, publishCmd.Flags().Lookup("retries"))
}

func TestComponentPublishArgValidation(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "missing args",
			args: nil,
		},
		{
			name: "missing destination",
			args: []string{"component.yaml"},
		},
		{
			name: "too many args",
			args: []string{"component.yaml", "oci://registry.example/team", "extra"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newComponentPublishCommand(newTestViper())
			_, err := executeComponentTestCommand(cmd, tt.args...)
			require.ErrorContains(t, err, "accepts 2 arg(s)")
		})
	}
}

func TestComponentPublishDestinationParsing(t *testing.T) {
	tests := []struct {
		name            string
		destination     string
		want            registry.Reference
		wantErrContains string
	}{
		{
			name:        "valid namespace",
			destination: "oci://registry.example/team/components",
			want: registry.Reference{
				Registry:   "registry.example",
				Repository: "team/components",
			},
		},
		{
			name:            "missing oci scheme",
			destination:     "registry.example/team/components",
			wantErrContains: "oci://",
		},
		{
			name:            "missing repository namespace",
			destination:     "oci://registry.example",
			wantErrContains: "missing registry or repository",
		},
		{
			name:            "tagged destination rejected",
			destination:     "oci://registry.example/team/components:v1",
			wantErrContains: "must not include a tag or digest",
		},
		{
			name:            "digest destination rejected",
			destination:     "oci://registry.example/team/components@sha256:2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824",
			wantErrContains: "must not include a tag or digest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseComponentPublishDestination(tt.destination)
			if tt.wantErrContains != "" {
				require.ErrorContains(t, err, tt.wantErrContains)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestComponentPublishPassesOptionsAndPrintsPublishedReference(t *testing.T) {
	oldPublish := componentPublish
	oldCachePath := config.CommonOptions.CachePath
	oldPlainHTTP := plainHTTP
	oldInsecureSkipTLSVerify := insecureSkipTLSVerify
	t.Cleanup(func() {
		componentPublish = oldPublish
		config.CommonOptions.CachePath = oldCachePath
		plainHTTP = oldPlainHTTP
		insecureSkipTLSVerify = oldInsecureSkipTLSVerify
	})

	config.CommonOptions.CachePath = t.TempDir()
	plainHTTP = true
	insecureSkipTLSVerify = true
	called := false
	componentPublish = func(ctx context.Context, componentFile string, dst registry.Reference, opts componentpackager.PublishOptions) (registry.Reference, error) {
		called = true
		require.NotNil(t, ctx)
		require.Equal(t, "component.yaml", componentFile)
		require.Equal(t, registry.Reference{Registry: "registry.example", Repository: "team/components"}, dst)
		require.Equal(t, 7, opts.OCIConcurrency)
		require.Equal(t, 3, opts.Retries)
		require.Equal(t, "upstream", opts.Flavor)
		require.Equal(t, "v1", opts.Tag)
		require.Equal(t, config.CommonOptions.CachePath, opts.CachePath)
		require.True(t, opts.RemoteOptions.PlainHTTP)
		require.True(t, opts.RemoteOptions.InsecureSkipTLSVerify)

		return registry.Reference{
			Registry:   dst.Registry,
			Repository: path.Join(dst.Repository, "demo"),
			Reference:  opts.Tag,
		}, nil
	}

	cmd := newComponentPublishCommand(newTestViper())
	out, err := executeComponentTestCommand(cmd,
		"--flavor", "upstream",
		"--tag", "v1",
		"--oci-concurrency", "7",
		"--retries", "3",
		"component.yaml",
		"oci://registry.example/team/components",
	)

	require.NoError(t, err)
	require.True(t, called)
	require.Equal(t, "oci://registry.example/team/components/demo:v1\n", out)
}

func TestComponentPublishDoesNotCallPackagerForInvalidDestination(t *testing.T) {
	oldPublish := componentPublish
	t.Cleanup(func() { componentPublish = oldPublish })
	componentPublish = func(context.Context, string, registry.Reference, componentpackager.PublishOptions) (registry.Reference, error) {
		return registry.Reference{}, errors.New("component.Publish should not be called")
	}

	cmd := newComponentPublishCommand(newTestViper())
	_, err := executeComponentTestCommand(cmd, "component.yaml", "registry.example/team/components")

	require.ErrorContains(t, err, "oci://")
}

func executeComponentTestCommand(cmd *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)
	cmd.SetContext(context.Background())
	err := cmd.Execute()
	return buf.String(), err
}
