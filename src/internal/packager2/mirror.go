// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager2

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/avast/retry-go/v4"

	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/internal/dns"
	"github.com/zarf-dev/zarf/src/internal/git"
	"github.com/zarf-dev/zarf/src/internal/gitea"
	"github.com/zarf-dev/zarf/src/internal/packager/images"
	"github.com/zarf-dev/zarf/src/internal/packager2/layout"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/types"
)

// MirrorOptions are the options for Mirror.
type MirrorOptions struct {
	Cluster         *cluster.Cluster
	PkgLayout       *layout.PackageLayout
	Filter          filters.ComponentFilterStrategy
	RegistryInfo    types.RegistryInfo
	GitInfo         types.GitServerInfo
	NoImageChecksum bool
	Retries         int
	PlainHTTP       bool
	OCIConcurrency  int
}

// Mirror mirrors the package contents to the given registry and git server.
func Mirror(ctx context.Context, opt MirrorOptions) error {
	err := pushImagesToRegistry(ctx, opt.PkgLayout, opt.Filter, opt.RegistryInfo, opt.NoImageChecksum, opt.PlainHTTP, opt.OCIConcurrency)
	if err != nil {
		return err
	}
	err = pushReposToRepository(ctx, opt.Cluster, opt.PkgLayout, opt.Filter, opt.GitInfo, opt.Retries)
	if err != nil {
		return err
	}
	return nil
}

func pushImagesToRegistry(ctx context.Context, pkgLayout *layout.PackageLayout, filter filters.ComponentFilterStrategy, regInfo types.RegistryInfo, noImgChecksum bool, plainHTTP bool, concurrency int) error {
	components, err := filter.Apply(pkgLayout.Pkg)
	if err != nil {
		return err
	}

	refs := []transform.Image{}
	for _, component := range components {
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
		PlainHTTP:       plainHTTP,
		SourceDirectory: pkgLayout.GetImageDir(),
		ImageList:       refs,
		OCIConcurrency:  concurrency,
		NoChecksum:      noImgChecksum,
		Arch:            pkgLayout.Pkg.Build.Architecture,
		RegInfo:         regInfo,
	}
	err = images.Push(ctx, pushConfig)
	if err != nil {
		return fmt.Errorf("failed to mirror images: %w", err)
	}
	return nil
}

func pushReposToRepository(ctx context.Context, c *cluster.Cluster, pkgLayout *layout.PackageLayout, filter filters.ComponentFilterStrategy, gitInfo types.GitServerInfo, retries int) error {
	l := logger.From(ctx)
	components, err := filter.Apply(pkgLayout.Pkg)
	if err != nil {
		return err
	}
	for _, component := range components {
		for _, repoURL := range component.Repos {
			tmpDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
			if err != nil {
				return err
			}
			defer os.RemoveAll(tmpDir)
			reposPath, err := pkgLayout.GetComponentDir(tmpDir, component.Name, layout.RepoComponentDir)
			if err != nil {
				return err
			}
			repository, err := git.Open(reposPath, repoURL)
			if err != nil {
				return err
			}
			err = retry.Do(func() error {
				if !dns.IsServiceURL(gitInfo.Address) {
					message.Infof("Pushing repository %s to server %s", repoURL, gitInfo.Address)
					l.Info("pushing repository to server", "repo", repoURL, "server", gitInfo.Address)
					err = repository.Push(ctx, gitInfo.Address, gitInfo.PushUsername, gitInfo.PushPassword)
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
				giteaClient, err := gitea.NewClient(tunnel.HTTPEndpoint(), gitInfo.PushUsername, gitInfo.PushPassword)
				if err != nil {
					return err
				}
				return tunnel.Wrap(func() error {
					message.Infof("Pushing repository %s to server %s", repoURL, tunnel.HTTPEndpoint())
					l.Info("pushing repository to server", "repo", repoURL, "server", tunnel.HTTPEndpoint())
					err = repository.Push(ctx, tunnel.HTTPEndpoint(), gitInfo.PushUsername, gitInfo.PushPassword)
					if err != nil {
						return err
					}
					// Add the read-only user to this repo
					// TODO: This should not be done here. Or the function name should be changed.
					repoName, err := transform.GitURLtoRepoName(repoURL)
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
	}
	return nil
}
