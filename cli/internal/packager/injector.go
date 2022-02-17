package packager

import (
	"io/ioutil"
	"net"
	"os"
	"regexp"
	"strings"

	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/k8s"
	"github.com/defenseunicorns/zarf/cli/internal/message"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
	"github.com/mholt/archiver/v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func runInjectionMadness(tempPath tempPaths) {
	message.Debugf("packager.runInjectionMadness(%v)", tempPath)

	spinner := message.NewProgressSpinner("Attempting to bootstrap the seed image into cluster")
	defer spinner.Stop()

	var err error
	var images []string
	var envVars []corev1.EnvVar

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

	spinner.Updatef("Gerating bootsrap payload SHASUMs")
	if envVars, err = buildEnvVars(tempPath); err != nil {
		message.Fatal(err, "Unable to build the injection pod environment variables")
	}

	spinner.Updatef("Creating the injector configmap")
	if err = createInjectorConfigmap(tempPath); err != nil {
		message.Fatal(err, "Unable to create the injector configmap")
	}

	if _, err = createService(); err != nil {
		message.Fatal(err, "Unable to create the injector service")
	}

	// https://regex101.com/r/iCe1iT/1
	zarfImageRegex := regexp.MustCompile(`(?m)^127\.0\.0\.1:31999`)

	// Try to create an injector pod using an existing image in the cluster
	for _, image := range images {
		// Don't try to run against the seed image if this is a secondary zarf init run
		if zarfImageRegex.MatchString(image) {
			continue
		}

		spinner.Updatef("Attempting to bootstrap with the %s", image)

		// Make sure the pod is not there first
		// Sanity buffer
		_ = k8s.DeletePod(k8s.ZarfNamespace, "injector")
		// Update the podspec image path
		pod := buildInjectionPod(image, envVars)
		if pod, err = k8s.CreatePod(pod); err != nil {
			message.Debug(err)
			continue
		} else {
			message.Debug(pod)
			if err = tryInjectorPayloadDeploy(tempPath); err != nil {
				message.Debug(err)
				// On failure just try the next image
				continue
			}
			spinner.Success()
			return
		}
	}

	spinner.Fatalf(nil, "Unable to perform the injection")
}

func tryInjectorPayloadDeploy(tempPath tempPaths) error {
	message.Debugf("packager.tryInjectorPayloadDeploy(%v)", tempPath)
	var err error
	var tarFile []byte
	var tcpConnection *net.TCPConn

	tarPath := tempPath.base + "/payload.tar"
	tarFileList := []string{
		tempPath.injectZarfBinary,
		tempPath.seedImages,
	}

	// Create a tar archive of the injector payload
	if err = archiver.Archive(tarFileList, tarPath); err != nil {
		return err
	}

	// Open the created archive for io.Copy
	if tarFile, err = ioutil.ReadFile(tarPath); err != nil {
		return err
	}

	// Establish the zarf connect tunnel
	tunnel := k8s.NewZarfTunnel()
	tunnel.Connect(k8s.ZarfRegistry, false)
	tunnel.Establish()
	defer tunnel.Close()

	receiver := &net.TCPAddr{
		IP:   net.IPv4(127, 0, 0, 1),
		Port: k8s.PortRegistry,
	}
	// Open the TCP connection for the new tunel
	if tcpConnection, err = net.DialTCP("tcp4", nil, receiver); err != nil {
		return err
	}
	tcpConnection.SetKeepAlive(true)
	tcpConnection.SetWriteBuffer(4096)
	tcpConnection.SetNoDelay(false)

	// Send the payload
	written, err := tcpConnection.Write(tarFile)
	message.Debugf("%d bytes sent", written)
	_ = tcpConnection.Close()
	if err != nil {
		return err
	}

	return nil
}

func createInjectorConfigmap(tempPath tempPaths) error {
	var err error
	configData := make(map[string][]byte)

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
	if _, err = k8s.CreateConfigmap(k8s.ZarfNamespace, "injector-binaries", configData); err != nil {
		return err
	}

	return nil
}

func createService() (*corev1.Service, error) {
	service := k8s.GenerateService(k8s.ZarfNamespace, "zarf-docker-registry")

	service.Labels["app"] = "docker-registry"
	service.Labels["app.kubernetes.io/managed-by"] = "Helm"

	service.Annotations["meta.helm.sh/release-name"] = "zarf-docker-registry"
	service.Annotations["meta.helm.sh/release-namespace"] = k8s.ZarfNamespace

	service.Spec.Type = corev1.ServiceTypeNodePort

	service.Spec.Ports = append(service.Spec.Ports, corev1.ServicePort{
		Port:     int32(5000),
		NodePort: int32(31999),
	})

	service.Spec.Selector = map[string]string{
		"app":     "docker-registry",
		"release": "zarf-docker-registry",
	}

	// Attempt to purse the service silently
	_ = k8s.DeleteService(k8s.ZarfNamespace, "zarf-docker-registry")

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
	if envVars["SHA256_IMAGES"], err = utils.GetSha256Sum(tempPath.seedImages); err != nil {
		return nil, err
	}

	// Add the zarf registry binary shasum env var
	if envVars["SHA256_ZARF"], err = utils.GetSha256Sum(tempPath.injectZarfBinary); err != nil {
		return nil, err
	}

	// Add the seed images list env var
	envVars["SEED_IMAGES"] = strings.Join(config.GetSeedImages(), " ")

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

func buildInjectionPod(image string, envVars []corev1.EnvVar) *corev1.Pod {
	pod := k8s.GeneratePod("injector", k8s.ZarfNamespace)
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
				{Name: "payload", MountPath: "/payload"},
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
