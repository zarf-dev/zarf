package packager

import (
	"fmt"
	"time"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/images"
	"github.com/defenseunicorns/zarf/src/internal/k8s"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/pki"
	"github.com/defenseunicorns/zarf/src/internal/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

/*
	preSeedRegistry does:
	 - waits for the cluster to be healthy
	 - gets the cluster architecture to use later to compare against the arch of the init package
	 - attempts to load an existing zarf secret (when would this ever be here if we're only running this on init?)
	 - Gets the cluster arch to use later to set the `state.StorageClass`
	 - hardcodes default state values ()
	 - gets a list of the current namespaces in the cluster and adds a label so that `zarf-agent` will ignore that namespace
	 - sets cli flag overrides to state values
	 - runs the injection maddness ()
	 - Saves the state..
*/
func seedZarfState(tempPath tempPaths) {
	message.Debugf("package.preSeedRegistry(%#v)", tempPath)

	var (
		clusterArch string
		distro      string
		err         error
	)

	spinner := message.NewProgressSpinner("Gathering cluster information")
	defer spinner.Stop()

	if err := k8s.WaitForHealthyCluster(5 * time.Minute); err != nil {
		spinner.Fatalf(err, "The cluster we are using never reported 'healthy'")
	}

	spinner.Updatef("Getting cluster architecture")
	if clusterArch, err = k8s.GetArchitecture(); err != nil {
		spinner.Errorf(err, "Unable to validate the cluster system architecture")
	}

	// Attempt to load an existing state prior to init
	spinner.Updatef("Checking cluster for existing Zarf deployment")
	state := k8s.LoadZarfState()

	// If the distro isn't populated in the state, assume this is a new cluster
	if state.Distro == "" {
		spinner.Updatef("New cluster, no prior Zarf deployments found")

		// If the K3s component is being deployed, skip distro detection
		if config.DeployOptions.ApplianceMode {
			distro = k8s.DistroIsK3s
			state.ZarfAppliance = true
		} else {
			// Otherwise, trying to detect the K8s distro type
			distro, err = k8s.DetectDistro()
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
		state.Architecture = config.GetArch()
		state.LoggingPassword = utils.RandomString(24)

		// Setup zarf agent PKI
		state.AgentTLS = pki.GeneratePKI(config.ZarfAgentHost)

		namespaces, err := k8s.GetNamespaces()
		if err != nil {
			message.Fatalf(err, "Unable to get k8s namespaces")
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
			if _, err = k8s.UpdateNamespace(&namespace); err != nil {
				// This is not a hard failure, but we should log it
				message.Errorf(err, "Unable to mark the namespace %s as ignored by Zarf Agent", namespace.Name)
			}
		}

	}

	if clusterArch != state.Architecture {
		spinner.Fatalf(nil, "The current Zarf package architecture %s does not match the cluster architecture %s", state.Architecture, clusterArch)
	}

	switch state.Distro {
	case k8s.DistroIsK3s, k8s.DistroIsK3d:
		state.StorageClass = "local-path"

	case k8s.DistroIsKind, k8s.DistroIsGKE:
		state.StorageClass = "standard"

	case k8s.DistroIsDockerDesktop:
		state.StorageClass = "hostpath"
	}

	if config.InitOptions.StorageClass != "" {
		state.StorageClass = config.InitOptions.StorageClass
	}

	state.GitServer = fillInEmptyGitServerValues(config.InitOptions.GitServer)

	if config.InitOptions.ContainerRegistryInfo.URL == "" {
		state.ContainerRegistryInfo.PushUser = config.ZarfRegistryPushUser
		state.ContainerRegistryInfo.PushPassword = utils.RandomString(48)
		state.ContainerRegistryInfo.PullUser = config.ZarfRegistryPullUser
		state.ContainerRegistryInfo.PullPassword = utils.RandomString(48)
		state.ContainerRegistryInfo.InternalRegistry = true
		state.ContainerRegistryInfo.NodePort = config.InitOptions.ContainerRegistryInfo.NodePort
		state.ContainerRegistryInfo.URL = fmt.Sprintf("http://%s:%d", config.IPV4Localhost, state.ContainerRegistryInfo.NodePort)
	} else {
		state.ContainerRegistryInfo = config.InitOptions.ContainerRegistryInfo

		// For external registries, the pull-user is the same as the push-user
		state.ContainerRegistryInfo.PullUser = state.ContainerRegistryInfo.PushUser
		state.ContainerRegistryInfo.PullPassword = state.ContainerRegistryInfo.PushPassword
	}

	spinner.Success()

	// Save the state back to K8s
	if err := k8s.SaveZarfState(state); err != nil {
		message.Fatal(err, "Unable to save the Zarf state data back to the cluster")
	}

	// Load state for the rest of the operations
	config.InitState(state)
}

func postSeedRegistry(tempPath tempPaths) {
	message.Debug("packager.postSeedRegistry(%#v)", tempPath)

	// Try to kill the injector pod now
	_ = k8s.DeletePod(k8s.ZarfNamespace, "injector")

	// Remove the configmaps
	labelMatch := map[string]string{"zarf-injector": "payload"}
	_ = k8s.DeleteConfigMapsByLabel(k8s.ZarfNamespace, labelMatch)

	// Remove the injector service
	_ = k8s.DeleteService(k8s.ZarfNamespace, "zarf-injector")

	// Push the seed images into to Zarf registry
	images.PushToZarfRegistry(tempPath.seedImage, []string{config.GetSeedImage()})
}

// Fill in empty GitServerInfo values with the defaults
func fillInEmptyGitServerValues(gitServer types.GitServerInfo) types.GitServerInfo {
	// Set default svc url if necessary
	if gitServer.Address == "" {
		gitServer.Address = config.ZarfInClusterGitServiceURL
		gitServer.Port = config.ZarfInClusterGitServicePort
		gitServer.InternalServer = true
	}

	// Set default push username and auto-generate a password
	if gitServer.PushUsername == "" {
		gitServer.PushUsername = config.ZarfGitPushUser
	}
	if gitServer.PushPassword == "" {
		gitServer.PushPassword = utils.RandomString(24)
	}

	// Set read-user information if using an internal repository, otherwise copy from the push-user
	if gitServer.ReadUsername == "" {
		if gitServer.InternalServer {
			gitServer.ReadUsername = config.ZarfGitReadUser
		} else {
			gitServer.ReadUsername = gitServer.PushUsername
		}
	}
	if gitServer.ReadPassword == "" {
		if gitServer.InternalServer {
			gitServer.ReadPassword = utils.RandomString(24)
		} else {
			gitServer.ReadPassword = gitServer.PushPassword
		}
	}

	return gitServer
}
