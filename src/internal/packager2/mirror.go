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

// ImagePushOptions are optional parameters to push images in a zarf package to a registry
type ImagePushOptions struct {
	Cluster         *cluster.Cluster
	NoImageChecksum bool
	Retries         int
	OCIConcurrency  int
	RemoteOptions
}

// PushImagesToRegistry pushes images in the package layout to the specified registry
func PushImagesToRegistry(ctx context.Context, pkgLayout *layout.PackageLayout, registryInfo types.RegistryInfo, opts ImagePushOptions) error {
	if pkgLayout == nil {
		return fmt.Errorf("package layout is required")
	}
	if registryInfo.Address == "" {
		return fmt.Errorf("registry address must be specified")
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

// RepoPushOptions are optional parameters to push images in a zarf package to a registry
type RepoPushOptions struct {
	Cluster *cluster.Cluster
	Retries int
}

// PushReposToRepository pushes Git repositories in the package layout to the registry
func PushReposToRepository(ctx context.Context, pkgLayout *layout.PackageLayout, gitInfo types.GitServerInfo, opts RepoPushOptions) (err error) {
	if pkgLayout == nil {
		return fmt.Errorf("package layout is required")
	}
	if gitInfo.Address == "" {
		return fmt.Errorf("git server address must be specified")
	}
	l := logger.From(ctx)
	for _, component := range pkgLayout.Pkg.Components {
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
					err = repository.Push(ctx, gitInfo.Address, gitInfo.PushUsername, gitInfo.PushPassword)
					if err != nil {
						return err
					}
					return nil
				}

				if opts.Cluster == nil {
					return retry.Unrecoverable(errors.New("cannot push to internal Git server when cluster is nil"))
				}
				namespace, name, port, err := dns.ParseServiceURL(gitInfo.Address)
				if err != nil {
					return retry.Unrecoverable(err)
				}
				tunnel, err := opts.Cluster.NewTunnel(namespace, cluster.SvcResource, name, "", 0, port)
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
			}, retry.Context(ctx), retry.Attempts(uint(opts.Retries)), retry.Delay(500*time.Millisecond))
			if err != nil {
				return fmt.Errorf("unable to push repo %s to the Git Server: %w", repoURL, err)
			}
		}
	}
	return nil
}
