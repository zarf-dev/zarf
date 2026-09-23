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

// componentSignOptions uses the shared signing configuration while retaining
// only component-specific execution state.
type componentSignOptions struct {
	confirm bool
	packageSigningFlags
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

	cmd.Flags().AddFlagSet(newSigningFlagSet(v, &o.packageSigningFlags, packageSigningViperKeys{
		signingKey:         VPkgSignSigningKey,
		signingKeyPassword: VPkgSignSigningKeyPassword,
		keyless:            VPkgSignKeyless,
		identityToken:      VPkgSignIdentityToken,
		fulcioURL:          VPkgSignFulcioURL,
		fulcioAuthFlow:     VPkgSignFulcioAuthFlow,
		oidcIssuer:         VPkgSignOIDCIssuer,
		oidcClientID:       VPkgSignOIDCClientID,
		rekorURL:           VPkgSignRekorURL,
		tlogUpload:         VPkgSignTlogUpload,
		tsaServerURL:       VPkgSignTSAServerURL,
	}, lang.CmdPackageSignFlagSigningKey, lang.CmdPackageSignFlagSigningKeyPass))
	cmd.Flags().BoolVar(&o.confirm, "confirm", false, lang.CmdPackageSignFlagConfirm)
	cmd.MarkFlagsMutuallyExclusive("keyless", "signing-key")

	return cmd
}

func (o *componentSignOptions) run(cmd *cobra.Command, args []string) error {
	if err := o.validateSigningMode(); err != nil {
		return err
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

	signOpts := signing.DefaultSignManifestOptions()
	signOpts.Key = o.signingKeyPath
	signOpts.Password = o.signingKeyPassword
	signOpts.IdentityToken = o.identityToken
	signOpts.FulcioURL = o.fulcioURL
	signOpts.FulcioAuthFlow = o.fulcioAuthFlow
	signOpts.OIDCIssuer = o.oidcIssuer
	signOpts.OIDCClientID = o.oidcClientID
	signOpts.RekorURL = o.rekorURL
	signOpts.TlogUpload = o.resolveTlogUpload(cmd, v, VPkgSignTlogUpload)
	signOpts.SkipConfirmation = o.confirm
	signOpts.TSAServerURL = o.tsaServerURL
	err = signing.SignManifest(cmd.Context(), componentRef.String(), signOpts, defaultRemoteOptions())
	if err != nil {
		return fmt.Errorf("failed to sign component manifest: %w", err)
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
	verifyOpts := signing.DefaultVerifyManifestOptions()
	verifyOpts.Key = o.publicKeyPath
	verifyOpts.CertificateIdentity = o.certificateIdentity
	verifyOpts.CertificateIdentityRegexp = o.certificateIdentityRegexp
	verifyOpts.CertificateOIDCIssuer = o.certificateOIDCIssuer
	verifyOpts.CertificateOIDCIssuerRegexp = o.certificateOIDCIssuerRegexp
	verifyOpts.TrustedRoot = o.trustedRoot
	verifyOpts.InsecureIgnoreTlog = o.validateKeylessVerifyTlog(cmd, getViper())
	verifyOpts.UseSignedTimestamps = o.useSignedTimestamps
	if err := signing.VerifyManifest(cmd.Context(), componentRef.String(), verifyOpts, defaultRemoteOptions()); err != nil {
		return fmt.Errorf("component signature verification failed: %w", err)
	}

	l.Info("component signature verification", "status", "PASSED")
	return nil
}
