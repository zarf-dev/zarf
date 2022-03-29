package packager

import (
	"time"

	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/images"
	"github.com/defenseunicorns/zarf/cli/internal/k8s"
	"github.com/defenseunicorns/zarf/cli/internal/message"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
)

func preSeedRegistry(tempPath tempPaths) {
	message.Debugf("package.preSeedRegistry(%v)", tempPath)

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

	// If the state is invalid, assume this is a new cluster
	if state.Secret == "" {
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
			spinner.Updatef("Detected K8s distro %v", distro)
		}

		// Defaults
		state.NodePort = "31999"
		state.Secret = utils.RandomString(120)
		state.Distro = distro
		state.Architecture = config.GetArch()
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

	// CLI provided overrides that haven't been processed already
	if config.DeployOptions.NodePort != "" {
		state.NodePort = config.DeployOptions.NodePort
	}
	if config.DeployOptions.Secret != "" {
		state.Secret = config.DeployOptions.Secret
	}
	if config.DeployOptions.StorageClass != "" {
		state.StorageClass = config.DeployOptions.StorageClass
	}

	spinner.Success()

	runInjectionMadness(tempPath)

	// Save the state back to K8s
	if err := k8s.SaveZarfState(state); err != nil {
		message.Fatal(err, "Unable to save the Zarf state data back to the cluster")
	}

	// Load state for the rest of the operations
	config.InitState(state)

	registrySecret := config.GetSecret(config.StateRegistryPush)
	// Now that we have what the password will be, we should add the login entry to the system's registry config
	if err := utils.DockerLogin(config.ZarfRegistry, config.ZarfRegistryPushUser, registrySecret); err != nil {
		message.Fatal(err, "Unable to add login credentials for the gitops registry")
	}
}

func postSeedRegistry(tempPath tempPaths) {
	message.Debug("packager.postSeedRegistry(%v)", tempPath)

	// Try to kill the injector pod now
	_ = k8s.DeletePod(k8s.ZarfNamespace, "injector")

	// Remove the configmaps
	labelMatch := map[string]string{"zarf-injector": "payload"}
	_ = k8s.DeleteConfigMapsByLabel(k8s.ZarfNamespace, labelMatch)

	// Remove the injector service
	_ = k8s.DeleteService(k8s.ZarfNamespace, "zarf-injector")

	// Push the seed images into to Zarf registry
	images.PushToZarfRegistry(tempPath.seedImage, []string{config.GetSeedImage()}, config.ZarfRegistry)
}
