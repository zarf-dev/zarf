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
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/defenseunicorns/pkg/helpers/v2"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/internal/healthchecks"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	v1ac "k8s.io/client-go/applyconfigurations/core/v1"
)

// StartInjection initializes a Zarf injection into the cluster.
func (c *Cluster) StartInjection(ctx context.Context, tmpDir, imagesDir string, injectorSeedSrcs []string) error {
	l := logger.From(ctx)
	start := time.Now()
	// Stop any previous running injection before starting.
	err := c.StopInjection(ctx)
	if err != nil {
		return err
	}

	l.Info("creating Zarf injector resources")

	resReq := v1ac.ResourceRequirements().
		WithRequests(corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(".5"),
			corev1.ResourceMemory: resource.MustParse("64Mi"),
		}).
		WithLimits(corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1"),
			corev1.ResourceMemory: resource.MustParse("256Mi"),
		})
	injectorImage, injectorNodeName, err := c.getInjectorImageAndNode(ctx, resReq)
	if err != nil {
		return err
	}

	payloadCmNames, shasum, err := c.createPayloadConfigMaps(ctx, tmpDir, imagesDir, injectorSeedSrcs)
	if err != nil {
		return fmt.Errorf("unable to generate the injector payload configmaps: %w", err)
	}

	b, err := os.ReadFile(filepath.Join(tmpDir, "zarf-injector"))
	if err != nil {
		return err
	}
	cm := v1ac.ConfigMap("rust-binary", ZarfNamespaceName).
		WithBinaryData(map[string][]byte{
			"zarf-injector": b,
		})
	_, err = c.Clientset.CoreV1().ConfigMaps(*cm.Namespace).Apply(ctx, cm, metav1.ApplyOptions{Force: true, FieldManager: FieldManagerName})
	if err != nil {
		return err
	}

	svcAc := v1ac.Service("zarf-injector", ZarfNamespaceName).
		WithSpec(v1ac.ServiceSpec().
			WithType(corev1.ServiceTypeNodePort).
			WithPorts(
				v1ac.ServicePort().WithPort(int32(5000)),
			).WithSelector(map[string]string{
			"app": "zarf-injector",
		}))
	svc, err := c.Clientset.CoreV1().Services(*svcAc.Namespace).Apply(ctx, svcAc, metav1.ApplyOptions{Force: true, FieldManager: FieldManagerName})
	if err != nil {
		return err
	}
	// TODO: Remove use of passing data through global variables.
	config.ZarfSeedPort = fmt.Sprintf("%d", svc.Spec.Ports[0].NodePort)

	pod := buildInjectionPod(injectorNodeName, injectorImage, payloadCmNames, shasum, resReq)
	_, err = c.Clientset.CoreV1().Pods(*pod.Namespace).Apply(ctx, pod, metav1.ApplyOptions{Force: true, FieldManager: FieldManagerName})
	if err != nil {
		return fmt.Errorf("error creating pod in cluster: %w", err)
	}

	waitCtx, waitCancel := context.WithTimeout(ctx, 60*time.Second)
	defer waitCancel()
	podRef := v1alpha1.NamespacedObjectKindReference{
		APIVersion: *pod.APIVersion,
		Kind:       *pod.Kind,
		Namespace:  *pod.Namespace,
		Name:       *pod.Name,
	}
	err = healthchecks.Run(waitCtx, c.Watcher, []v1alpha1.NamespacedObjectKindReference{podRef})
	if err != nil {
		return err
	}

	l.Debug("done with injection", "duration", time.Since(start))
	return nil
}

// StopInjection handles cleanup once the seed registry is up.
func (c *Cluster) StopInjection(ctx context.Context) error {
	start := time.Now()
	l := logger.From(ctx)
	l.Debug("deleting injector resources")
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
	l.Debug("done deleting injector resources", "duration", time.Since(start))
	return nil
}

func (c *Cluster) createPayloadConfigMaps(ctx context.Context, tmpDir, imagesDir string, injectorSeedSrcs []string) ([]string, string, error) {
	l := logger.From(ctx)
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
	l.Info("adding archived binary configmaps of registry image to the cluster")
	for i, data := range chunks {
		fileName := fmt.Sprintf("zarf-payload-%03d", i)

		cm := v1ac.ConfigMap(fileName, ZarfNamespaceName).
			WithLabels(map[string]string{
				"zarf-injector": "payload",
			}).
			WithBinaryData(map[string][]byte{
				fileName: data,
			})
		_, err = c.Clientset.CoreV1().ConfigMaps(ZarfNamespaceName).Apply(ctx, cm, metav1.ApplyOptions{Force: true, FieldManager: FieldManagerName})
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
func (c *Cluster) getInjectorImageAndNode(ctx context.Context, resReq *v1ac.ResourceRequirementsApplyConfiguration) (string, string, error) {
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
		if nodeDetails.Status.Allocatable.Cpu().Cmp(*resReq.Requests.Cpu()) < 0 ||
			nodeDetails.Status.Allocatable.Memory().Cmp(*resReq.Requests.Memory()) < 0 {
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

func buildInjectionPod(nodeName, image string, payloadCmNames []string, shasum string, resReq *v1ac.ResourceRequirementsApplyConfiguration) *v1ac.PodApplyConfiguration {
	executeMode := int32(0777)
	userID := int64(1000)
	groupID := int64(2000)
	fsGroupID := int64(2000)
	volumes := []*v1ac.VolumeApplyConfiguration{
		v1ac.Volume().
			WithName("init").
			WithConfigMap(
				v1ac.ConfigMapVolumeSource().
					WithName("rust-binary").
					WithDefaultMode(executeMode),
			),
		v1ac.Volume().
			WithName("seed").
			WithEmptyDir(&v1ac.EmptyDirVolumeSourceApplyConfiguration{})}

	volumeMounts := []*v1ac.VolumeMountApplyConfiguration{
		v1ac.VolumeMount().
			WithName("init").
			WithMountPath("/zarf-init/zarf-injector").
			WithSubPath("zarf-injector"),
		v1ac.VolumeMount().
			WithName("seed").
			WithMountPath("/zarf-seed"),
	}

	for _, filename := range payloadCmNames {
		volumes = append(volumes, v1ac.Volume().
			WithName(filename).
			WithConfigMap(
				v1ac.ConfigMapVolumeSource().
					WithName(filename),
			))
		volumeMounts = append(volumeMounts, v1ac.VolumeMount().
			WithName(filename).
			WithMountPath(fmt.Sprintf("/zarf-init/%s", filename)).
			WithSubPath(filename))
	}

	pod := v1ac.Pod("injector", ZarfNamespaceName).
		WithLabels(map[string]string{
			"app":      "zarf-injector",
			AgentLabel: "ignore",
		}).
		WithSpec(
			v1ac.PodSpec().
				WithNodeName(nodeName).
				WithRestartPolicy(corev1.RestartPolicyNever).
				WithSecurityContext(
					v1ac.PodSecurityContext().
						WithRunAsUser(userID).
						WithRunAsGroup(groupID).
						WithFSGroup(fsGroupID).
						WithSeccompProfile(
							v1ac.SeccompProfile().
								WithType(corev1.SeccompProfileTypeRuntimeDefault),
						),
				).
				WithContainers(
					v1ac.Container().
						WithName("injector").
						WithImage(image).
						WithImagePullPolicy(corev1.PullIfNotPresent).
						WithWorkingDir("/zarf-init").
						WithCommand("/zarf-init/zarf-injector", shasum).
						WithVolumeMounts(volumeMounts...).
						WithSecurityContext(
							v1ac.SecurityContext().
								WithReadOnlyRootFilesystem(true).
								WithAllowPrivilegeEscalation(false).
								WithRunAsNonRoot(true).
								WithCapabilities(v1ac.Capabilities().WithDrop(corev1.Capability("ALL"))),
						).
						WithReadinessProbe(
							v1ac.Probe().
								WithPeriodSeconds(2).
								WithSuccessThreshold(1).
								WithFailureThreshold(10).
								WithHTTPGet(
									v1ac.HTTPGetAction().
										WithPath("/v2/").
										WithPort(intstr.FromInt(5000)),
								),
						).
						WithResources(resReq),
				).
				WithVolumes(volumes...),
		)

	return pod
}
