package packager

import (
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"time"

	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/k8s"
	"github.com/defenseunicorns/zarf/cli/internal/message"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/mholt/archiver/v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func runInjectionMadness(tempPath tempPaths) {
	message.Debugf("packager.runInjectionMadness(%v)", tempPath)

	spinner := message.NewProgressSpinner("Attempting to bootstrap the seed image into the cluster")
	defer spinner.Stop()

	var err error
	var images []string
	var envVars []corev1.EnvVar
	var payloadConfigmaps []string

	// Try to create the zarf namesapce
	spinner.Updatef("Creating the Zarf namespace")
	if _, err := k8s.CreateNamespace(k8s.ZarfNamespace, nil); err != nil {
		message.Fatal(err, "Unable to create the zarf namespace")
	}

	// Get all the images from the cluster
	spinner.Updatef("Getting the list of existing cluster images")
	if images, err = k8s.GetAllImages(); err != nil {
		message.Fatal(err, "Unable to generate a list of candidate images to perform the registry injection")
	}

	spinner.Updatef("Generating bootstrap payload SHASUMs")
	if envVars, err = buildEnvVars(tempPath); err != nil {
		message.Fatal(err, "Unable to build the injection pod environment variables")
	}

	spinner.Updatef("Creating the injector configmap")
	if err = createInjectorConfigmap(tempPath); err != nil {
		message.Fatal(err, "Unable to create the injector configmap")
	}

	if service, err := createService(); err != nil {
		message.Fatal(err, "Unable to create the injector service")
	} else {
		config.ZarfSeedPort = fmt.Sprintf("%d", service.Spec.Ports[0].NodePort)
	}

	if payloadConfigmaps, err = createPayloadConfigmaps(tempPath); err != nil {
		message.Fatal(err, "Unable to generate the injector payload configmaps")
	}

	// https://regex101.com/r/eLS3at/1
	zarfImageRegex := regexp.MustCompile(`(?m)^127\.0\.0\.1:`)

	// Try to create an injector pod using an existing image in the cluster
	for _, image := range images {
		// Don't try to run against the seed image if this is a secondary zarf init run
		if zarfImageRegex.MatchString(image) {
			continue
		}

		spinner.Updatef("Attempting to bootstrap with the %s", image)

		// Make sure the pod is not there first
		_ = k8s.DeletePod(k8s.ZarfNamespace, "injector")
		// Update the podspec image path
		pod := buildInjectionPod(image, envVars, payloadConfigmaps)
		if pod, err = k8s.CreatePod(pod); err != nil {
			message.Debug(err)
			continue
		} else {
			message.Debug(pod)
			// temp padding
			time.Sleep(10 * time.Second)
			spinner.Success()
			return
		}
	}

	spinner.Fatalf(nil, "Unable to perform the injection")
}

func createPayloadConfigmaps(tempPath tempPaths) ([]string, error) {
	message.Debugf("packager.tryInjectorPayloadDeploy(%v)", tempPath)
	var (
		err        error
		tarFile    []byte
		chunks     [][]byte
		configMaps []string
	)

	// Chunk size has to accomdate base64 encoding & etcd 1MB limit
	chunkSize := 1024 * 672
	tarPath := tempPath.base + "/payload.tgz"
	tarFileList := []string{
		tempPath.injectZarfBinary,
		tempPath.seedImage,
	}
	labels := map[string]string{
		"zarf-injector": "payload",
	}

	// Create a tar archive of the injector payload
	if err = archiver.Archive(tarFileList, tarPath); err != nil {
		return configMaps, err
	}

	// Open the created archive for io.Copy
	if tarFile, err = ioutil.ReadFile(tarPath); err != nil {
		return configMaps, err
	}

	for {
		if len(tarFile) == 0 {
			break
		}

		// don't bust slice length
		if len(tarFile) < chunkSize {
			chunkSize = len(tarFile)
		}

		chunks = append(chunks, tarFile[0:chunkSize])
		tarFile = tarFile[chunkSize:]
	}

	for idx, data := range chunks {
		// Create a cat-friendly filename
		fileName := fmt.Sprintf("zarf-payload-%02d\n", idx)

		// Store the binary data
		configData := map[string][]byte{
			fileName: data,
		}

		// Try to delete configmap silently
		_ = k8s.DeleteConfigmap(k8s.ZarfNamespace, fileName)

		// Attempt to create the configmap in the cluster
		if _, err = k8s.ReplaceConfigmap(k8s.ZarfNamespace, fileName, labels, configData); err != nil {
			return configMaps, err
		}

		configMaps = append(configMaps, fileName)
	}

	return configMaps, nil
}

func hasSeedImages(tunnelPort int) bool {
	message.Debugf("packager.hasSeedImages()")

	baseUrl := fmt.Sprintf("%s:%d", config.IPV4Localhost, tunnelPort)
	seedImage := config.GetSeedImage()
	ref := fmt.Sprintf("%s/%s", baseUrl, seedImage)
	timeout := time.After(15 * time.Second)

	for {
		// delay check 3 seconds
		time.Sleep(3 * time.Second)
		select {

		// on timeout abort
		case <-timeout:
			message.Debug("seed image check timed out")
			return false

		// after delay, try running
		default:
			//
			if _, err := crane.Manifest(ref, config.GetCraneOptions()); err != nil {
				message.Debugf("Could not get image ref %s: %w", ref, err)
			} else {
				return true
			}
		}
	}
}

func createInjectorConfigmap(tempPath tempPaths) error {
	var err error
	configData := make(map[string][]byte)
	labels := map[string]string{
		"zarf-injector": "init",
	}

	// Add the init.sh binary data to the configmap
	if configData["init.sh"], err = os.ReadFile(tempPath.injectScript); err != nil {
		return err
	}

	// Add the busybox binary data to the configmap
	if configData["busybox"], err = os.ReadFile(tempPath.injectBinary); err != nil {
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

	// Add the busybox shasum env var
	if envVars["SHA256_BUSYBOX"], err = utils.GetSha256Sum(tempPath.injectBinary); err != nil {
		return nil, err
	}

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

	// Setup the env vars, this one needs more testing but seems to make busybox sad in some images if not set
	encodedEnvVars := []corev1.EnvVar{{Name: "USER", Value: "root"}}
	for name, value := range envVars {
		encodedEnvVars = append(encodedEnvVars, corev1.EnvVar{
			Name:  name,
			Value: value,
		})
	}

	return encodedEnvVars, nil
}

func buildInjectionPod(image string, envVars []corev1.EnvVar, payloadConfigmaps []string) *corev1.Pod {
	pod := k8s.GeneratePod("injector", k8s.ZarfNamespace)
	executeMode := int32(0777)

	pod.Labels["app"] = "zarf-injector"

	pod.Spec.RestartPolicy = corev1.RestartPolicyNever

	pod.Spec.Containers = []corev1.Container{
		{
			Name:       "injector",
			Image:      image,
			WorkingDir: "/payload",
			Command:    []string{"/zarf-bin/init.sh"},

			VolumeMounts: []corev1.VolumeMount{
				{Name: "bin-volume", MountPath: "/zarf-bin"},
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

			Env: envVars,
		},
	}

	pod.Spec.Volumes = []corev1.Volume{
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

	// Iterate over all the payload configmaps and add their mounts
	for _, filename := range payloadConfigmaps {
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

		pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      filename,
			MountPath: fmt.Sprintf("/payload/%s", filename),
			SubPath:   filename,
		})
	}

	return pod
}
