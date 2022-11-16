// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains zarf-specific cluster management functions
package cluster

import (
	"fmt"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/k8s"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/pki"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

func (c *Cluster) InitZarfState(tempPath types.TempPaths, initOptions types.ZarfInitOptions) error {
	message.Debugf("package.preSeedRegistry(%#v)", tempPath)

	var (
		clusterArch string
		distro      string
		err         error
	)

	spinner := message.NewProgressSpinner("Gathering cluster information")
	defer spinner.Stop()

	spinner.Updatef("Getting cluster architecture")
	if clusterArch, err = c.Kube.GetArchitecture(); err != nil {
		spinner.Errorf(err, "Unable to validate the cluster system architecture")
	}

	// Attempt to load an existing state prior to init
	// NOTE: We are ignoring the error here because we don't really expect a state to exist yet
	spinner.Updatef("Checking cluster for existing Zarf deployment")
	state, _ := c.LoadZarfState()

	// If the distro isn't populated in the state, assume this is a new cluster
	if state.Distro == "" {
		spinner.Updatef("New cluster, no prior Zarf deployments found")

		// If the K3s component is being deployed, skip distro detection
		if initOptions.ApplianceMode {
			distro = k8s.DistroIsK3s
			state.ZarfAppliance = true
		} else {
			// Otherwise, trying to detect the K8s distro type
			distro, err = c.Kube.DetectDistro()
			if err != nil {
				// This is a basic failure right now but likely could be polished to provide user guidance to resolve
				spinner.Fatalf(err, "Unable to connect to the cluster to verify the distro")
			}
		}

		if distro != k8s.DistroIsUnknown {
			spinner.Updatef("Detected K8s distro %s", distro)
		}

		// Defaults
		state.Distro = distro
		state.Architecture = clusterArch
		state.LoggingSecret = utils.RandomString(config.ZarfGeneratedPasswordLen)

		// Setup zarf agent PKI
		state.AgentTLS = pki.GeneratePKI(config.ZarfAgentHost)

		namespaces, err := c.Kube.GetNamespaces()
		if err != nil {
			return fmt.Errorf("unable to get the Kubernetes namespaces: %w", err)
		}
		// Mark existing namespaces as ignored for the zarf agent to prevent mutating resources we don't own
		for _, namespace := range namespaces.Items {
			spinner.Updatef("Marking existing namespace %s as ignored by Zarf Agent", namespace.Name)
			if namespace.Labels == nil {
				// Ensure label map exists to avoid nil panic
				namespace.Labels = make(map[string]string)
			}
			// This label will tell the Zarf Agent to ignore this namespace
			namespace.Labels["zarf.dev/agent"] = "ignore"
			if _, err = c.Kube.UpdateNamespace(&namespace); err != nil {
				// This is not a hard failure, but we should log it
				message.Errorf(err, "Unable to mark the namespace %s as ignored by Zarf Agent", namespace.Name)
			}
		}

	}

	if clusterArch != state.Architecture {
		return fmt.Errorf("cluster architecture %s does not match the Zarf state architecture %s", clusterArch, state.Architecture)
	}

	switch state.Distro {
	case k8s.DistroIsK3s, k8s.DistroIsK3d:
		state.StorageClass = "local-path"

	case k8s.DistroIsKind, k8s.DistroIsGKE:
		state.StorageClass = "standard"

	case k8s.DistroIsDockerDesktop:
		state.StorageClass = "hostpath"
	}

	if initOptions.StorageClass != "" {
		state.StorageClass = initOptions.StorageClass
	}

	state.GitServer = c.fillInEmptyGitServerValues(initOptions.GitServer)
	state.RegistryInfo = c.fillInEmptyContainerRegistryValues(initOptions.RegistryInfo)

	spinner.Success()

	// Save the state back to K8s
	if err := c.SaveZarfState(state); err != nil {
		return fmt.Errorf("unable to save the Zarf state: %w", err)
	}

	return nil
}

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
	if err := c.Kube.DeleteService(ZarfNamespace, "zarf-injector"); err != nil {
		return err
	}

	// Push the seed images into to Zarf registry
	// seedImage := fmt.Sprintf("%s:%s", config.ZarfSeedImage, config.ZarfSeedTag)
	// err := images.PushToZarfRegistry(tempPath.seedImage, []string{seedImage}, false)

	return nil
}

func (c *Cluster) fillInEmptyContainerRegistryValues(containerRegistry types.RegistryInfo) types.RegistryInfo {
	// Set default url if an external registry was not provided
	if containerRegistry.Address == "" {
		containerRegistry.InternalRegistry = true
		containerRegistry.NodePort = config.ZarfInClusterContainerRegistryNodePort
		containerRegistry.Address = fmt.Sprintf("http://%s:%d", config.IPV4Localhost, containerRegistry.NodePort)
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

// Fill in empty GitServerInfo values with the defaults
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
