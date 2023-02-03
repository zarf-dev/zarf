// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/k8s"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/mholt/archiver/v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// The chunk size for the tarball chunks.
var payloadChunkSize = 1024 * 768

// StartInjectionMadness initializes a Zarf injection into the cluster.
func (c *Cluster) StartInjectionMadness(tempPath types.TempPaths) {
	message.Debugf("packager.runInjectionMadness(%#v)", tempPath)

	spinner := message.NewProgressSpinner("Attempting to bootstrap the seed image into the cluster")
	defer spinner.Success()

	var err error
	var images k8s.ImageNodeMap
	var payloadConfigmaps []string
	var sha256sum string

	// Get all the images from the cluster
	spinner.Updatef("Getting the list of existing cluster images")
	if images, err = c.Kube.GetAllImages(); err != nil {
		spinner.Fatalf(err, "Unable to generate a list of candidate images to perform the registry injection")
	}

	spinner.Updatef("Creating the injector configmap")
	if err = c.createInjectorConfigmap(tempPath); err != nil {
		spinner.Fatalf(err, "Unable to create the injector configmap")
	}

	spinner.Updatef("Creating the injector service")
	if service, err := c.createService(); err != nil {
		spinner.Fatalf(err, "Unable to create the injector service")
	} else {
		config.ZarfSeedPort = fmt.Sprintf("%d", service.Spec.Ports[0].NodePort)
	}

	spinner.Updatef("Loading the seed registry configmaps")
	if payloadConfigmaps, sha256sum, err = c.createPayloadConfigmaps(tempPath, spinner); err != nil {
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
		_ = c.Kube.DeletePod(ZarfNamespace, "injector")

		// Update the podspec image path and use the first node found
		pod, err := c.buildInjectionPod(node[0], image, payloadConfigmaps, sha256sum)
		if err != nil {
			// Just debug log the output because failures just result in trying the next image
			message.Debug(err)
			continue
		}

		// Create the pod in the cluster
		pod, err = c.Kube.CreatePod(pod)
		if err != nil {
			// Just debug log the output because failures just result in trying the next image
			message.Debug(pod, err)
			continue
		}

		// if no error, try and wait for a seed image to be present, return if successful
		if c.injectorIsReady(spinner) {
			return
		}

		// Otherwise just continue to try next image
	}

	// All images were exhausted and still no happiness
	spinner.Fatalf(nil, "Unable to perform the injection")
}

// StopInjectionMadness handles cleanup once the seed registry is up.
func (c *Cluster) StopInjectionMadness() error {
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

func (c *Cluster) createPayloadConfigmaps(tempPath types.TempPaths, spinner *message.Spinner) ([]string, string, error) {
	message.Debugf("packager.tryInjectorPayloadDeploy(%#v)", tempPath)
	var configMaps []string

	// Chunk size has to accommodate base64 encoding & etcd 1MB limit
	tarPath := filepath.Join(tempPath.Base, "payload.tgz")
	tarFileList, err := filepath.Glob(filepath.Join(tempPath.Base, "seed-image", "*"))
	if err != nil {
		return configMaps, "", err
	}

	spinner.Updatef("Creating the seed registry archive to send to the cluster")
	// Create a tar archive of the injector payload
	if err := archiver.Archive(tarFileList, tarPath); err != nil {
		return configMaps, "", err
	}

	chunks, sha256sum, err := utils.SplitFile(tarPath, payloadChunkSize)
	if err != nil {
		return configMaps, "", err
	}

	spinner.Updatef("Splitting the archive into binary configmaps")

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
		if _, err = c.Kube.ReplaceConfigmap(ZarfNamespace, fileName, configData); err != nil {
			return configMaps, "", err
		}

		// Add the configmap to the configmaps slice for later usage in the pod
		configMaps = append(configMaps, fileName)

		// Give the control plane a 250ms buffer between each configmap
		time.Sleep(250 * time.Millisecond)
	}

	return configMaps, sha256sum, nil
}

// Test for pod readiness and seed image presence.
func (c *Cluster) injectorIsReady(spinner *message.Spinner) bool {
	message.Debugf("packager.injectorIsReady()")

	// Establish the zarf connect tunnel
	tunnel, err := NewZarfTunnel()
	if err != nil {
		message.Warnf("Unable to establish a tunnel to look for seed images: %#v", err)
		return false
	}
	tunnel.AddSpinner(spinner)
	tunnel.Connect(ZarfInjector, false)
	defer tunnel.Close()

	spinner.Updatef("Testing the injector for seed image availability")

	seedRegistry := fmt.Sprintf("%s/v2/library/%s/manifests/%s", tunnel.HTTPEndpoint(), config.ZarfSeedImage, config.ZarfSeedTag)
	if resp, err := http.Get(seedRegistry); err != nil || resp.StatusCode != 200 {
		// Just debug log the output because failures just result in trying the next image
		message.Debug(resp, err)
		return false
	}

	spinner.Updatef("Seed image found, injector is ready")
	return true
}

func (c *Cluster) createInjectorConfigmap(tempPath types.TempPaths) error {
	var err error
	configData := make(map[string][]byte)

	// Add the injector binary data to the configmap
	if configData["zarf-injector"], err = os.ReadFile(tempPath.InjectBinary); err != nil {
		return err
	}

	// Try to delete configmap silently
	_ = c.Kube.DeleteConfigmap(ZarfNamespace, "rust-binary")

	// Attempt to create the configmap in the cluster
	if _, err = c.Kube.CreateConfigmap(ZarfNamespace, "rust-binary", configData); err != nil {
		return err
	}

	return nil
}

func (c *Cluster) createService() (*corev1.Service, error) {
	service := c.Kube.GenerateService(ZarfNamespace, "zarf-injector")

	service.Spec.Type = corev1.ServiceTypeNodePort
	service.Spec.Ports = append(service.Spec.Ports, corev1.ServicePort{
		Port: int32(5000),
	})
	service.Spec.Selector = map[string]string{
		"app": "zarf-injector",
	}

	// Attempt to purse the service silently
	_ = c.Kube.DeleteService(ZarfNamespace, "zarf-injector")

	return c.Kube.CreateService(service)
}

// buildInjectionPod return a pod for injection with the appropriate containers to perform the injection.
func (c *Cluster) buildInjectionPod(node, image string, payloadConfigmaps []string, payloadShasum string) (*corev1.Pod, error) {
	pod := c.Kube.GeneratePod("injector", ZarfNamespace)
	executeMode := int32(0777)

	pod.Labels["app"] = "zarf-injector"

	// Ensure zarf agent doesn't break the injector on future runs
	pod.Labels[agentLabel] = "ignore"

	// Bind the pod to the node the image was found on
	pod.Spec.NodeSelector = map[string]string{"kubernetes.io/hostname": node}

	// Do not try to restart the pod as it will be deleted/re-created instead
	pod.Spec.RestartPolicy = corev1.RestartPolicyNever

	pod.Spec.Containers = []corev1.Container{
		{
			Name: "injector",

			// An existing image already present on the cluster
			Image: image,

			// PullIfNotPresent because some distros provide a way (even in airgap) to pull images from local or direct-connected registries
			ImagePullPolicy: corev1.PullIfNotPresent,

			// This directory's contents come from the init container output
			WorkingDir: "/zarf-init",

			// Call the injector with shasum of the tarball
			Command: []string{"/zarf-init/zarf-injector", payloadShasum},

			// Shared mount between the init and regular containers
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "init",
					MountPath: "/zarf-init/zarf-injector",
					SubPath:   "zarf-injector",
				},
				{
					Name:      "seed",
					MountPath: "/zarf-seed",
				},
			},

			// Readiness probe to optimize the pod startup time
			ReadinessProbe: &corev1.Probe{
				PeriodSeconds:    2,
				SuccessThreshold: 1,
				FailureThreshold: 10,
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/v2/",               // path to health check
						Port: intstr.FromInt(5000), // port to health check
					},
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
		},
	}

	pod.Spec.Volumes = []corev1.Volume{
		// Contains the rust binary and collection of configmaps from the tarball (seed image).
		{
			Name: "init",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "rust-binary",
					},
					DefaultMode: &executeMode,
				},
			},
		},
		// Empty directory to hold the seed image (new dir to avoid permission issues)
		{
			Name: "seed",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	// Iterate over all the payload configmaps and add their mounts.
	for _, filename := range payloadConfigmaps {
		// Create the configmap volume from the given filename.
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

		// Create the volume mount to place the new volume in the working directory
		pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      filename,
			MountPath: fmt.Sprintf("/zarf-init/%s", filename),
			SubPath:   filename,
		})
	}

	return pod, nil
}
