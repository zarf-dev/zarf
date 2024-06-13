// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/defenseunicorns/pkg/helpers/v2"
	pkgkubernetes "github.com/defenseunicorns/pkg/kubernetes"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/k8s"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/mholt/archiver/v3"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/cli-utils/pkg/object"
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
func (c *Cluster) StartInjectionMadness(ctx context.Context, tmpDir string, imagesDir string, injectorSeedSrcs []string) error {
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
		return fmt.Errorf("unable to create the seed images directory: %w", err)
	}

	var err error
	var images imageNodeMap
	var payloadConfigmaps []string
	var sha256sum string

	findImagesCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	images, err = c.getImagesAndNodesForInjection(findImagesCtx)
	if err != nil {
		return err
	}

	if err = c.createInjectorConfigMap(ctx, tmp.InjectionBinary); err != nil {
		return fmt.Errorf("unable to create the injector configmap: %w", err)
	}

	service, err := c.createService(ctx)
	if err != nil {
		return fmt.Errorf("unable to create the injector service: %w", err)
	}
	config.ZarfSeedPort = fmt.Sprintf("%d", service.Spec.Ports[0].NodePort)

	_, err = c.loadSeedImages(imagesDir, tmp.SeedImagesDir, injectorSeedSrcs)
	if err != nil {
		return fmt.Errorf("unable to load the injector seed image from the package: %w", err)
	}

	if payloadConfigmaps, sha256sum, err = c.createPayloadConfigMaps(ctx, tmp.SeedImagesDir, tmp.InjectorPayloadTarGz, spinner); err != nil {
		return fmt.Errorf("unable to generate the injector payload configmaps: %w", err)
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
		// TODO: Explain why no grace period is given.
		deleteGracePeriod := int64(0)
		deletePolicy := metav1.DeletePropagationForeground
		deleteOpts := metav1.DeleteOptions{
			GracePeriodSeconds: &deleteGracePeriod,
			PropagationPolicy:  &deletePolicy,
		}
		selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": "zarf-injector",
			},
		})
		if err != nil {
			return err
		}
		listOpts := metav1.ListOptions{
			LabelSelector: selector.String(),
		}
		err = c.Clientset.CoreV1().Pods(ZarfNamespaceName).DeleteCollection(ctx, deleteOpts, listOpts)
		if err != nil {
			return err
		}

		pod, err := c.buildInjectionPod(node[0], image, payloadConfigmaps, sha256sum)
		if err != nil {
			return fmt.Errorf("error making injection pod: %w", err)
		}

		pod, err = c.Clientset.CoreV1().Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("error creating pod in cluster: %w", err)
		}

		objs := []object.ObjMetadata{
			{
				GroupKind: schema.GroupKind{
					Kind: "Pod",
				},
				Namespace: ZarfNamespaceName,
				Name:      pod.Name,
			},
		}
		waitCtx, waitCancel := context.WithTimeout(ctx, 60*time.Second)
		defer waitCancel()
		err = pkgkubernetes.WaitForReady(waitCtx, c.Watcher, objs)
		if err != nil {
			return err
		}
		spinner.Success()
		// Otherwise just continue to try next image
	}
	return nil
}

// StopInjectionMadness handles cleanup once the seed registry is up.
func (c *Cluster) StopInjectionMadness(ctx context.Context) error {
	// Try to kill the injector pod now
	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app": "zarf-injector",
		},
	})
	if err != nil {
		return err
	}
	listOpts := metav1.ListOptions{
		LabelSelector: selector.String(),
	}
	err = c.Clientset.CoreV1().Pods(ZarfNamespaceName).DeleteCollection(ctx, metav1.DeleteOptions{}, listOpts)
	if err != nil {
		return err
	}

	// Remove the configmaps
	selector, err = metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{
			"zarf-injector": "payload",
		},
	})
	if err != nil {
		return err
	}
	listOpts = metav1.ListOptions{
		LabelSelector: selector.String(),
	}
	err = c.Clientset.CoreV1().ConfigMaps(ZarfNamespaceName).DeleteCollection(ctx, metav1.DeleteOptions{}, listOpts)
	if err != nil {
		return err
	}

	// Remove the injector service
	err = c.Clientset.CoreV1().Services(ZarfNamespaceName).Delete(ctx, "zarf-injector", metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (c *Cluster) loadSeedImages(imagesDir, seedImagesDir string, injectorSeedSrcs []string) ([]transform.Image, error) {
	seedImages := []transform.Image{}
	localReferenceToDigest := make(map[string]string)

	// Load the injector-specific images and save them as seed-images
	for _, src := range injectorSeedSrcs {
		ref, err := transform.ParseImageRef(src)
		if err != nil {
			return nil, fmt.Errorf("failed to create ref for image %s: %w", src, err)
		}
		img, err := utils.LoadOCIImage(imagesDir, ref)
		if err != nil {
			return nil, err
		}

		if err := crane.SaveOCI(img, seedImagesDir); err != nil {
			return nil, err
		}

		seedImages = append(seedImages, ref)

		// Get the image digest so we can set an annotation in the image.json later
		imgDigest, err := img.Digest()
		if err != nil {
			return nil, err
		}
		// This is done _without_ the domain (different from pull.go) since the injector only handles local images
		localReferenceToDigest[ref.Path+ref.TagOrDigest] = imgDigest.String()
	}

	if err := utils.AddImageNameAnnotation(seedImagesDir, localReferenceToDigest); err != nil {
		return nil, fmt.Errorf("unable to format OCI layout: %w", err)
	}

	return seedImages, nil
}

func (c *Cluster) createPayloadConfigMaps(ctx context.Context, seedImagesDir, tarPath string, spinner *message.Spinner) ([]string, string, error) {
	var configMaps []string

	// Chunk size has to accommodate base64 encoding & etcd 1MB limit
	tarFileList, err := filepath.Glob(filepath.Join(seedImagesDir, "*"))
	if err != nil {
		return configMaps, "", err
	}

	// Create a tar archive of the injector payload
	if err := archiver.Archive(tarFileList, tarPath); err != nil {
		return configMaps, "", err
	}

	chunks, sha256sum, err := helpers.ReadFileByChunks(tarPath, payloadChunkSize)
	if err != nil {
		return configMaps, "", err
	}

	chunkCount := len(chunks)

	// Loop over all chunks and generate configmaps
	for idx, data := range chunks {
		// Create a cat-friendly filename
		fileName := fmt.Sprintf("zarf-payload-%03d", idx)

		spinner.Updatef("Adding archive binary configmap %d of %d to the cluster", idx+1, chunkCount)

		// Attempt to create the configmap in the cluster
		// TODO: Replace with create or update.
		err := c.Clientset.CoreV1().ConfigMaps(ZarfNamespaceName).Delete(ctx, fileName, metav1.DeleteOptions{})
		if err != nil && !kerrors.IsNotFound(err) {
			return nil, "", err
		}
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fileName,
				Namespace: ZarfNamespaceName,
			},
			BinaryData: map[string][]byte{
				fileName: data,
			},
		}
		_, err = c.Clientset.CoreV1().ConfigMaps(ZarfNamespaceName).Create(ctx, cm, metav1.CreateOptions{})
		if err != nil {
			return nil, "", err
		}

		// Add the configmap to the configmaps slice for later usage in the pod
		configMaps = append(configMaps, fileName)

		// Give the control plane a 250ms buffer between each configmap
		time.Sleep(250 * time.Millisecond)
	}

	return configMaps, sha256sum, nil
}

func (c *Cluster) createInjectorConfigMap(ctx context.Context, binaryPath string) error {
	name := "rust-binary"
	// TODO: Replace with a create or update.
	err := c.Clientset.CoreV1().ConfigMaps(ZarfNamespaceName).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		return err
	}
	b, err := os.ReadFile(binaryPath)
	if err != nil {
		return err
	}
	configData := map[string][]byte{
		"zarf-injector": b,
	}
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ZarfNamespaceName,
		},
		BinaryData: configData,
	}
	_, err = c.Clientset.CoreV1().ConfigMaps(configMap.Namespace).Create(ctx, configMap, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (c *Cluster) createService(ctx context.Context) (*corev1.Service, error) {
	svc := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "zarf-injector",
			Namespace: ZarfNamespaceName,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeNodePort,
			Ports: []corev1.ServicePort{
				{
					Port: int32(5000),
				},
			},
			Selector: map[string]string{
				"app": "zarf-injector",
			},
		},
	}
	// TODO: Replace with create or update
	err := c.Clientset.CoreV1().Services(svc.Namespace).Delete(ctx, svc.Name, metav1.DeleteOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		return nil, err
	}
	svc, err = c.Clientset.CoreV1().Services(svc.Namespace).Create(ctx, svc, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return svc, nil
}

// buildInjectionPod return a pod for injection with the appropriate containers to perform the injection.
func (c *Cluster) buildInjectionPod(node, image string, payloadConfigmaps []string, payloadShasum string) (*corev1.Pod, error) {
	executeMode := int32(0777)

	// Create a SHA-256 hash of the image name to allow unique injector pod names.
	// This prevents collisions where `zarf init` is ran back to back and a previous injector pod still exists.
	hasher := sha256.New()
	if _, err := hasher.Write([]byte(image)); err != nil {
		return nil, err
	}
	hash := hex.EncodeToString(hasher.Sum(nil))[:8]

	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("injector-%s", hash),
			Namespace: ZarfNamespaceName,
			Labels: map[string]string{
				"app":          "zarf-injector",
				k8s.AgentLabel: "ignore",
			},
		},
		Spec: corev1.PodSpec{
			NodeName: node,
			// Do not try to restart the pod as it will be deleted/re-created instead
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
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
			},
			Volumes: []corev1.Volume{
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

// getImagesAndNodesForInjection checks for images on schedulable nodes within a cluster.
func (c *Cluster) getImagesAndNodesForInjection(ctx context.Context) (imageNodeMap, error) {
	result := make(imageNodeMap)

	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("get image list timed-out: %w", ctx.Err())
		case <-timer.C:
			listOpts := metav1.ListOptions{
				FieldSelector: fmt.Sprintf("status.phase=%s", corev1.PodRunning),
			}
			podList, err := c.Clientset.CoreV1().Pods(corev1.NamespaceAll).List(ctx, listOpts)
			if err != nil {
				return nil, fmt.Errorf("unable to get the list of %q pods in the cluster: %w", corev1.PodRunning, err)
			}

			for _, pod := range podList.Items {
				nodeName := pod.Spec.NodeName

				nodeDetails, err := c.Clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
				if err != nil {
					return nil, fmt.Errorf("unable to get the node %q: %w", nodeName, err)
				}

				if nodeDetails.Status.Allocatable.Cpu().Cmp(injectorRequestedCPU) < 0 ||
					nodeDetails.Status.Allocatable.Memory().Cmp(injectorRequestedMemory) < 0 {
					continue
				}

				if hasBlockingTaints(nodeDetails.Spec.Taints) {
					continue
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

			if len(result) > 0 {
				return result, nil
			}

			c.Log("No images found on any node. Retrying...")
			timer.Reset(2 * time.Second)
		}
	}
}

func hasBlockingTaints(taints []corev1.Taint) bool {
	for _, taint := range taints {
		if taint.Effect == corev1.TaintEffectNoSchedule || taint.Effect == corev1.TaintEffectNoExecute {
			return true
		}
	}
	return false
}
