// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/distribution/reference"
	"github.com/spf13/cobra"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/internal/dns"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/images"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/ocischeme"
	orasRegistry "oras.land/oras-go/v2/registry"
	orasRemote "oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

// registryListResponseHeaderTimeout matches the default used by images.PushOptions/PullOptions.
const registryListResponseHeaderTimeout = 10 * time.Second

func newRegistryListCommand() *cobra.Command {
	var fullRef, omitDigestTags, plainHTTP, insecureSkipTLSVerify, deprecatedInsecure bool

	cmd := &cobra.Command{
		Use:     "ls REPO",
		Short:   "List the tags in a repo",
		Args:    cobra.ExactArgs(1),
		Example: lang.CmdToolsRegistryListExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			// --insecure used to mean both of these at once; keeping this behavior for anyone still using it.
			if deprecatedInsecure {
				plainHTTP = true
				insecureSkipTLSVerify = true
			}
			return runRegistryList(cmd.Context(), cmd.OutOrStdout(), args[0], plainHTTP, insecureSkipTLSVerify, fullRef, omitDigestTags)
		},
	}
	cmd.Flags().BoolVar(&fullRef, "full-ref", false, "(Optional) if true, print the full image reference")
	cmd.Flags().BoolVarP(&omitDigestTags, "omit-digest-tags", "O", false, "(Optional), if true, omit digest tags (e.g., ':sha256-...')")
	cmd.Flags().BoolVar(&plainHTTP, "plain-http", false, "(Optional) if true, use plain HTTP instead of HTTPS")
	cmd.Flags().BoolVar(&insecureSkipTLSVerify, "insecure-skip-tls-verify", false, "(Optional) if true, skip TLS certificate verification")
	cmd.Flags().BoolVar(&deprecatedInsecure, "insecure", false, "(Optional) if true, use plain HTTP and skip TLS certificate verification")
	if err := cmd.Flags().MarkDeprecated("insecure", "use --plain-http and --insecure-skip-tls-verify instead"); err != nil {
		panic(fmt.Errorf("marking --insecure deprecated: %w", err))
	}
	return cmd
}

func normalizeRepoRef(repoRef string) (string, error) {
	named, err := reference.ParseNormalizedNamed(repoRef)
	if err != nil {
		return "", fmt.Errorf("parsing repo %q: %w", repoRef, err)
	}
	if !reference.IsNameOnly(named) {
		return "", fmt.Errorf("repo %q must not include a tag or digest", repoRef)
	}
	return named.Name(), nil
}

func runRegistryList(ctx context.Context, out io.Writer, repoRef string, plainHTTP, insecureSkipTLSVerify, fullRef, omitDigestTags bool) error {
	repoRef, err := normalizeRepoRef(repoRef)
	if err != nil {
		return err
	}
	conn, err := setupRegistryAuth(ctx, repoRef, plainHTTP, insecureSkipTLSVerify)
	if err != nil {
		return err
	}
	listFn := func() error {
		return listRegistryTags(ctx, out, conn, plainHTTP, insecureSkipTLSVerify, fullRef, omitDigestTags)
	}
	if conn.tunnel == nil {
		return listFn()
	}
	defer conn.tunnel.Close()
	return conn.tunnel.Wrap(listFn)
}

// registryConnection bundles the reference, auth client, tunnel, and resolved scheme needed to list a repo's tags.
type registryConnection struct {
	ref            string
	client         *auth.Client
	tunnel         *cluster.Tunnel
	plainHTTP      bool
	plainHTTPKnown bool
}

// registryHost returns the registry host:port for repoRef.
func registryHost(repoRef string) (string, error) {
	ref, err := orasRegistry.ParseReference(repoRef)
	if err != nil {
		return "", fmt.Errorf("parsing repo %q: %w", repoRef, err)
	}
	return ref.Host(), nil
}

// setupRegistryAuth builds an ORAS auth client for repoRef, transparently tunneling to and authenticating with a Zarf-managed registry when repoRef targets one, and otherwise falling back to the default Docker credential store.
func setupRegistryAuth(ctx context.Context, repoRef string, plainHTTP, insecureSkipTLSVerify bool) (registryConnection, error) {
	l := logger.From(ctx)

	client, err := images.NewAuthClientFromDocker(ctx, insecureSkipTLSVerify, registryListResponseHeaderTimeout, nil)
	if err != nil {
		return registryConnection{}, err
	}
	conn := registryConnection{ref: repoRef, client: client}

	c, err := cluster.New(ctx)
	if err != nil {
		// Not connected to a Zarf-managed cluster; use the default Docker credentials.
		return conn, nil
	}

	l.Info("retrieving registry information from Zarf state")

	s, err := c.LoadState(ctx)
	if err != nil {
		l.Warn("could not get Zarf state from Kubernetes cluster, continuing without state information", "error", err.Error())
		return conn, nil
	}

	// Check to see if it matches the existing internal address.
	if !strings.HasPrefix(repoRef, s.RegistryInfo.Address) {
		return conn, nil
	}

	endpoint, tunnel, err := c.ConnectToZarfRegistryEndpoint(ctx, s.RegistryInfo)
	if err != nil {
		return registryConnection{}, err
	}
	conn.tunnel = tunnel

	if tunnel != nil {
		l.Info("opening a tunnel to the Zarf registry", "localEndpoint", endpoint, "clusterAddress", s.RegistryInfo.Address)
		givenAddress := fmt.Sprintf("%s/", s.RegistryInfo.Address)
		tunnelAddress := fmt.Sprintf("%s/", endpoint)
		conn.ref = strings.Replace(repoRef, givenAddress, tunnelAddress, 1)
	}

	credentialHost, err := registryHost(conn.ref)
	if err != nil {
		return registryConnection{}, err
	}

	client.Credential = auth.StaticCredential(credentialHost, auth.Credential{
		Username: s.RegistryInfo.PushUsername,
		Password: s.RegistryInfo.PushPassword,
	})

	if s.RegistryInfo.ShouldUseMTLS() {
		t, err := getZarfRegistryMTLSTransport(ctx, c)
		if err != nil {
			return registryConnection{}, err
		}
		client.Client.Transport = t
	}

	resolvedPlainHTTP, err := s.RegistryInfo.ResolvePlainHTTP(ctx, credentialHost, plainHTTP, ocischeme.ProbeOptions{InsecureSkipTLSVerify: insecureSkipTLSVerify})
	if err != nil {
		return registryConnection{}, err
	}
	conn.plainHTTP = resolvedPlainHTTP
	conn.plainHTTPKnown = true

	return conn, nil
}

func listRegistryTags(ctx context.Context, out io.Writer, conn registryConnection, plainHTTP, insecureSkipTLSVerify, fullRef, omitDigestTags bool) error {
	ref, err := orasRegistry.ParseReference(conn.ref)
	if err != nil {
		return fmt.Errorf("parsing repo %q: %w", conn.ref, err)
	}

	repo := &orasRemote.Repository{
		Reference: ref,
		Client:    conn.client,
	}

	switch {
	case conn.plainHTTPKnown:
		repo.PlainHTTP = conn.plainHTTP
	case plainHTTP:
		repo.PlainHTTP = true
	case dns.IsLocalOrPrivate(ref.Host()):
		resolved, err := ocischeme.From(ctx).UsePlainHTTP(ctx, ref.Host(), ocischeme.ProbeOptions{InsecureSkipTLSVerify: insecureSkipTLSVerify})
		if err != nil {
			return fmt.Errorf("probing scheme for %s: %w", ref.Host(), err)
		}
		repo.PlainHTTP = resolved
	}

	tags, err := orasRegistry.Tags(ctx, repo)
	if err != nil {
		return fmt.Errorf("reading tags for %s: %w", conn.ref, err)
	}

	for _, tag := range tags {
		if omitDigestTags && strings.HasPrefix(tag, "sha256-") {
			continue
		}
		if fullRef {
			fmt.Fprintf(out, "%s/%s:%s\n", ref.Registry, ref.Repository, tag)
		} else {
			fmt.Fprintln(out, tag)
		}
	}
	return nil
}
