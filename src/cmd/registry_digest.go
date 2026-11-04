// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"archive/tar"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/distribution/reference"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cobra"
	"github.com/zarf-dev/zarf/src/config/lang"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/oci"
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
				return tarballDigest(cmd.Context(), cmd.OutOrStdout(), tarballPath, imageRef)
			}
			platform, err := cmd.Flags().GetString("platform")
			if err != nil {
				return err
			}
			return runRegistryDigest(cmd.Context(), cmd.OutOrStdout(), imageRef, plainHTTP, insecureSkipTLSVerify, fullRef, platform)
		},
	}
	cmd.Flags().StringVar(&tarballPath, "tarball", "", "(Optional) path to a tar archive of an OCI image layout")
	cmd.Flags().BoolVar(&fullRef, "full-ref", false, "(Optional) if true, print the full image reference by digest")
	cmd.Flags().BoolVar(&plainHTTP, "plain-http", false, "(Optional) if true, use plain HTTP instead of HTTPS")
	cmd.Flags().BoolVar(&insecureSkipTLSVerify, "insecure-skip-tls-verify", false, "(Optional) if true, skip TLS certificate verification")
	cmd.Flags().BoolVar(&deprecatedInsecure, "insecure", false, "(Optional) if true, use plain HTTP and skip TLS certificate verification")
	if err := cmd.Flags().MarkDeprecated("insecure", "use --plain-http and --insecure-skip-tls-verify instead"); err != nil {
		panic(fmt.Errorf("marking --insecure deprecated: %w", err))
	}
	return cmd
}

// tarballDigest computes the digest of an image in a local OCI image layout tar archive,
// without contacting any registry. If ref is empty, the tarball must contain exactly one
// manifest in its index.json.
func tarballDigest(ctx context.Context, out io.Writer, tarballPath, ref string) error {
	if ref == "" {
		digest, err := soleManifestDigest(tarballPath)
		if err != nil {
			return err
		}
		fmt.Fprintln(out, digest)
		return nil
	}
	store, err := oci.NewFromTar(ctx, tarballPath)
	if err != nil {
		return fmt.Errorf("loading OCI image layout from %q: %w", tarballPath, err)
	}
	desc, err := store.Resolve(ctx, ref)
	if err != nil {
		return fmt.Errorf("resolving %q in %q: %w", ref, tarballPath, err)
	}
	fmt.Fprintln(out, desc.Digest.String())
	return nil
}

func soleManifestDigest(tarballPath string) (_ string, err error) {
	f, err := os.Open(tarballPath)
	if err != nil {
		return "", fmt.Errorf("opening %q: %w", tarballPath, err)
	}
	defer func() { err = errors.Join(err, f.Close()) }()

	tr := tar.NewReader(f)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return "", fmt.Errorf("%q does not contain an OCI image layout %s", tarballPath, ocispec.ImageIndexFile)
		}
		if err != nil {
			return "", fmt.Errorf("reading %q: %w", tarballPath, err)
		}
		if path.Clean(hdr.Name) != ocispec.ImageIndexFile {
			continue
		}
		var index ocispec.Index
		if err := json.NewDecoder(tr).Decode(&index); err != nil {
			return "", fmt.Errorf("decoding %s: %w", ocispec.ImageIndexFile, err)
		}
		if len(index.Manifests) != 1 {
			return "", fmt.Errorf("%q contains %d manifests, specify one as IMAGE", tarballPath, len(index.Manifests))
		}
		return index.Manifests[0].Digest.String(), nil
	}
}

func normalizeImageRef(imageRef string) (string, error) {
	named, err := reference.ParseNormalizedNamed(imageRef)
	if err != nil {
		return "", fmt.Errorf("parsing image %q: %w", imageRef, err)
	}
	return reference.TagNameOnly(named).String(), nil
}

func runRegistryDigest(ctx context.Context, out io.Writer, imageRef string, plainHTTP, insecureSkipTLSVerify, fullRef bool, platform string) error {
	imageRef, err := normalizeImageRef(imageRef)
	if err != nil {
		return err
	}
	conn, err := setupRegistryAuth(ctx, imageRef, plainHTTP, insecureSkipTLSVerify)
	if err != nil {
		return err
	}
	digestFn := func() error {
		return resolveDigest(ctx, out, conn, plainHTTP, insecureSkipTLSVerify, fullRef, platform)
	}
	if conn.tunnel == nil {
		return digestFn()
	}
	defer conn.tunnel.Close()
	return conn.tunnel.Wrap(digestFn)
}

func resolveDigest(ctx context.Context, out io.Writer, conn registryConnection, plainHTTP, insecureSkipTLSVerify, fullRef bool, platform string) error {
	ref, err := orasRegistry.ParseReference(conn.ref)
	if err != nil {
		return fmt.Errorf("parsing image %q: %w", conn.ref, err)
	}

	targetPlatform, err := parseTargetPlatform(platform)
	if err != nil {
		return err
	}

	repo := &orasRemote.Repository{
		Reference: ref,
		Client:    conn.client,
	}
	repo.PlainHTTP, err = resolveConnPlainHTTP(ctx, conn, ref.Host(), plainHTTP, insecureSkipTLSVerify)
	if err != nil {
		return err
	}

	desc, err := oras.Resolve(ctx, repo, conn.ref, oras.ResolveOptions{TargetPlatform: targetPlatform})
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

func parseTargetPlatform(platform string) (*ocispec.Platform, error) {
	if platform == "" || platform == "all" {
		return nil, nil
	}
	spec, osVersion, _ := strings.Cut(platform, ":")
	parts := strings.Split(spec, "/")
	if len(parts) < 2 || len(parts) > 3 {
		return nil, fmt.Errorf("invalid platform %q: expected os/arch[/variant][:osversion]", platform)
	}
	p := &ocispec.Platform{OS: parts[0], Architecture: parts[1], OSVersion: osVersion}
	if len(parts) == 3 {
		p.Variant = parts[2]
	}
	return p, nil
}
