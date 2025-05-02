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
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/types"
)

// MirrorOptions are the options for Mirror.
type MirrorOptions struct {
	Cluster               *cluster.Cluster
	PkgLayout             *layout.PackageLayout
	RegistryInfo          types.RegistryInfo
	GitInfo               types.GitServerInfo
	NoImageChecksum       bool
	Retries               int
	PlainHTTP             bool
	OCIConcurrency        int
	InsecureSkipTLSVerify bool
}

// Mirror mirrors the package contents to the given registry and git server.
func Mirror(ctx context.Context, opt MirrorOptions) error {
	err := pushImagesToRegistry(ctx, opt.PkgLayout, opt.RegistryInfo, opt.NoImageChecksum, opt.PlainHTTP, opt.OCIConcurrency, opt.Retries, opt.InsecureSkipTLSVerify)
	if err != nil {
		return err
	}
	err = pushReposToRepository(ctx, opt.Cluster, opt.PkgLayout, opt.GitInfo, opt.Retries)
	if err != nil {
		return err
	}
	return nil
}

func pushImagesToRegistry(ctx context.Context, pkgLayout *layout.PackageLayout, registryInfo types.RegistryInfo, noImgChecksum bool, plainHTTP bool, concurrency int, retries int, insecure bool) error {
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
		OCIConcurrency:        concurrency,
		SourceDirectory:       pkgLayout.GetImageDir(),
		RegistryInfo:          registryInfo,
		ImageList:             refs,
		PlainHTTP:             plainHTTP,
		NoChecksum:            noImgChecksum,
		Arch:                  pkgLayout.Pkg.Build.Architecture,
		Retries:               retries,
		InsecureSkipTLSVerify: insecure,
	}
	err := images.Push(ctx, pushConfig)
	if err != nil {
		return fmt.Errorf("failed to push images: %w", err)
	}
	return nil
}

func pushReposToRepository(ctx context.Context, c *cluster.Cluster, pkgLayout *layout.PackageLayout, gitInfo types.GitServerInfo, retries int) error {
	l := logger.From(ctx)
	for _, component := range pkgLayout.Pkg.Components {
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
