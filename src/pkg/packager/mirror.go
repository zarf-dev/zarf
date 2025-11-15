// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/avast/retry-go/v4"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/internal/dns"
	"github.com/zarf-dev/zarf/src/internal/git"
	"github.com/zarf-dev/zarf/src/internal/gitea"
	"github.com/zarf-dev/zarf/src/internal/packager/images"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/pkg/utils"
)

// ImagePushOptions are optional parameters to push images in a zarf package to a registry
type ImagePushOptions struct {
	Cluster         *cluster.Cluster
	NoImageChecksum bool
	Retries         int
	OCIConcurrency  int
	RemoteOptions
}

// PushImagesToRegistry pushes images in the package layout to the specified registry
func PushImagesToRegistry(ctx context.Context, pkgLayout *layout.PackageLayout, registryInfo state.RegistryInfo, opts ImagePushOptions) error {
	if pkgLayout == nil {
		return fmt.Errorf("package layout is required")
	}
	if registryInfo.Address == "" {
		return fmt.Errorf("registry address must be specified")
	}
	if opts.Retries == 0 {
		opts.Retries = config.ZarfDefaultRetries
	}
	refs := []transform.Image{}
	for _, component := range pkgLayout.Pkg.Components {
		for _, img := range component.Images {
			ref, err := transform.ParseImageRef(img)
			if err != nil {
				return fmt.Errorf("failed to create ref for image %s: %w", img, err)
			}
			refs = append(refs, ref)
		}
	}
	if len(refs) == 0 {
		return nil
	}
	pushConfig := images.PushConfig{
		OCIConcurrency:        opts.OCIConcurrency,
		SourceDirectory:       pkgLayout.GetImageDirPath(),
		RegistryInfo:          registryInfo,
		ImageList:             refs,
		PlainHTTP:             opts.PlainHTTP,
		NoChecksum:            opts.NoImageChecksum,
		Arch:                  pkgLayout.Pkg.Build.Architecture,
		Retries:               opts.Retries,
		InsecureSkipTLSVerify: opts.InsecureSkipTLSVerify,
		Cluster:               opts.Cluster,
	}
	err := images.Push(ctx, pushConfig)
	if err != nil {
		return fmt.Errorf("failed to push images: %w", err)
	}
	return nil
}

// RepoPushOptions are optional parameters to push repos in a zarf package to a Git server
type RepoPushOptions struct {
	Cluster       *cluster.Cluster
	Retries       int
	NoGitChecksum bool
}

// PushReposToRepository pushes Git repositories in the package layout to the Git server
func PushReposToRepository(ctx context.Context, pkgLayout *layout.PackageLayout, gitInfo state.GitServerInfo, opts RepoPushOptions) error {
	if pkgLayout == nil {
		return fmt.Errorf("package layout is required")
	}
	if opts.Retries == 0 {
		opts.Retries = config.ZarfDefaultRetries
	}
	if gitInfo.Address == "" {
		return fmt.Errorf("git server address must be specified")
	}
	for _, component := range pkgLayout.Pkg.Components {
		err := pushComponentReposToRegistry(ctx, component, pkgLayout, gitInfo, opts.Cluster, opts.Retries, opts.NoGitChecksum)
		if err != nil {
			return err
		}
	}
	return nil
}

func pushComponentReposToRegistry(ctx context.Context, component v1alpha1.ZarfComponent,
	pkgLayout *layout.PackageLayout, gitInfo state.GitServerInfo, c *cluster.Cluster, retries int, noGitChecksum bool) (err error) {
	l := logger.From(ctx)
	for _, repoURL := range component.Repos {
		tmpDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
		if err != nil {
			return err
		}
		defer func() {
			err = errors.Join(err, os.RemoveAll(tmpDir))
		}()
		reposPath, err := pkgLayout.GetComponentDir(ctx, tmpDir, component.Name, layout.RepoComponentDir)
		if err != nil {
			return err
		}
		repository, err := git.Open(reposPath, repoURL)
		if err != nil {
			return err
		}
		err = retry.Do(func() error {
			if !dns.IsServiceURL(gitInfo.Address) {
				l.Info("pushing repository to server", "repo", repoURL, "server", gitInfo.Address)
				err = repository.Push(ctx, gitInfo.Address, gitInfo.PushUsername, gitInfo.PushPassword, noGitChecksum)
				if err != nil {
					return err
				}
				return nil
			}

			if c == nil {
				return retry.Unrecoverable(errors.New("cannot push to internal Git server when cluster is nil"))
			}
			namespace, name, port, err := dns.ParseServiceURL(gitInfo.Address)
			if err != nil {
				return retry.Unrecoverable(err)
			}
			tunnel, err := c.NewTunnel(namespace, cluster.SvcResource, name, "", 0, port)
			if err != nil {
				return err
			}
			_, err = tunnel.Connect(ctx)
			if err != nil {
				return err
			}
			defer tunnel.Close()
			// tunnel is create with the default listenAddress - there will only be one endpoint until otherwise supported
			endpoints := tunnel.HTTPEndpoints()
			if len(endpoints) == 0 {
				return errors.New("no tunnel endpoints found")
			}
			giteaClient, err := gitea.NewClient(endpoints[0], gitInfo.PushUsername, gitInfo.PushPassword)
			if err != nil {
				return err
			}
			return tunnel.Wrap(func() error {
				l.Info("pushing repository to server", "repo", repoURL, "server", endpoints[0])
				err = repository.Push(ctx, endpoints[0], gitInfo.PushUsername, gitInfo.PushPassword, noGitChecksum)
				if err != nil {
					return err
				}
				// Add the read-only user to this repo
				// TODO: This should not be done here. Or the function name should be changed.
				repoName, err := transform.GitURLtoRepoName(repoURL, noGitChecksum)
				if err != nil {
					return retry.Unrecoverable(err)
				}
				err = giteaClient.AddReadOnlyUserToRepository(ctx, repoName, gitInfo.PullUsername)
				if err != nil {
					return fmt.Errorf("unable to add the read only user to the repo %s: %w", repoName, err)
				}
				return nil
			})
		}, retry.Context(ctx), retry.Attempts(uint(retries)), retry.Delay(500*time.Millisecond))
		if err != nil {
			return fmt.Errorf("unable to push repo %s to the Git Server: %w", repoURL, err)
		}
	}
	return nil
}
