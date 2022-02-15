package packager

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/images"
	"github.com/defenseunicorns/zarf/cli/internal/k8s"
	"github.com/defenseunicorns/zarf/cli/internal/message"
	"github.com/defenseunicorns/zarf/cli/internal/utils"

	"github.com/distribution/distribution/v3/configuration"
	"github.com/distribution/distribution/v3/registry"
	_ "github.com/distribution/distribution/v3/registry/auth/htpasswd"             // used for embedded registry
	_ "github.com/distribution/distribution/v3/registry/storage/driver/filesystem" // used for embedded registry
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var zarfSeedWriteTarget = fmt.Sprintf("%s:%s", config.IPV4Localhost, config.ZarfSeedWritePort)

func LoadInternalSeedRegistry(path string, seedImages []string) {
	// Launch the embedded registry to load the seed images (r/w mode)
	startSeedRegistry(false)

	// Populate the seed registry
	images.PushToZarfRegistry(path, seedImages, zarfSeedWriteTarget)

	// Now start the registry read-only and wait for exit
	startSeedRegistry(true)

	// Keep this open until an interrupt signal is received
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		os.Exit(0)
	}()

	for {
		runtime.Gosched()
	}
}

func startSeedRegistry(readOnly bool) {
	message.Debugf("packager.startSeedRegistry(%v)", readOnly)
	registryConfig := &configuration.Configuration{}

	if message.GetLogLevel() >= message.DebugLevel {
		registryConfig.Log.Level = "debug"
	} else {
		registryConfig.Log.AccessLog.Disabled = true
		registryConfig.Log.Formatter = "text"
		registryConfig.Log.Level = "error"
	}

	registryConfig.HTTP.DrainTimeout = 0
	registryConfig.HTTP.Secret = utils.RandomString(20)

	fileStorage := configuration.Parameters{
		"rootdirectory": ".zarf-registry",
	}

	if readOnly {
		// Read-only binds to all addresses
		registryConfig.HTTP.Addr = ":" + config.ZarfSeedReadPort
		registryConfig.Storage = configuration.Storage{
			"filesystem": fileStorage,
			"maintenance": configuration.Parameters{
				"readonly": map[interface{}]interface{}{
					"enabled": true,
				},
			},
		}
	} else {
		// Read-write only listen on localhost
		registryConfig.HTTP.Addr = zarfSeedWriteTarget
		registryConfig.Storage = configuration.Storage{
			"filesystem": fileStorage,
		}
	}

	message.Debug(registryConfig)

	embeddedRegistry, err := registry.NewRegistry(context.TODO(), registryConfig)
	if err != nil {
		message.Fatal(err, "Unable to start the embedded registry")
	}

	go func() {
		if err := embeddedRegistry.ListenAndServe(); err != nil {
			message.Fatal(err, "Unable to start the embedded registry")
		}
	}()

}

func preSeedRegistry(tempPath tempPaths) {
	message.Debugf("package.preSeedRegistry(%v)", tempPath)

	var (
		distro string
		err    error
		inject struct {
			command string
			args    []string
		}
	)

	// Attempt to load an existing state prior to init
	state := k8s.LoadZarfState()

	if state.Secret == "" || state.Distro == k8s.DistroIsUnknown {
		// If the state is invalid, assume this is a new cluster
		message.Debug("New cluster, no zarf state found")

		// If the K3s component is being deployed, skip distro detection
		if config.DeployOptions.ApplianceMode {
			distro = k8s.DistroIsK3s
			state.ZarfAppliance = true
		} else {
			// Otherwise, trying to detect the K8s distro type
			distro, err = k8s.DetectDistro()
			if err != nil {
				// This is a basic failure right now but likely could be polished to provide user guidance to resolve
				message.Fatal(err, "Unable to connect to the k8s cluster to verify the distro")
			}
		}

		message.Debugf("Detected K8s distro %v", distro)

		// Defaults
		state.Registry.NodePort = "31999"
		state.Secret = utils.RandomString(120)
		state.Distro = distro
		state.Architecture = config.GetBuildData().Architecture
	}

	switch state.Distro {
	case k8s.DistroIsK3s:
		state.StorageClass = "local-path"
		state.Registry.SeedType = config.ZarfSeedTypeCLIInject
		inject.command = "k3s"
		inject.args = []string{"ctr", "images", "import", tempPath.seedImages}

	case k8s.DistroIsK3d:
		state.StorageClass = "local-path"
		clusterName := getClusterName("k3d")
		state.Registry.SeedType = config.ZarfSeedTypeCLIInject
		inject.command = "k3d"
		inject.args = []string{"images", "import", tempPath.seedImages, "--cluster", clusterName}

	case k8s.DistroIsKind:
		state.StorageClass = "standard"
		// See https://github.com/kubernetes-sigs/kind/blob/v0.11.1/pkg/cluster/internal/kubeconfig/internal/kubeconfig/helpers.go#L24
		clusterName := getClusterName("kind")
		state.Registry.SeedType = config.ZarfSeedTypeCLIInject
		inject.command = "kind"
		inject.args = []string{"load", "image-archive", tempPath.seedImages, "--name", clusterName}

	case k8s.DistroIsDockerDesktop:
		state.StorageClass = "hostpath"
		state.Registry.SeedType = config.ZarfSeedTypeCLIInject
		inject.command = "docker"
		inject.args = []string{"load", "-i", tempPath.seedImages}

	case k8s.DistroIsMicroK8s:
		state.Registry.SeedType = config.ZarfSeedTypeCLIInject
		inject.command = "microk8s"
		inject.args = []string{"ctr", "images", "import", tempPath.seedImages}

	default:
		state.Registry.SeedType = config.ZarfSeedTypeInClusterRegistry
	}

	switch state.Registry.SeedType {
	case config.ZarfSeedTypeCLIInject:
		var (
			output  string
			spinner = message.NewProgressSpinner("Injecting Zarf registry image using %s", inject.command)
		)
		defer spinner.Stop()

		// If this is a seed image injection, attempt to run it and warn if there is an error
		output, err = utils.ExecCommand(false, nil, inject.command, inject.args...)
		message.Debug(output)
		if err != nil {
			spinner.Errorf(err, "Unable to inject the seed image from the %s archive", tempPath.seedImages)
			spinner.Stop()
		} else {
			spinner.Success()
		}

	case config.ZarfSeedTypeInClusterRegistry:
		// Try to create the zarf namesapce
		if _, err := k8s.CreateNamespace("zarf", nil); err != nil {
			message.Fatal(err, "Unabel to create the zarf namespace")
		}

		configData := make(map[string][]byte)
		// @todo generate configdata

		configmap, err := k8s.CreateConfigmap("zarf", "injector-binaries", configData)

		// @todo compute binary shasums


		// Get all the images from the cluster
		images, err := k8s.GetAllImages()
		if err != nil {
			message.Fatal(err, "Unable to generate a list of candidate images to perform the registry injection")
		}

		// Try to create an injector pod using the images in the cluster
		for _, image := range images {
			pod := createInjectionPod(image)
			if pod, err = k8s.CreatePod(pod); err != nil {
				message.Debug(err)
				continue
			} else {
				message.Debug(pod)

				// @todo zarf connect
				// @todo send binaries to net cat
				break
			}
		}
	}

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

	// Push the seed images into to Zarf registry
	images.PushToZarfRegistry(tempPath.seedImages, config.GetSeedImages(), config.ZarfRegistry)
}

func getClusterName(prefix string) string {
	message.Debugf("packager.getClusterName(%v)", prefix)

	if ctx, err := k8s.GetContext(); err != nil {
		message.Error(err, "Unable to auto-inject the registry image into KIND")
		return ""
	} else {
		return strings.Replace(ctx, prefix+"-", "", 1)
	}
}

func createInjectionPod(image string) *corev1.Pod {
	pod := k8s.GeneratePod("injector", "zarf")
	executeMode := int32(0777)

	pod.Labels["app"] = "docker-registry"
	pod.Labels["release"] = "zarf-docker-registry"

	pod.Spec.RestartPolicy = corev1.RestartPolicyNever

	pod.Spec.Containers = []corev1.Container{
		{
			Name:       "injector",
			Image:      image,
			WorkingDir: "/payload",
			Command:    []string{"/zarf-bin/init.sh"},

			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "payload",
					MountPath: "/payload",
				},
				{
					Name:      "bin-volume",
					MountPath: "/zarf-bin",
				},
			},

			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse(".5"),
					corev1.ResourceMemory: resource.MustParse("64Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
			},

			Env: []corev1.EnvVar{
				{
					Name:  "SHA256_BUSYBOX",
					Value: "6a04784d38e8ced6432e683edb07015172acf57bf658d0653b1dc51c43b55643",
				},
				{
					Name:  "SHA256_ZARF",
					Value: "3618dde085ba1c4bbef0f65d5c5864121af1d9fe99098b5e5bca7be1d670d01a",
				},
				{
					Name:  "SHA256_IMAGES",
					Value: "4cc08af8e749ba6fdbc0123847c039ec92c54ed147fc45f7ac6ab505fa44a70e",
				},
				{
					Name:  "USER",
					Value: "root",
				},
			},
		},
	}

	pod.Spec.Volumes = []corev1.Volume{
		// Payload volume just ensures we have a safe empty directory to write the netcat paylaod to
		{
			Name: "payload",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		// Bin volume hosts the busybox binare and init script
		{
			Name: "bin-volume",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "injector-binaries",
					},
					DefaultMode: &executeMode,
				},
			},
		},
	}

	return pod
}
