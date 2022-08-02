package packager

import (
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"time"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/k8s"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/mholt/archiver/v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// This needs to be < 672 KBs, but larger sizes tend to face some timeout issues on GKE
// We may need to make this dynamic in the future
var payloadChunkSize = 1024 * 512

func runInjectionMadness(tempPath tempPaths) {
	message.Debugf("packager.runInjectionMadness(%#v)", tempPath)

	spinner := message.NewProgressSpinner("Attempting to bootstrap the seed image into the cluster")
	defer spinner.Success()

	var err error
	var images k8s.ImageNodeMap
	var envVars []corev1.EnvVar
	var payloadConfigmaps []string
	var sha256sum string

	// Try to create the zarf namespace
	spinner.Updatef("Creating the Zarf namespace")
	if _, err := k8s.CreateNamespace(k8s.ZarfNamespace, nil); err != nil {
		spinner.Fatalf(err, "Unable to create the zarf namespace")
	}

	// Get all the images from the cluster
	spinner.Updatef("Getting the list of existing cluster images")
	if images, err = k8s.GetAllImages(); err != nil {
		spinner.Fatalf(err, "Unable to generate a list of candidate images to perform the registry injection")
	}

	spinner.Updatef("Generating bootstrap payload SHASUMs")
	if envVars, err = buildEnvVars(tempPath); err != nil {
		spinner.Fatalf(err, "Unable to build the injection pod environment variables")
	}

	spinner.Updatef("Creating the injector configmap")
	if err = createInjectorConfigmap(tempPath); err != nil {
		spinner.Fatalf(err, "Unable to create the injector configmap")
	}

	spinner.Updatef("Creating the injector service")
	if service, err := createService(); err != nil {
		spinner.Fatalf(err, "Unable to create the injector service")
	} else {
		config.ZarfSeedPort = fmt.Sprintf("%d", service.Spec.Ports[0].NodePort)
	}

	spinner.Updatef("Loading the seed registry configmaps")
	if payloadConfigmaps, sha256sum, err = createPayloadConfigmaps(tempPath, spinner); err != nil {
		spinner.Fatalf(err, "Unable to generate the injector payload configmaps")
	}

	// https://regex101.com/r/eLS3at/1
	zarfImageRegex := regexp.MustCompile(`(?m)^127\.0\.0\.1:`)

	// Try to create an injector pod using an existing image in the cluster
	for image, node := range images {
		// Don't try to run against the seed image if this is a secondary zarf init run
		if zarfImageRegex.MatchString(image) {
			continue
		}

		spinner.Updatef("Attempting to bootstrap with the %s/%s", node, image)

		// Make sure the pod is not there first
		_ = k8s.DeletePod(k8s.ZarfNamespace, "injector")

		// Update the podspec image path and use the first node found
		pod := buildInjectionPod(node[0], image, envVars, payloadConfigmaps, sha256sum)

		// Create the pod in the cluster
		pod, err = k8s.CreatePod(pod)

		// Just debug log the output because failures just result in trying the next image
		message.Debug(pod, err)

		// if no error, try and wait for a seed image to be present, return if successful
		if err == nil && hasSeedImages(spinner) {
			return
		}

		// Otherwise just continue to try next image
	}

	// All images were exhausted and still no happiness
	spinner.Fatalf(nil, "Unable to perform the injection")
}

func createPayloadConfigmaps(tempPath tempPaths, spinner *message.Spinner) ([]string, string, error) {
	message.Debugf("packager.tryInjectorPayloadDeploy(%#v)", tempPath)
	var (
		err        error
		tarFile    []byte
		chunks     [][]byte
		configMaps []string
		sha256sum  string
	)

	// Chunk size has to accomdate base64 encoding & etcd 1MB limit
	tarPath := tempPath.base + "/payload.tgz"
	tarFileList := []string{
		tempPath.injectZarfBinary,
		tempPath.seedImage,
	}
	labels := map[string]string{
		"zarf-injector": "payload",
	}

	spinner.Updatef("Creating the seed registry archive to send to the cluster")
	// Create a tar archive of the injector payload
	if err = archiver.Archive(tarFileList, tarPath); err != nil {
		return configMaps, "", err
	}

	// Open the created archive for io.Copy
	if tarFile, err = ioutil.ReadFile(tarPath); err != nil {
		return configMaps, "", err
	}

	//Calculate the sha256sum of the tarFile before we split it up
	sha256sum = fmt.Sprintf("%x", sha256.Sum256(tarFile))

	spinner.Updatef("Splitting the archive into binary configmaps")
	// Loop over the tarball breaking it into chunks based on the payloadChunkSize
	for {
		if len(tarFile) == 0 {
			break
		}

		// don't bust slice length
		if len(tarFile) < payloadChunkSize {
			payloadChunkSize = len(tarFile)
		}

		chunks = append(chunks, tarFile[0:payloadChunkSize])
		tarFile = tarFile[payloadChunkSize:]
	}

	chunkCount := len(chunks)

	// Loop over all chunks and generate configmaps
	for idx, data := range chunks {
		// Create a cat-friendly filename
		fileName := fmt.Sprintf("zarf-payload-%03d", idx)

		// Store the binary data
		configData := map[string][]byte{
			fileName: data,
		}

		spinner.Updatef("Adding archive binary configmap %d of %d to the cluster", idx+1, chunkCount)

		// Attempt to create the configmap in the cluster
		if _, err = k8s.ReplaceConfigmap(k8s.ZarfNamespace, fileName, labels, configData); err != nil {
			return configMaps, "", err
		}

		// Add the configmap to the configmaps slice for later usage in the pod
		configMaps = append(configMaps, fileName)

		// Give the control plane a 250ms buffer between each configmap
		time.Sleep(250 * time.Millisecond)
	}

	return configMaps, sha256sum, nil
}

func hasSeedImages(spinner *message.Spinner) bool {
	message.Debugf("packager.hasSeedImages()")

	// Establish the zarf connect tunnel
	tunnel := k8s.NewZarfTunnel()
	tunnel.AddSpinner(spinner)
	tunnel.Connect(k8s.ZarfInjector, false)
	defer tunnel.Close()

	baseUrl := tunnel.Endpoint()
	seedImage := config.GetSeedImage()
	ref := fmt.Sprintf("%s/%s", baseUrl, seedImage)
	timeout := time.After(20 * time.Second)

	for {
		// Delay check for one second
		time.Sleep(1 * time.Second)
		select {

		// On timeout abort
		case <-timeout:
			message.Debug("seed image check timed out")
			return false

		// After delay, try running
		default:
			// Check for the existence of the image in the injection pod registry, on error continue
			if _, err := crane.Manifest(ref, config.GetCraneOptions()...); err != nil {
				message.Debugf("Could not get image ref %s: %#v", ref, err)
			} else {
				// If not error, return true, there image is present
				return true
			}
		}
	}
}

func createInjectorConfigmap(tempPath tempPaths) error {
	var err error
	configData := make(map[string][]byte)
	labels := map[string]string{
		"zarf-injector": "payload",
	}

	// Add the injector binary data to the configmap
	if configData["zarf-injector"], err = os.ReadFile(tempPath.injectBinary); err != nil {
		return err
	}

	// Try to delete configmap silently
	_ = k8s.DeleteConfigmap(k8s.ZarfNamespace, "injector-binaries")

	// Attempt to create the configmap in the cluster
	if _, err = k8s.CreateConfigmap(k8s.ZarfNamespace, "injector-binaries", labels, configData); err != nil {
		return err
	}

	return nil
}

func createService() (*corev1.Service, error) {
	service := k8s.GenerateService(k8s.ZarfNamespace, "zarf-injector")

	service.Spec.Type = corev1.ServiceTypeNodePort
	service.Spec.Ports = append(service.Spec.Ports, corev1.ServicePort{
		Port: int32(5000),
	})
	service.Spec.Selector = map[string]string{
		"app": "zarf-injector",
	}

	// Attempt to purse the service silently
	_ = k8s.DeleteService(k8s.ZarfNamespace, "zarf-injector")

	return k8s.CreateService(service)
}

func buildEnvVars(tempPath tempPaths) ([]corev1.EnvVar, error) {
	var err error
	envVars := make(map[string]string)

	// Add the seed images shasum env var
	if envVars["SHA256_IMAGE"], err = utils.GetSha256Sum(tempPath.seedImage); err != nil {
		return nil, err
	}

	// Add the zarf registry binary shasum env var
	if envVars["SHA256_ZARF"], err = utils.GetSha256Sum(tempPath.injectZarfBinary); err != nil {
		return nil, err
	}

	// Add the seed images list env var
	envVars["SEED_IMAGE"] = config.GetSeedImage()

	// Setup the env vars
	encodedEnvVars := []corev1.EnvVar{}
	for name, value := range envVars {
		encodedEnvVars = append(encodedEnvVars, corev1.EnvVar{
			Name:  name,
			Value: value,
		})
	}

	return encodedEnvVars, nil
}

// buildInjectionPod return a pod for injection with the appropriate containers to perform the injection
func buildInjectionPod(node, image string, envVars []corev1.EnvVar, payloadConfigmaps []string, payloadShasum string) *corev1.Pod {
	pod := k8s.GeneratePod("injector", k8s.ZarfNamespace)
	executeMode := int32(0777)
	seedImage := config.GetSeedImage()

	pod.Labels["app"] = "zarf-injector"

	// Bind the pod to the node the image was found on
	pod.Spec.NodeSelector = map[string]string{"kubernetes.io/hostname": node}

	// Do not try to restart the pod as it will be deleted/re-created instead
	pod.Spec.RestartPolicy = corev1.RestartPolicyNever

	// Init container used to combine and decompress the split tarball into the stage2 directory for use in the main container
	pod.Spec.InitContainers = []corev1.Container{
		{
			Name: "init-injector",
			// An existing image already present on the cluster
			Image: image,
			// PullIfNotPresent because some distros provide a way (even in airgap) to pull images from local or direct-connected registries
			ImagePullPolicy: corev1.PullIfNotPresent,
			// This directory is filled via the configmap injections
			WorkingDir: "/zarf-stage1",
			Command:    []string{"/zarf-stage1/zarf-injector", payloadShasum},

			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "stage1",
					MountPath: "/zarf-stage1/zarf-injector",
					SubPath:   "zarf-injector",
				},
				{
					Name:      "stage2",
					MountPath: "/zarf-stage2",
				},
			},

			// Keep resources as light as possible as we aren't actually running the container's other binaries
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse(".5"),
					corev1.ResourceMemory: resource.MustParse("64Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("2"),
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
			},

			Env: envVars,
		},
	}

	// Container definition for the injector pod
	pod.Spec.Containers = []corev1.Container{
		{
			Name: "injector",
			// An existing image already present on the cluster
			Image: image,
			// PullIfNotPresent because some distros provide a way (even in airgap) to pull images from local or direct-connected registries
			ImagePullPolicy: corev1.PullIfNotPresent,
			// This directory's contents come from the init container output
			WorkingDir: "/zarf-stage2",
			Command: []string{
				"/zarf-stage2/zarf-registry",
				"/zarf-stage2/seed-image.tar",
				seedImage,
				utils.SwapHost(seedImage, "127.0.0.1:5001"),
			},

			// Shared mount between the init and regular containers
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "stage2",
					MountPath: "/zarf-stage2",
				},
			},

			// Keep resources as light as possible as we aren't actually running the container's other binaries
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

			Env: envVars,
		},
	}

	pod.Spec.Volumes = []corev1.Volume{
		// Stage1 contains the rust binary and collection of configmaps from the tarball (go binary + seed image)
		{
			Name: "stage1",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "injector-binaries",
					},
					DefaultMode: &executeMode,
				},
			},
		},
		// Stage2 is an emtpy directory shared between the containers
		{
			Name: "stage2",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	// Iterate over all the payload configmaps and add their mounts
	for _, filename := range payloadConfigmaps {
		// Create the configmap volume from the given filename
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: filename,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: filename,
					},
				},
			},
		})

		// Create the volume mount to place the new volume in the stage1 directory
		pod.Spec.InitContainers[0].VolumeMounts = append(pod.Spec.InitContainers[0].VolumeMounts, corev1.VolumeMount{
			Name:      filename,
			MountPath: fmt.Sprintf("/zarf-stage1/%s", filename),
			SubPath:   filename,
		})
	}

	return pod
}
