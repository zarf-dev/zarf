// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/mholt/archiver/v3"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/defenseunicorns/pkg/helpers/v2"
	pkgkubernetes "github.com/defenseunicorns/pkg/kubernetes"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
)

// StartInjection initializes a Zarf injection into the cluster.
func (c *Cluster) StartInjection(ctx context.Context, tmpDir, imagesDir string, injectorSeedSrcs []string) error {
	// Stop any previous running injection before starting.
	err := c.StopInjection(ctx)
	if err != nil {
		return err
	}

	spinner := message.NewProgressSpinner("Attempting to bootstrap the seed image into the cluster")
	defer spinner.Stop()

	resReq := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(".5"),
			corev1.ResourceMemory: resource.MustParse("64Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1"),
			corev1.ResourceMemory: resource.MustParse("256Mi"),
		},
	}
	injectorImage, injectorNodeName, err := c.getInjectorImageAndNode(ctx, resReq)
	if err != nil {
		return err
	}

	payloadCmNames, shasum, err := c.createPayloadConfigMaps(ctx, spinner, tmpDir, imagesDir, injectorSeedSrcs)
	if err != nil {
		return fmt.Errorf("unable to generate the injector payload configmaps: %w", err)
	}

	b, err := os.ReadFile(filepath.Join(tmpDir, "zarf-injector"))
	if err != nil {
		return err
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ZarfNamespaceName,
			Name:      "rust-binary",
		},
		BinaryData: map[string][]byte{
			"zarf-injector": b,
		},
	}
	_, err = c.Clientset.CoreV1().ConfigMaps(cm.Namespace).Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	svc := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ZarfNamespaceName,
			Name:      "zarf-injector",
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
	svc, err = c.Clientset.CoreV1().Services(svc.Namespace).Create(ctx, svc, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	// TODO: Remove use of passing data through global variables.
	config.ZarfSeedPort = fmt.Sprintf("%d", svc.Spec.Ports[0].NodePort)

	pod := buildInjectionPod(injectorNodeName, injectorImage, payloadCmNames, shasum, resReq)
	_, err = c.Clientset.CoreV1().Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("error creating pod in cluster: %w", err)
	}

	waitCtx, waitCancel := context.WithTimeout(ctx, 60*time.Second)
	defer waitCancel()
	err = pkgkubernetes.WaitForReadyRuntime(waitCtx, c.Watcher, []runtime.Object{pod})
	if err != nil {
		return err
	}

	spinner.Success()
	return nil
}

// StopInjection handles cleanup once the seed registry is up.
func (c *Cluster) StopInjection(ctx context.Context) error {
	err := c.Clientset.CoreV1().Pods(ZarfNamespaceName).Delete(ctx, "injector", metav1.DeleteOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		return err
	}
	err = c.Clientset.CoreV1().Services(ZarfNamespaceName).Delete(ctx, "zarf-injector", metav1.DeleteOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		return err
	}
	err = c.Clientset.CoreV1().ConfigMaps(ZarfNamespaceName).Delete(ctx, "rust-binary", metav1.DeleteOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		return err
	}
	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{
			"zarf-injector": "payload",
		},
	})
	if err != nil {
		return err
	}
	listOpts := metav1.ListOptions{
		LabelSelector: selector.String(),
	}
	err = c.Clientset.CoreV1().ConfigMaps(ZarfNamespaceName).DeleteCollection(ctx, metav1.DeleteOptions{}, listOpts)
	if err != nil {
		return err
	}

	// This is needed because labels were not present in payload config maps previously.
	// Without this injector will fail if the config maps exist from a previous Zarf version.
	cmList, err := c.Clientset.CoreV1().ConfigMaps(ZarfNamespaceName).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, cm := range cmList.Items {
		if !strings.HasPrefix(cm.Name, "zarf-payload-") {
			continue
		}
		err = c.Clientset.CoreV1().ConfigMaps(ZarfNamespaceName).Delete(ctx, cm.Name, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	// TODO: Replace with wait package in the future.
	err = wait.PollUntilContextCancel(ctx, time.Second, true, func(ctx context.Context) (bool, error) {
		_, err := c.Clientset.CoreV1().Pods(ZarfNamespaceName).Get(ctx, "injector", metav1.GetOptions{})
		if kerrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *Cluster) createPayloadConfigMaps(ctx context.Context, spinner *message.Spinner, tmpDir, imagesDir string, injectorSeedSrcs []string) ([]string, string, error) {
	tarPath := filepath.Join(tmpDir, "payload.tar.gz")
	seedImagesDir := filepath.Join(tmpDir, "seed-images")
	if err := helpers.CreateDirectory(seedImagesDir, helpers.ReadWriteExecuteUser); err != nil {
		return nil, "", fmt.Errorf("unable to create the seed images directory: %w", err)
	}

	localReferenceToDigest := map[string]string{}
	for _, src := range injectorSeedSrcs {
		ref, err := transform.ParseImageRef(src)
		if err != nil {
			return nil, "", fmt.Errorf("failed to create ref for image %s: %w", src, err)
		}
		img, err := utils.LoadOCIImage(imagesDir, ref)
		if err != nil {
			return nil, "", err
		}
		if err := crane.SaveOCI(img, seedImagesDir); err != nil {
			return nil, "", err
		}
		imgDigest, err := img.Digest()
		if err != nil {
			return nil, "", err
		}
		localReferenceToDigest[ref.Path+ref.TagOrDigest] = imgDigest.String()
	}
	if err := utils.AddImageNameAnnotation(seedImagesDir, localReferenceToDigest); err != nil {
		return nil, "", fmt.Errorf("unable to format OCI layout: %w", err)
	}

	// Chunk size has to accommodate base64 encoding & etcd 1MB limit
	tarFileList, err := filepath.Glob(filepath.Join(seedImagesDir, "*"))
	if err != nil {
		return nil, "", err
	}
	if err := archiver.Archive(tarFileList, tarPath); err != nil {
		return nil, "", err
	}
	payloadChunkSize := 1024 * 768
	chunks, shasum, err := helpers.ReadFileByChunks(tarPath, payloadChunkSize)
	if err != nil {
		return nil, "", err
	}

	cmNames := []string{}
	for i, data := range chunks {
		fileName := fmt.Sprintf("zarf-payload-%03d", i)

		spinner.Updatef("Adding archive binary configmap %d of %d to the cluster", i+1, len(chunks))

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ZarfNamespaceName,
				Name:      fileName,
				Labels: map[string]string{
					"zarf-injector": "payload",
				},
			},
			BinaryData: map[string][]byte{
				fileName: data,
			},
		}
		_, err = c.Clientset.CoreV1().ConfigMaps(ZarfNamespaceName).Create(ctx, cm, metav1.CreateOptions{})
		if err != nil {
			return nil, "", err
		}
		cmNames = append(cmNames, fileName)

		// Give the control plane a 250ms buffer between each configmap
		time.Sleep(250 * time.Millisecond)
	}
	return cmNames, shasum, nil
}

// getImagesAndNodesForInjection checks for images on schedulable nodes within a cluster.
func (c *Cluster) getInjectorImageAndNode(ctx context.Context, resReq corev1.ResourceRequirements) (string, string, error) {
	// Regex for Zarf seed image
	zarfImageRegex, err := regexp.Compile(`(?m)^127\.0\.0\.1:`)
	if err != nil {
		return "", "", err
	}
	listOpts := metav1.ListOptions{
		FieldSelector: fmt.Sprintf("status.phase=%s", corev1.PodRunning),
	}
	podList, err := c.Clientset.CoreV1().Pods(corev1.NamespaceAll).List(ctx, listOpts)
	if err != nil {
		return "", "", err
	}
	for _, pod := range podList.Items {
		nodeDetails, err := c.Clientset.CoreV1().Nodes().Get(ctx, pod.Spec.NodeName, metav1.GetOptions{})
		if err != nil {
			return "", "", err
		}
		if nodeDetails.Status.Allocatable.Cpu().Cmp(resReq.Requests[corev1.ResourceCPU]) < 0 ||
			nodeDetails.Status.Allocatable.Memory().Cmp(resReq.Requests[corev1.ResourceMemory]) < 0 {
			continue
		}
		if hasBlockingTaints(nodeDetails.Spec.Taints) {
			continue
		}
		for _, container := range pod.Spec.Containers {
			if zarfImageRegex.MatchString(container.Image) {
				continue
			}
			return container.Image, pod.Spec.NodeName, nil
		}
		for _, container := range pod.Spec.InitContainers {
			if zarfImageRegex.MatchString(container.Image) {
				continue
			}
			return container.Image, pod.Spec.NodeName, nil
		}
		for _, container := range pod.Spec.EphemeralContainers {
			if zarfImageRegex.MatchString(container.Image) {
				continue
			}
			return container.Image, pod.Spec.NodeName, nil
		}
	}
	return "", "", fmt.Errorf("no suitable injector image or node exists")
}

func hasBlockingTaints(taints []corev1.Taint) bool {
	for _, taint := range taints {
		if taint.Effect == corev1.TaintEffectNoSchedule || taint.Effect == corev1.TaintEffectNoExecute {
			return true
		}
	}
	return false
}

func buildInjectionPod(nodeName, image string, payloadCmNames []string, shasum string, resReq corev1.ResourceRequirements) *corev1.Pod {
	executeMode := int32(0777)

	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "injector",
			Namespace: ZarfNamespaceName,
			Labels: map[string]string{
				"app":      "zarf-injector",
				AgentLabel: "ignore",
			},
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
			// Do not try to restart the pod as it will be deleted/re-created instead.
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:            "injector",
					Image:           image,
					ImagePullPolicy: corev1.PullIfNotPresent,
					WorkingDir:      "/zarf-init",
					Command:         []string{"/zarf-init/zarf-injector", shasum},
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
					ReadinessProbe: &corev1.Probe{
						PeriodSeconds:    2,
						SuccessThreshold: 1,
						FailureThreshold: 10,
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/v2/",
								Port: intstr.FromInt(5000),
							},
						},
					},
					Resources: resReq,
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

	for _, filename := range payloadCmNames {
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
			MountPath: fmt.Sprintf("/zarf-init/%s", filename),
			SubPath:   filename,
		})
	}

	return pod
}
