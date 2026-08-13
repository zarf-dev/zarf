// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cmd

import (
	"errors"
	"path"
	"strings"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/packager"
	"oras.land/oras-go/v2/registry"
)

func newComponentCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "component",
		Short: lang.CmdComponentShort,
	}

	cmd.AddCommand(newComponentPublishCommand(getViper()))
	return cmd
}

type componentPublishOptions struct {
	flavor         string
	ociConcurrency int
}

func newComponentPublishCommand(v *viper.Viper) *cobra.Command {
	o := &componentPublishOptions{}
	cmd := &cobra.Command{
		Use:     "publish COMPONENT_FILE OCI_REPOSITORY",
		Short:   lang.CmdComponentPublishShort,
		Example: lang.CmdComponentPublishExample,
		Args:    cobra.ExactArgs(2),
		// Once v1beta1 is available this should be unhidden
		Hidden: true,
		RunE:   o.run,
	}

	cmd.Flags().StringVarP(&o.flavor, "flavor", "f", v.GetString(VPkgCreateFlavor), lang.CmdComponentPublishFlagFlavor)
	cmd.Flags().IntVar(&o.ociConcurrency, "oci-concurrency", v.GetInt(VPkgOCIConcurrency), lang.CmdPackageFlagConcurrency)
	return cmd
}

func (o *componentPublishOptions) run(cmd *cobra.Command, args []string) error {
	if !helpers.IsOCIURL(args[1]) {
		return errors.New("registry must be prefixed with 'oci://'")
	}

	parts := strings.Split(strings.TrimPrefix(args[1], helpers.OCIURLPrefix), "/")
	destination := registry.Reference{
		Registry:   parts[0],
		Repository: path.Join(parts[1:]...),
	}
	if err := destination.ValidateRegistry(); err != nil {
		return err
	}

	_, err := packager.PublishComponent(cmd.Context(), args[0], destination, packager.PublishComponentOptions{
		Flavor:         o.flavor,
		OCIConcurrency: o.ociConcurrency,
		RemoteOptions:  defaultRemoteOptions(),
	})
	return err
}
