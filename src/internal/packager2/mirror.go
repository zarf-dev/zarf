// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager2

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/logs"
	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/internal/dns"
	"github.com/zarf-dev/zarf/src/internal/git"
	"github.com/zarf-dev/zarf/src/internal/gitea"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/layout"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/types"
)

// MirrorOptions are the options for Mirror.
type MirrorOptions struct {
	Cluster         *cluster.Cluster
	PackagePaths    layout.PackagePaths
	Filter          filters.ComponentFilterStrategy
	RegistryInfo    types.RegistryInfo
	GitInfo         types.GitServerInfo
	NoImageChecksum bool
	Retries         int
}

// Mirror mirrors the package contents to the given registry and git server.
func Mirror(ctx context.Context, opt MirrorOptions) error {
	err := pushImagesToRegistry(ctx, opt.Cluster, opt.PackagePaths, opt.Filter, opt.RegistryInfo, opt.NoImageChecksum, opt.Retries)
	if err != nil {
		return err
	}
	err = pushReposToRepository(ctx, opt.Cluster, opt.PackagePaths, opt.Filter, opt.GitInfo, opt.Retries)
	if err != nil {
		return err
	}
	return nil
}

func pushImagesToRegistry(ctx context.Context, c *cluster.Cluster, pkgPaths layout.PackagePaths, filter filters.ComponentFilterStrategy, regInfo types.RegistryInfo, noImgChecksum bool, retries int) error {
	logs.Warn.SetOutput(&message.DebugWriter{})
	logs.Progress.SetOutput(&message.DebugWriter{})

	pkg, _, err := pkgPaths.ReadZarfYAML()
	if err != nil {
		return err
	}
	components, err := filter.Apply(pkg)
	if err != nil {
		return err
	}
	pkg.Components = components

	images := map[transform.Image]v1.Image{}
	for _, component := range pkg.Components {
		for _, img := range component.Images {
			ref, err := transform.ParseImageRef(img)
			if err != nil {
				return fmt.Errorf("failed to create ref for image %s: %w", img, err)
			}
			if _, ok := images[ref]; ok {
				continue
			}
			ociImage, err := utils.LoadOCIImage(pkgPaths.Images.Base, ref)
			if err != nil {
				return err
			}
			images[ref] = ociImage
		}
	}
	if len(images) == 0 {
		return nil
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig.InsecureSkipVerify = config.CommonOptions.InsecureSkipTLSVerify
	// TODO (@WSTARR) This is set to match the TLSHandshakeTimeout to potentially mitigate effects of https://github.com/zarf-dev/zarf/issues/1444
	transport.ResponseHeaderTimeout = 10 * time.Second
	transportWithProgressBar := helpers.NewTransport(transport, nil)

	pushOptions := []crane.Option{
		crane.WithPlatform(&v1.Platform{OS: "linux", Architecture: pkg.Build.Architecture}),
		crane.WithTransport(transportWithProgressBar),
		crane.WithAuth(authn.FromConfig(authn.AuthConfig{
			Username: regInfo.PushUsername,
			Password: regInfo.PushPassword,
		})),
		crane.WithUserAgent("zarf"),
		crane.WithNoClobber(true),
		crane.WithJobs(1),
	}
	if config.CommonOptions.InsecureSkipTLSVerify {
		pushOptions = append(pushOptions, crane.Insecure)
	}

	for refInfo, img := range images {
		err = retry.Do(func() error {
			pushImage := func(registryUrl string) error {
				names := []string{}
				if !noImgChecksum {
					offlineNameCRC, err := transform.ImageTransformHost(registryUrl, refInfo.Reference)
					if err != nil {
						return retry.Unrecoverable(err)
					}
					names = append(names, offlineNameCRC)
				}
				offlineName, err := transform.ImageTransformHostWithoutChecksum(registryUrl, refInfo.Reference)
				if err != nil {
					return retry.Unrecoverable(err)
				}
				names = append(names, offlineName)
				for _, name := range names {
					message.Infof("Pushing image %s", name)
					err = crane.Push(img, name, pushOptions...)
					if err != nil {
						return err
					}
				}
				return nil
			}

			if !dns.IsServiceURL(regInfo.Address) {
				return pushImage(regInfo.Address)
			}

			if c == nil {
				return retry.Unrecoverable(errors.New("cannot push to internal OCI registry when cluster is nil"))
			}
			namespace, name, port, err := dns.ParseServiceURL(regInfo.Address)
			if err != nil {
				return err
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
			err = tunnel.Wrap(func() error {
				return pushImage(tunnel.Endpoint())
			})
			if err != nil {
				return err
			}
			return nil
		}, retry.Context(ctx), retry.Attempts(uint(retries)), retry.Delay(500*time.Millisecond))
		if err != nil {
			return err
		}
	}
	return nil
}

func pushReposToRepository(ctx context.Context, c *cluster.Cluster, pkgPaths layout.PackagePaths, filter filters.ComponentFilterStrategy, gitInfo types.GitServerInfo, retries int) error {
	pkg, _, err := pkgPaths.ReadZarfYAML()
	if err != nil {
		return err
	}
	components, err := filter.Apply(pkg)
	if err != nil {
		return err
	}
	pkg.Components = components

	for _, component := range pkg.Components {
		for _, repoURL := range component.Repos {
			repository, err := git.Open(pkgPaths.Components.Dirs[component.Name].Repos, repoURL)
			if err != nil {
				return err
			}
			err = retry.Do(func() error {
				if !dns.IsServiceURL(gitInfo.Address) {
					message.Infof("Pushing repository %s to server %s", repoURL, gitInfo.Address)
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
