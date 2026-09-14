// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cmd

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/component"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/signing"
	"oras.land/oras-go/v2/registry"
)

func newComponentCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "component",
		Short: lang.CmdComponentShort,
		// Once v1beta1 is available this should be unhidden
		Hidden: true,
	}

	v := getViper()
	cmd.AddCommand(newComponentPublishCommand(v))
	cmd.AddCommand(newComponentSignCommand(v))
	cmd.AddCommand(newComponentVerifyCommand(v))
	return cmd
}

type componentPublishOptions struct {
	ociConcurrency int
	retries        int
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

	cmd.Flags().IntVar(&o.ociConcurrency, "oci-concurrency", v.GetInt(VPkgOCIConcurrency), lang.CmdPackageFlagConcurrency)
	cmd.Flags().IntVar(&o.retries, "retries", v.GetInt(VPkgPublishRetries), lang.CmdPackageFlagRetries)
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

	_, err := component.Publish(cmd.Context(), args[0], destination, component.PublishOptions{
		OCIConcurrency: o.ociConcurrency,
		Retries:        o.retries,
		RemoteOptions:  defaultRemoteOptions(),
	})
	return err
}

// componentSignOptions intentionally embeds the shared package-signing
// options: a component manifest and a package blob use the same keyless and
// key-based Sigstore configuration.
type componentSignOptions struct {
	packageSignOptions
}

func newComponentSignCommand(v *viper.Viper) *cobra.Command {
	o := &componentSignOptions{}
	cmd := &cobra.Command{
		Use:     "sign COMPONENT_SOURCE",
		Aliases: []string{"s"},
		Args:    cobra.ExactArgs(1),
		Short:   lang.CmdComponentSignShort,
		Example: lang.CmdComponentSignExample,
		RunE:    o.run,
	}

	cmd.Flags().StringVar(&o.signingKeyPath, "signing-key", v.GetString(VPkgSignSigningKey), lang.CmdPackageSignFlagSigningKey)
	cmd.Flags().StringVar(&o.signingKeyPassword, "signing-key-pass", v.GetString(VPkgSignSigningKeyPassword), lang.CmdPackageSignFlagSigningKeyPass)
	cmd.Flags().BoolVar(&o.keyless, "keyless", v.GetBool(VPkgSignKeyless), lang.CmdPackageSignFlagKeyless)
	cmd.Flags().StringVar(&o.identityToken, "identity-token", v.GetString(VPkgSignIdentityToken), lang.CmdPackageSignFlagIdentityToken)
	cmd.Flags().StringVar(&o.fulcioURL, "fulcio-url", v.GetString(VPkgSignFulcioURL), lang.CmdPackageSignFlagFulcioURL)
	cmd.Flags().StringVar(&o.fulcioAuthFlow, "fulcio-auth-flow", v.GetString(VPkgSignFulcioAuthFlow), lang.CmdPackageSignFlagFulcioAuthFlow)
	cmd.Flags().StringVar(&o.oidcIssuer, "oidc-issuer", v.GetString(VPkgSignOIDCIssuer), lang.CmdPackageSignFlagOIDCIssuer)
	cmd.Flags().StringVar(&o.oidcClientID, "oidc-client-id", v.GetString(VPkgSignOIDCClientID), lang.CmdPackageSignFlagOIDCClientID)
	cmd.Flags().StringVar(&o.rekorURL, "rekor-url", v.GetString(VPkgSignRekorURL), lang.CmdPackageSignFlagRekorURL)
	cmd.Flags().BoolVar(&o.tlogUpload, "tlog-upload", v.GetBool(VPkgSignTlogUpload), lang.CmdPackageSignFlagTlogUpload)
	cmd.Flags().BoolVar(&o.confirm, "confirm", false, lang.CmdPackageSignFlagConfirm)
	cmd.Flags().StringVar(&o.tsaServerURL, "tsa-server-url", v.GetString(VPkgSignTSAServerURL), lang.CmdPackageSignFlagTSAServerURL)
	cmd.MarkFlagsMutuallyExclusive("keyless", "signing-key")

	return cmd
}

func (o *componentSignOptions) run(cmd *cobra.Command, args []string) error {
	if !o.keyless && o.signingKeyPath == "" {
		return errors.New("--signing-key is required (or pass --keyless for Sigstore keyless flow)")
	}

	componentSource := strings.TrimPrefix(args[0], helpers.OCIURLPrefix)
	componentRef, err := registry.ParseReference(componentSource)
	if err != nil {
		return fmt.Errorf("component source must be a published OCI reference: %w", err)
	}
	if err := componentRef.Validate(); err != nil {
		return fmt.Errorf("invalid component source: %w", err)
	}

	if o.keyless {
		logger.From(cmd.Context()).Info("signing component manifest via Sigstore keyless flow")
	} else {
		logger.From(cmd.Context()).Info("signing component manifest with provided key")
	}

	err = signing.CosignSignManifestWithOptions(cmd.Context(), componentRef.String(), o.buildSignBlobOptions(cmd), defaultRemoteOptions())
	if err != nil {
		return fmt.Errorf("failed to sign component manifest: %w", err)
	}
	if o.keyless {
		if info, bundleErr := signing.GetManifestBundleInfo(cmd.Context(), componentRef.String(), defaultRemoteOptions()); bundleErr == nil {
			if info.Identity != "" {
				logger.From(cmd.Context()).Info("keyless signed component", "identity", info.Identity, "issuer", info.Issuer)
			}
		} else {
			logger.From(cmd.Context()).Debug("could not read component signature bundle metadata", "error", bundleErr)
		}
	}

	logger.From(cmd.Context()).Info("component manifest signed successfully", "source", helpers.OCIURLPrefix+componentRef.String())
	return nil
}

type componentVerifyOptions struct {
	packageVerifyFlags
}

func newComponentVerifyCommand(v *viper.Viper) *cobra.Command {
	o := &componentVerifyOptions{}
	cmd := &cobra.Command{
		Use:     "verify COMPONENT_SOURCE",
		Aliases: []string{"v"},
		Args:    cobra.ExactArgs(1),
		Short:   lang.CmdComponentVerifyShort,
		Long:    lang.CmdComponentVerifyLong,
		Example: lang.CmdComponentVerifyExample,
		RunE:    o.run,
	}

	cmd.Flags().StringVarP(&o.publicKeyPath, "key", "k", v.GetString(VPkgPublicKey), lang.CmdPackageVerifyFlagKey)
	cmd.Flags().AddFlagSet(newKeylessVerifyFlagSet(v, &o.packageVerifyFlags))
	if err := cmd.Flags().SetAnnotation("key", flagGroupAnnotation, []string{verifyFlagGroupTitle}); err != nil {
		panic(err)
	}
	markVerifyFlagsMutuallyExclusive(cmd)

	return cmd
}

func (o *componentVerifyOptions) run(cmd *cobra.Command, args []string) error {
	componentSource := strings.TrimPrefix(args[0], helpers.OCIURLPrefix)
	componentRef, err := registry.ParseReference(componentSource)
	if err != nil {
		return fmt.Errorf("component source must be a published OCI reference: %w", err)
	}
	if err := componentRef.Validate(); err != nil {
		return fmt.Errorf("invalid component source: %w", err)
	}

	l := logger.From(cmd.Context())
	l.Info("verifying component manifest signature", "source", helpers.OCIURLPrefix+componentRef.String())
	if err := signing.CosignVerifyManifestWithOptions(cmd.Context(), componentRef.String(), *o.buildVerifyBlobOptions(cmd, v), defaultRemoteOptions()); err != nil {
		return fmt.Errorf("component signature verification failed: %w", err)
	}

	l.Info("component signature verification", "status", "PASSED")
	return nil
}
