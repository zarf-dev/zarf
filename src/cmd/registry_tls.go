// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/zarf-dev/zarf/src/types"
)

type sourceRegistryFlags struct {
	sourceRegistryTLS types.RegistryTLSOptions
}

func (f *sourceRegistryFlags) sourceRemoteOptions() types.RemoteOptions {
	opts := defaultRemoteOptions()
	opts.RegistryTLS = f.sourceRegistryTLS
	return opts
}

type destinationRegistryFlags struct {
	destinationRegistryTLS types.RegistryTLSOptions
}

func (f *destinationRegistryFlags) destinationRemoteOptions() types.RemoteOptions {
	opts := defaultRemoteOptions()
	opts.RegistryTLS = f.destinationRegistryTLS
	return opts
}

func addRegistryTLSFlags(cmd *cobra.Command, v *viper.Viper, opts *types.RegistryTLSOptions, role, configPrefix string) {
	prefix := role + "-registry-tls-"
	key := configPrefix + "." + role + "_registry.tls_"
	cmd.Flags().StringVar(&opts.CAFile, prefix+"ca", v.GetString(key+"ca"), fmt.Sprintf("Path to a PEM-encoded CA bundle used to verify %s Zarf package registries (adds to system trust)", role))
	cmd.Flags().StringVar(&opts.CertFile, prefix+"cert", v.GetString(key+"cert"), fmt.Sprintf("Path to a PEM-encoded client certificate chain used to authenticate to %s Zarf package registries", role))
	cmd.Flags().StringVar(&opts.KeyFile, prefix+"key", v.GetString(key+"key"), fmt.Sprintf("Path to the PEM-encoded private key for the %s Zarf package registry client certificate", role))
	// Check resolved values as well as flags so config files and environment variables
	// can supply either member of the pair.
	preRun := cmd.PreRunE
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if (opts.CertFile == "") != (opts.KeyFile == "") {
			return fmt.Errorf("--%scert and --%skey must be provided together", prefix, prefix)
		}
		if preRun != nil {
			return preRun(cmd, args)
		}
		return nil
	}
}
