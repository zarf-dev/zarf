// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"oras.land/oras-go/v2/registry"

	"github.com/zarf-dev/zarf/src/config/lang"
	componentpackager "github.com/zarf-dev/zarf/src/pkg/packager/component"
)

var componentPublish = componentpackager.Publish

type componentPublishOptions struct {
	flavor         string
	tag            string
	ociConcurrency int
	retries        int
}

func newComponentCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "component",
		Short: lang.CmdComponentShort,
	}

	cmd.AddCommand(newComponentPublishCommand(getViper()))

	return cmd
}

func newComponentPublishCommand(v *viper.Viper) *cobra.Command {
	o := &componentPublishOptions{}

	cmd := &cobra.Command{
		Use:     "publish COMPONENT_FILE OCI_REPOSITORY",
		Short:   lang.CmdComponentPublishShort,
		Example: lang.CmdComponentPublishExample,
		Args:    cobra.ExactArgs(2),
		RunE:    o.run,
	}

	cmd.Flags().StringVarP(&o.flavor, "flavor", "f", v.GetString(VPkgCreateFlavor), lang.CmdComponentPublishFlagFlavor)
	cmd.Flags().StringVarP(&o.tag, "tag", "t", "", lang.CmdComponentPublishFlagTag)
	cmd.Flags().IntVar(&o.ociConcurrency, "oci-concurrency", v.GetInt(VPkgOCIConcurrency), lang.CmdPackageFlagConcurrency)
	cmd.Flags().IntVar(&o.retries, "retries", v.GetInt(VPkgPublishRetries), lang.CmdPackageFlagRetries)

	return cmd
}

func (o *componentPublishOptions) run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	dstRef, err := parseComponentPublishDestination(args[1])
	if err != nil {
		return err
	}

	cachePath, err := getCachePath(ctx)
	if err != nil {
		return err
	}

	publishedRef, err := componentPublish(ctx, args[0], dstRef, componentpackager.PublishOptions{
		OCIConcurrency: o.ociConcurrency,
		Retries:        o.retries,
		Flavor:         o.flavor,
		Tag:            o.tag,
		CachePath:      cachePath,
		RemoteOptions:  defaultRemoteOptions(),
	})
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "oci://%s\n", publishedRef.String()); err != nil {
		return fmt.Errorf("writing published component reference: %w", err)
	}
	return nil
}

func parseComponentPublishDestination(destination string) (registry.Reference, error) {
	if !helpers.IsOCIURL(destination) {
		return registry.Reference{}, errors.New("registry must be prefixed with 'oci://'")
	}

	ref, err := registry.ParseReference(strings.TrimPrefix(destination, helpers.OCIURLPrefix))
	if err != nil {
		return registry.Reference{}, err
	}
	if ref.Reference != "" {
		return registry.Reference{}, errors.New("registry namespace must not include a tag or digest")
	}
	return ref, nil
}
