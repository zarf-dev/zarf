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
	"github.com/zarf-dev/zarf/src/pkg/signing"
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
	flavor             string
	ociConcurrency     int
	retries            int
	signingKeyPath     string
	signingKeyPassword string
	confirm            bool
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
	cmd.Flags().IntVar(&o.retries, "retries", v.GetInt(VPkgPublishRetries), lang.CmdPackageFlagRetries)
	cmd.Flags().StringVar(&o.signingKeyPath, "signing-key", v.GetString(VPkgPublishSigningKey), lang.CmdPackagePublishFlagSigningKey)
	cmd.Flags().StringVar(&o.signingKeyPassword, "signing-key-pass", v.GetString(VPkgPublishSigningKeyPassword), lang.CmdPackagePublishFlagSigningKeyPassword)
	cmd.Flags().BoolVarP(&o.confirm, "confirm", "c", false, lang.CmdComponentPublishFlagConfirm)
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

	signOpts := signing.DefaultSignBlobOptions()
	signOpts.Key = o.signingKeyPath
	signOpts.Password = o.signingKeyPassword
	signOpts.SkipConfirmation = o.confirm
	_, err := packager.PublishComponent(cmd.Context(), args[0], destination, packager.PublishComponentOptions{
		Flavor:          o.flavor,
		SignBlobOptions: signOpts,
		OCIConcurrency:  o.ociConcurrency,
		Retries:         o.retries,
		RemoteOptions:   defaultRemoteOptions(),
	})
	return err
}
