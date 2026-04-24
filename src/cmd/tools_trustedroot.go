// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cmd

import (
	"context"
	"errors"

	"github.com/sigstore/cosign/v3/cmd/cosign/cli/options"
	"github.com/sigstore/cosign/v3/cmd/cosign/cli/trustedroot"
	"github.com/spf13/cobra"

	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/utils"
)

func newTrustedRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trusted-root",
		Short: lang.CmdToolsTrustedRootShort,
	}

	cmd.AddCommand(newTrustedRootCreateCommand())

	return cmd
}

// newTrustedRootCreateCommand mirrors cosign's own `trusted-root create` wiring.
//
// Cosign maintains two struct representations of this command: options.TrustedRootCreateOptions
// (flag binding, AddFlags) and trustedroot.CreateCmd (execution, Exec). Neither has the methods
// we need on its own, so we bind flags onto Options and translate to CreateCmd before Exec.
//
// Reference implementation:
//
//	https://github.com/sigstore/cosign/blob/main/cmd/cosign/cli/trustedroot.go :: trustedRootCreate()
//
// Why not mount cosign's exported cli.TrustedRoot() cobra command directly? Its RunE reads
// cosign's package-level `ro` RootOptions which is zero-valued outside cosign's own root
// command (breaking the timeout), its help text and examples reference `cosign` not `zarf`,
// and injecting our validation guard would require fragile RunE wrapping.
func newTrustedRootCreateCommand() *cobra.Command {
	o := &options.TrustedRootCreateOptions{}

	cmd := &cobra.Command{
		Use:   "create",
		Short: lang.CmdToolsTrustedRootCreateShort,
		Long:  lang.CmdToolsTrustedRootCreateLong,
		Example: `  # Retrieve the public Sigstore trusted root via TUF
  zarf tools trusted-root create --with-default-services --out trusted_root.json

  # Compose a trusted root from custom Sigstore infrastructure
  zarf tools trusted-root create \
    --fulcio="url=https://fulcio.example.com,certificate-chain=/path/to/fulcio.pem" \
    --rekor="url=https://rekor.example.com,public-key=/path/to/rekor.pub,start-time=2024-01-01T00:00:00Z" \
    --out trusted_root.json

  # Extend public defaults with additional private TSA
  zarf tools trusted-root create \
    --with-default-services \
    --tsa="url=https://tsa.example.com,certificate-chain=/path/to/tsa.pem" \
    --out trusted_root.json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Intentional divergence from cosign: cosign produces an empty trusted root JSON
			// on bare invocation. Zarf errors instead, since an empty trusted root is
			// functionally useless for verification and almost always indicates a missed
			// flag rather than intended output.
			if !o.WithDefaultServices &&
				len(o.Fulcio) == 0 &&
				len(o.Rekor) == 0 &&
				len(o.CTFE) == 0 &&
				len(o.TSA) == 0 &&
				len(o.CertChain) == 0 &&
				len(o.CtfeKeyPath) == 0 &&
				len(o.RekorKeyPath) == 0 &&
				len(o.TSACertChainPath) == 0 {
				return errors.New("provide --with-default-services to retrieve the public Sigstore trusted root, or specify --fulcio/--rekor/--ctfe/--tsa to compose a custom trusted root")
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), utils.CosignDefaultTimeout)
			defer cancel()
			return optionsToCreateCmd(o).Exec(ctx)
		},
	}

	o.AddFlags(cmd)
	return cmd
}

// optionsToCreateCmd translates cosign's flag-binding struct to its execution struct.
// New fields on either struct added in future cosign versions must be added here to
// reach Exec. TestTrustedRootTranslationCoverage verifies that every exported field on
// options.TrustedRootCreateOptions is forwarded to the corresponding field on
// trustedroot.CreateCmd.
func optionsToCreateCmd(o *options.TrustedRootCreateOptions) *trustedroot.CreateCmd {
	return &trustedroot.CreateCmd{
		FulcioSpecs:         o.Fulcio,
		RekorSpecs:          o.Rekor,
		CTFESpecs:           o.CTFE,
		TSASpecs:            o.TSA,
		WithDefaultServices: o.WithDefaultServices,
		NoDefaultFulcio:     o.NoDefaultFulcio,
		NoDefaultCTFE:       o.NoDefaultCTFE,
		NoDefaultTSA:        o.NoDefaultTSA,
		NoDefaultRekor:      o.NoDefaultRekor,
		Out:                 o.Out,
		// Deprecated flags — cosign accepts them with warnings; pass through for parity.
		CertChain:        o.CertChain,
		FulcioURI:        o.FulcioURI,
		CtfeKeyPath:      o.CtfeKeyPath,
		CtfeStartTime:    o.CtfeStartTime,
		CtfeEndTime:      o.CtfeEndTime,
		CtfeURL:          o.CtfeURL,
		RekorKeyPath:     o.RekorKeyPath,
		RekorStartTime:   o.RekorStartTime,
		RekorEndTime:     o.RekorEndTime,
		RekorURL:         o.RekorURL,
		TSACertChainPath: o.TSACertChainPath,
		TSAURI:           o.TSAURI,
	}
}
