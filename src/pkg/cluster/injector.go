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

	"github.com/defenseunicorns/pkg/helpers"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/k8s"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/mholt/archiver/v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// The chunk size for the tarball chunks.
var payloadChunkSize = 1024 * 768

var (
	injectorRequestedCPU    = resource.MustParse(".5")
	injectorRequestedMemory = resource.MustParse("64Mi")
	injectorLimitCPU        = resource.MustParse("1")
	injectorLimitMemory     = resource.MustParse("256Mi")
)

// imageNodeMap is a map of image/node pairs.
type imageNodeMap map[string][]string

// StartInjectionMadness initializes a Zarf injection into the cluster.
func (c *Cluster) StartInjectionMadness(tmpDir string, imagesDir string, injectorSeedSrcs []string) {
	spinner := message.NewProgressSpinner("Attempting to bootstrap the seed image into the cluster")
	defer spinner.Stop()

	tmp := layout.InjectionMadnessPaths{
		SeedImagesDir: filepath.Join(tmpDir, "seed-images"),
		// should already exist
		InjectionBinary: filepath.Join(tmpDir, "zarf-injector"),
		// gets created here
		InjectorPayloadTarGz: filepath.Join(tmpDir, "payload.tar.gz"),
	}

	if err := helpers.CreateDirectory(tmp.SeedImagesDir, helpers.ReadWriteExecuteUser); err != nil {
		spinner.Fatalf(err, "Unable to create the seed images directory")
	}

	var err error
	var images imageNodeMap
	var payloadConfigmaps []string
	var sha256sum string
	var seedImages []transform.Image

	// Get all the images from the cluster
	timeout := 5 * time.Minute
	spinner.Updatef("Getting the list of existing cluster images (%s timeout)", timeout.String())
	if images, err = c.getImagesAndNodesForInjection(timeout); err != nil {
		spinner.Fatalf(err, "Unable to generate a list of candidate images to perform the registry injection")
	}

	spinner.Updatef("Creating the injector configmap")
	if err = c.createInjectorConfigmap(tmp.InjectionBinary); err != nil {
		spinner.Fatalf(err, "Unable to create the injector configmap")
	}

	spinner.Updatef("Creating the injector service")
	if service, err := c.createService(); err != nil {
		spinner.Fatalf(err, "Unable to create the injector service")
	} else {
		config.ZarfSeedPort = fmt.Sprintf("%d", service.Spec.Ports[0].NodePort)
	}

	spinner.Updatef("Loading the seed image from the package")
	if seedImages, err = c.loadSeedImages(imagesDir, tmp.SeedImagesDir, injectorSeedSrcs, spinner); err != nil {
		spinner.Fatalf(err, "Unable to load the injector seed image from the package")
	}

	spinner.Updatef("Loading the seed registry configmaps")
	if payloadConfigmaps, sha256sum, err = c.createPayloadConfigmaps(tmp.SeedImagesDir, tmp.InjectorPayloadTarGz, spinner); err != nil {
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
		_ = c.DeletePod(ZarfNamespaceName, "injector")

		// Update the podspec image path and use the first node found
		pod, err := c.buildInjectionPod(node[0], image, payloadConfigmaps, sha256sum)
		if err != nil {
			// Just debug log the output because failures just result in trying the next image
			message.Debug(err)
			continue
		}

		// Create the pod in the cluster
		pod, err = c.CreatePod(pod)
		if err != nil {
			// Just debug log the output because failures just result in trying the next image
			message.Debug(pod, err)
			continue
		}

		// if no error, try and wait for a seed image to be present, return if successful
		if c.injectorIsReady(seedImages, spinner) {
			spinner.Success()
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
	if err := c.DeletePod(ZarfNamespaceName, "injector"); err != nil {
		return err
	}

	// Remove the configmaps
	labelMatch := map[string]string{"zarf-injector": "payload"}
	if err := c.DeleteConfigMapsByLabel(ZarfNamespaceName, labelMatch); err != nil {
		return err
	}

	// Remove the injector service
	return c.DeleteService(ZarfNamespaceName, "zarf-injector")
}

func (c *Cluster) loadSeedImages(imagesDir, seedImagesDir string, injectorSeedSrcs []string, spinner *message.Spinner) ([]transform.Image, error) {
	seedImages := []transform.Image{}
	localReferenceToDigest := make(map[string]string)

	// Load the injector-specific images and save them as seed-images
	for _, src := range injectorSeedSrcs {
		spinner.Updatef("Loading the seed image '%s' from the package", src)
		ref, err := transform.ParseImageRef(src)
		if err != nil {
			return seedImages, fmt.Errorf("failed to create ref for image %s: %w", src, err)
		}
		img, err := utils.LoadOCIImage(imagesDir, ref)
		if err != nil {
			return seedImages, err
		}

		crane.SaveOCI(img, seedImagesDir)

		seedImages = append(seedImages, ref)

		// Get the image digest so we can set an annotation in the image.json later
		imgDigest, err := img.Digest()
		if err != nil {
			return seedImages, err
		}
		// This is done _without_ the domain (different from pull.go) since the injector only handles local images
		localReferenceToDigest[ref.Path+ref.TagOrDigest] = imgDigest.String()
	}

	if err := utils.AddImageNameAnnotation(seedImagesDir, localReferenceToDigest); err != nil {
		return seedImages, fmt.Errorf("unable to format OCI layout: %w", err)
	}

	return seedImages, nil
}

func (c *Cluster) createPayloadConfigmaps(seedImagesDir, tarPath string, spinner *message.Spinner) ([]string, string, error) {
	var configMaps []string

	// Chunk size has to accommodate base64 encoding & etcd 1MB limit
	tarFileList, err := filepath.Glob(filepath.Join(seedImagesDir, "*"))
	if err != nil {
		return configMaps, "", err
	}

	spinner.Updatef("Creating the seed registry archive to send to the cluster")
	// Create a tar archive of the injector payload
	if err := archiver.Archive(tarFileList, tarPath); err != nil {
		return configMaps, "", err
	}

	chunks, sha256sum, err := helpers.ReadFileByChunks(tarPath, payloadChunkSize)
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
		if _, err = c.ReplaceConfigmap(ZarfNamespaceName, fileName, configData); err != nil {
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
func (c *Cluster) injectorIsReady(seedImages []transform.Image, spinner *message.Spinner) bool {
	tunnel, err := c.NewTunnel(ZarfNamespaceName, k8s.SvcResource, ZarfInjectorName, "", 0, ZarfInjectorPort)
	if err != nil {
		return false
	}

	_, err = tunnel.Connect()
	if err != nil {
		return false
	}
	defer tunnel.Close()

	spinner.Updatef("Testing the injector for seed image availability")

	for _, seedImage := range seedImages {
		seedRegistry := fmt.Sprintf("%s/v2/%s/manifests/%s", tunnel.HTTPEndpoint(), seedImage.Path, seedImage.Tag)

		var resp *http.Response
		var err error
		err = tunnel.Wrap(func() error {
			resp, err = http.Get(seedRegistry)
			return err
		})

		if err != nil || resp.StatusCode != 200 {
			// Just debug log the output because failures just result in trying the next image
			message.Debug(resp, err)
			return false
		}
	}

	spinner.Updatef("Seed image found, injector is ready")
	return true
}

func (c *Cluster) createInjectorConfigmap(binaryPath string) error {
	var err error
	configData := make(map[string][]byte)

	// Add the injector binary data to the configmap
	if configData["zarf-injector"], err = os.ReadFile(binaryPath); err != nil {
		return err
	}

	// Try to delete configmap silently
	_ = c.DeleteConfigmap(ZarfNamespaceName, "rust-binary")

	// Attempt to create the configmap in the cluster
	if _, err = c.CreateConfigmap(ZarfNamespaceName, "rust-binary", configData); err != nil {
		return err
	}

	return nil
}

func (c *Cluster) createService() (*corev1.Service, error) {
	service := c.GenerateService(ZarfNamespaceName, "zarf-injector")

	service.Spec.Type = corev1.ServiceTypeNodePort
	service.Spec.Ports = append(service.Spec.Ports, corev1.ServicePort{
		Port: int32(5000),
	})
	service.Spec.Selector = map[string]string{
		"app": "zarf-injector",
	}

	// Attempt to purse the service silently
	_ = c.DeleteService(ZarfNamespaceName, "zarf-injector")

	return c.CreateService(service)
}

// buildInjectionPod return a pod for injection with the appropriate containers to perform the injection.
func (c *Cluster) buildInjectionPod(node, image string, payloadConfigmaps []string, payloadShasum string) (*corev1.Pod, error) {
	pod := c.GeneratePod("injector", ZarfNamespaceName)
	executeMode := int32(0777)

	pod.Labels["app"] = "zarf-injector"

	// Ensure zarf agent doesn't break the injector on future runs
	pod.Labels[k8s.AgentLabel] = "ignore"

	// Bind the pod to the node the image was found on
	pod.Spec.NodeName = node

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
					corev1.ResourceCPU:    injectorRequestedCPU,
					corev1.ResourceMemory: injectorRequestedMemory,
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    injectorLimitCPU,
					corev1.ResourceMemory: injectorLimitMemory,
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

// GetImagesFromAvailableNodes checks for images on schedulable nodes within a cluster and returns
func (c *Cluster) getImagesAndNodesForInjection(timeoutDuration time.Duration) (imageNodeMap, error) {
	timeout := time.After(timeoutDuration)
	result := make(imageNodeMap)

	for {
		select {

		// On timeout abort
		case <-timeout:
			return nil, fmt.Errorf("get image list timed-out")

		// After delay, try running
		default:
			pods, err := c.GetPods(corev1.NamespaceAll, metav1.ListOptions{
				FieldSelector: fmt.Sprintf("status.phase=%s", corev1.PodRunning),
			})
			if err != nil {
				return nil, fmt.Errorf("unable to get the list of %q pods in the cluster: %w", corev1.PodRunning, err)
			}

		findImages:
			for _, pod := range pods.Items {
				nodeName := pod.Spec.NodeName

				nodeDetails, err := c.GetNode(nodeName)
				if err != nil {
					return nil, fmt.Errorf("unable to get the node %q: %w", nodeName, err)
				}

				if nodeDetails.Status.Allocatable.Cpu().Cmp(injectorRequestedCPU) < 0 ||
					nodeDetails.Status.Allocatable.Memory().Cmp(injectorRequestedMemory) < 0 {
					continue findImages
				}

				for _, taint := range nodeDetails.Spec.Taints {
					if taint.Effect == corev1.TaintEffectNoSchedule || taint.Effect == corev1.TaintEffectNoExecute {
						continue findImages
					}
				}

				for _, container := range pod.Spec.InitContainers {
					result[container.Image] = append(result[container.Image], nodeName)
				}

				for _, container := range pod.Spec.Containers {
					result[container.Image] = append(result[container.Image], nodeName)
				}

				for _, container := range pod.Spec.EphemeralContainers {
					result[container.Image] = append(result[container.Image], nodeName)
				}
			}
		}

		if len(result) < 1 {
			c.Log("no images found: %w")
			time.Sleep(2 * time.Second)
		} else {
			return result, nil
		}
	}
}
