// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/distribution/reference"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/spf13/cobra"
	"github.com/zarf-dev/zarf/src/config/lang"
	"oras.land/oras-go/v2"
	orasRegistry "oras.land/oras-go/v2/registry"
	orasRemote "oras.land/oras-go/v2/registry/remote"
)

func newRegistryDigestCommand() *cobra.Command {
	var tarballPath string
	var fullRef, plainHTTP, insecureSkipTLSVerify, deprecatedInsecure bool

	cmd := &cobra.Command{
		Use:     "digest IMAGE",
		Short:   "Get the digest of an image",
		Args:    cobra.MaximumNArgs(1),
		Example: lang.CmdToolsRegistryDigestExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			if tarballPath == "" && len(args) == 0 {
				if err := cmd.Help(); err != nil {
					return err
				}
				return errors.New("image reference required without --tarball")
			}
			if fullRef && tarballPath != "" {
				return errors.New("cannot specify --full-ref with --tarball")
			}
			// --insecure used to mean both of these at once; keeping this behavior for anyone still using it.
			if deprecatedInsecure {
				plainHTTP = true
				insecureSkipTLSVerify = true
			}
			var imageRef string
			if len(args) > 0 {
				imageRef = args[0]
			}
			if tarballPath != "" {
				return tarballDigest(cmd.OutOrStdout(), tarballPath, imageRef)
			}
			return runRegistryDigest(cmd.Context(), cmd.OutOrStdout(), imageRef, plainHTTP, insecureSkipTLSVerify, fullRef)
		},
	}
	cmd.Flags().StringVar(&tarballPath, "tarball", "", "(Optional) path to tarball containing the image")
	cmd.Flags().BoolVar(&fullRef, "full-ref", false, "(Optional) if true, print the full image reference by digest")
	cmd.Flags().BoolVar(&plainHTTP, "plain-http", false, "(Optional) if true, use plain HTTP instead of HTTPS")
	cmd.Flags().BoolVar(&insecureSkipTLSVerify, "insecure-skip-tls-verify", false, "(Optional) if true, skip TLS certificate verification")
	cmd.Flags().BoolVar(&deprecatedInsecure, "insecure", false, "(Optional) if true, use plain HTTP and skip TLS certificate verification")
	if err := cmd.Flags().MarkDeprecated("insecure", "use --plain-http and --insecure-skip-tls-verify instead"); err != nil {
		panic(fmt.Errorf("marking --insecure deprecated: %w", err))
	}
	return cmd
}

// tarballDigest computes the digest of a local OCI tarball, without contacting any registry.
func tarballDigest(out io.Writer, tarballPath, tag string) error {
	var t *name.Tag
	if tag != "" {
		parsed, err := name.NewTag(tag)
		if err != nil {
			return fmt.Errorf("parsing tag %q: %w", tag, err)
		}
		t = &parsed
	}
	img, err := tarball.ImageFromPath(tarballPath, t)
	if err != nil {
		return fmt.Errorf("loading image from %q: %w", tarballPath, err)
	}
	digest, err := img.Digest()
	if err != nil {
		return fmt.Errorf("computing digest: %w", err)
	}
	fmt.Fprintln(out, digest.String())
	return nil
}

func normalizeImageRef(imageRef string) (string, error) {
	named, err := reference.ParseNormalizedNamed(imageRef)
	if err != nil {
		return "", fmt.Errorf("parsing image %q: %w", imageRef, err)
	}
	return reference.TagNameOnly(named).String(), nil
}

func runRegistryDigest(ctx context.Context, out io.Writer, imageRef string, plainHTTP, insecureSkipTLSVerify, fullRef bool) error {
	imageRef, err := normalizeImageRef(imageRef)
	if err != nil {
		return err
	}
	conn, err := setupRegistryAuth(ctx, imageRef, plainHTTP, insecureSkipTLSVerify)
	if err != nil {
		return err
	}
	digestFn := func() error {
		return resolveDigest(ctx, out, conn, plainHTTP, insecureSkipTLSVerify, fullRef)
	}
	if conn.tunnel == nil {
		return digestFn()
	}
	defer conn.tunnel.Close()
	return conn.tunnel.Wrap(digestFn)
}

func resolveDigest(ctx context.Context, out io.Writer, conn registryConnection, plainHTTP, insecureSkipTLSVerify, fullRef bool) error {
	ref, err := orasRegistry.ParseReference(conn.ref)
	if err != nil {
		return fmt.Errorf("parsing image %q: %w", conn.ref, err)
	}

	repo := &orasRemote.Repository{
		Reference: ref,
		Client:    conn.client,
	}
	repo.PlainHTTP, err = resolveConnPlainHTTP(ctx, conn, ref.Host(), plainHTTP, insecureSkipTLSVerify)
	if err != nil {
		return err
	}

	desc, err := oras.Resolve(ctx, repo, conn.ref, oras.DefaultResolveOptions)
	if err != nil {
		return fmt.Errorf("resolving digest for %s: %w", conn.ref, err)
	}

	if fullRef {
		fmt.Fprintf(out, "%s/%s@%s\n", ref.Registry, ref.Repository, desc.Digest)
	} else {
		fmt.Fprintln(out, desc.Digest)
	}
	return nil
}
