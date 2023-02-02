// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"fmt"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

// PostSeedRegistry handles cleanup once the seed registry is up.
func (c *Cluster) PostSeedRegistry(tempPath types.TempPaths) error {
	message.Debugf("cluster.PostSeedRegistry(%#v)", tempPath)

	// Try to kill the injector pod now
	if err := c.Kube.DeletePod(ZarfNamespace, "injector"); err != nil {
		return err
	}

	// Remove the configmaps
	labelMatch := map[string]string{"zarf-injector": "payload"}
	if err := c.Kube.DeleteConfigMapsByLabel(ZarfNamespace, labelMatch); err != nil {
		return err
	}

	// Remove the injector service
	return c.Kube.DeleteService(ZarfNamespace, "zarf-injector")
}

func (c *Cluster) fillInEmptyContainerRegistryValues(containerRegistry types.RegistryInfo) types.RegistryInfo {
	// Set default NodePort if none was provided
	if containerRegistry.NodePort == 0 {
		containerRegistry.NodePort = config.ZarfInClusterContainerRegistryNodePort
	}

	// Set default url if an external registry was not provided
	if containerRegistry.Address == "" {
		containerRegistry.InternalRegistry = true
		containerRegistry.Address = fmt.Sprintf("%s:%d", config.IPV4Localhost, containerRegistry.NodePort)
	}

	// Generate a push-user password if not provided by init flag
	if containerRegistry.PushPassword == "" {
		containerRegistry.PushPassword = utils.RandomString(config.ZarfGeneratedPasswordLen)
	}

	// Set pull-username if not provided by init flag
	if containerRegistry.PullUsername == "" {
		if containerRegistry.InternalRegistry {
			containerRegistry.PullUsername = config.ZarfRegistryPullUser
		} else {
			// If this is an external registry and a pull-user wasn't provided, use the same credentials as the push user
			containerRegistry.PullUsername = containerRegistry.PushUsername
		}
	}
	if containerRegistry.PullPassword == "" {
		if containerRegistry.InternalRegistry {
			containerRegistry.PullPassword = utils.RandomString(config.ZarfGeneratedPasswordLen)
		} else {
			// If this is an external registry and a pull-user wasn't provided, use the same credentials as the push user
			containerRegistry.PullPassword = containerRegistry.PushPassword
		}
	}

	if containerRegistry.Secret == "" {
		containerRegistry.Secret = utils.RandomString(config.ZarfGeneratedSecretLen)
	}

	return containerRegistry
}

// Fill in empty GitServerInfo values with the defaults.
func (c *Cluster) fillInEmptyGitServerValues(gitServer types.GitServerInfo) types.GitServerInfo {
	// Set default svc url if an external repository was not provided
	if gitServer.Address == "" {
		gitServer.Address = config.ZarfInClusterGitServiceURL
		gitServer.InternalServer = true
	}

	// Generate a push-user password if not provided by init flag
	if gitServer.PushPassword == "" {
		gitServer.PushPassword = utils.RandomString(config.ZarfGeneratedPasswordLen)
	}

	// Set read-user information if using an internal repository, otherwise copy from the push-user
	if gitServer.PullUsername == "" {
		if gitServer.InternalServer {
			gitServer.PullUsername = config.ZarfGitReadUser
		} else {
			gitServer.PullUsername = gitServer.PushUsername
		}
	}
	if gitServer.PullPassword == "" {
		if gitServer.InternalServer {
			gitServer.PullPassword = utils.RandomString(config.ZarfGeneratedPasswordLen)
		} else {
			gitServer.PullPassword = gitServer.PushPassword
		}
	}

	return gitServer
}
