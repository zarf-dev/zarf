package packager

import (
	"os"
	"strings"

	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/k8s"
	"github.com/defenseunicorns/zarf/cli/internal/message"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func runInjectionMadness(tempPath tempPaths) {
	message.Debugf("packager.runInjectionMadness(%v)", tempPath)

	var err error
	var images []string
	var envVars []corev1.EnvVar

	// Try to create the zarf namesapce
	if _, err := k8s.CreateNamespace("zarf", nil); err != nil {
		message.Fatal(err, "Unable to create the zarf namespace")
	}

	// Get all the images from the cluster
	if images, err = k8s.GetAllImages(); err != nil {
		message.Fatal(err, "Unable to generate a list of candidate images to perform the registry injection")
	}

	if envVars, err = buildEnvVars(tempPath); err != nil {
		message.Fatal(err, "Unable to build the injection pod environment variables")
	}

	if err = createInjectorConfigmap(tempPath); err != nil {
		message.Fatal(err, "Unable to create the injector configmap")
	}

	if _, err = createService(); err != nil {
		message.Fatal(err, "Unable to create the injector service")
	}

	// Try to create an injector pod using an existing image in the cluster
	for _, image := range images {
		pod := buildInjectionPod(image, envVars)
		if pod, err = k8s.CreatePod(pod); err != nil {
			message.Debug(err)
			continue
		} else {
			message.Debug(pod)

			// Establish the zarf connect tunnel
			tunnel := k8s.NewZarfTunnel()
			tunnel.Connect(k8s.ZarfRegistry, false)
			defer tunnel.Close()

			// @todo send binaries to net cat
			break
		}
	}
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

	// Attempt to create the configmap in the cluster
	if _, err = k8s.CreateConfigmap("zarf", "injector-binaries", configData); err != nil {
		return err
	}

	return nil
}

func createService() (*corev1.Service, error) {
	service := k8s.GenerateService("zarf", "zarf-docker-registry")

	service.Labels["zarf.dev/connect-name"] = "seed-registry"
	service.Labels["app"] = "docker-registry"
	service.Labels["app.kubernetes.io/managed-by"] = "Helm"

	service.Annotations["meta.helm.sh/release-name"] = "zarf-docker-registry"
	service.Annotations["meta.helm.sh/release-namespace"] = "zarf"

	service.Spec.Type = corev1.ServiceTypeNodePort

	service.Spec.Ports = append(service.Spec.Ports, corev1.ServicePort{
		Port:     int32(5000),
		NodePort: int32(31999),
	})

	service.Spec.Selector = map[string]string{
		"app":     "docker-registry",
		"release": "zarf-docker-registry",
	}

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
