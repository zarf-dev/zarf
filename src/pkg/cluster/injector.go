// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/google/go-containerregistry/pkg/crane"
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
	"github.com/zarf-dev/zarf/src/pkg/archive"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	v1aa "k8s.io/client-go/applyconfigurations/apps/v1"
	v1ac "k8s.io/client-go/applyconfigurations/core/v1"
	v1am "k8s.io/client-go/applyconfigurations/meta/v1"
)

var zarfImageRegex = regexp.MustCompile(`(?m)^(127\.0\.0\.1|\[::1\]):`)

// StartInjection initializes a Zarf injection into the cluster.
func (c *Cluster) StartInjection(ctx context.Context, tmpDir, imagesDir string, injectorSeedSrcs []string, registryNodePort int, useRegistryProxy bool, ipFamily state.IPFamily) error {
	l := logger.From(ctx)
	start := time.Now()
	// Stop any previous running injection before starting.
	err := c.StopInjection(ctx, useRegistryProxy)
	if err != nil {
		return err
	}

	l.Info("creating Zarf injector resources")

	payloadCmNames, shasum, err := c.CreateInjectorConfigMaps(ctx, tmpDir, imagesDir, injectorSeedSrcs)
	if err != nil {
		return err
	}

	err = c.RunInjection(ctx, useRegistryProxy, payloadCmNames, registryNodePort, shasum, ipFamily)
	if err != nil {
		return err
	}

	l.Debug("done with injection", "duration", time.Since(start))
	return nil
}

// CreateInjectorConfigMaps creates the required configmaps to run the injector
func (c *Cluster) CreateInjectorConfigMaps(ctx context.Context, tmpDir, imagesDir string, injectorSeedSrcs []string) ([]string, string, error) {
	payloadCmNames, shasum, err := c.createPayloadConfigMaps(ctx, tmpDir, imagesDir, injectorSeedSrcs)
	if err != nil {
		return nil, "", fmt.Errorf("unable to generate the injector payload configmaps: %w", err)
	}

	b, err := os.ReadFile(filepath.Join(tmpDir, "zarf-injector"))
	if err != nil {
		return nil, "", err
	}
	cm := v1ac.ConfigMap("rust-binary", state.ZarfNamespaceName).
		WithBinaryData(map[string][]byte{
			"zarf-injector": b,
		})
	_, err = c.Clientset.CoreV1().ConfigMaps(*cm.Namespace).Apply(ctx, cm, metav1.ApplyOptions{Force: true, FieldManager: FieldManagerName})
	if err != nil {
		return nil, "", err
	}
	return payloadCmNames, shasum, nil
}

// RunInjection starts the injection process. It assumes that the rust and image payload configmaps are already in the cluster
func (c *Cluster) RunInjection(ctx context.Context, useRegistryProxy bool, payloadCmNames []string, registryNodePort int, shasum string, ipFamily state.IPFamily) error {
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

	var zarfSeedPort int32
	if !useRegistryProxy {
		svc, err := c.createInjectorNodeportService(ctx, registryNodePort)
		if err != nil {
			return err
		}
		zarfSeedPort = svc.Spec.Ports[0].NodePort

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
	} else {
		svcAc := v1ac.Service("zarf-injector", state.ZarfNamespaceName).
			WithSpec(v1ac.ServiceSpec().
				WithType(corev1.ServiceTypeClusterIP).
				WithPorts(
					v1ac.ServicePort().WithPort(int32(5000)),
				).WithSelector(map[string]string{
				"app": "zarf-injector",
			}))
		_, err := c.Clientset.CoreV1().Services(*svcAc.Namespace).Apply(ctx, svcAc, metav1.ApplyOptions{Force: true, FieldManager: FieldManagerName})
		if err != nil {
			return err
		}
		dsSpec := buildInjectionDaemonset(injectorImage, payloadCmNames, shasum, resReq, ipFamily)
		ds, err := c.Clientset.AppsV1().DaemonSets(state.ZarfNamespaceName).Apply(ctx, dsSpec, metav1.ApplyOptions{Force: true, FieldManager: FieldManagerName})
		if err != nil {
			return fmt.Errorf("error creating daemonset in cluster: %w", err)
		}
		// FIXME: this should be hostPort for hostport and containerport for the hostNetwork
		zarfSeedPort = ds.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort
		// TODO wait for DaemonSet
	}

	// TODO: Remove use of passing data through global variables.
	config.ZarfSeedPort = int(zarfSeedPort)
	return nil
}

// StopInjection handles cleanup once the seed registry is up.
func (c *Cluster) StopInjection(ctx context.Context, useRegistryProxy bool) error {
	start := time.Now()
	l := logger.From(ctx)
	l.Debug("deleting injector resources")
	if useRegistryProxy {
		err := c.Clientset.AppsV1().DaemonSets(state.ZarfNamespaceName).Delete(ctx, "zarf-injector", metav1.DeleteOptions{})
		if err != nil && !kerrors.IsNotFound(err) {
			return err
		}
		err = c.Clientset.CoreV1().Services(state.ZarfNamespaceName).Delete(ctx, "zarf-injector", metav1.DeleteOptions{})
		if err != nil && !kerrors.IsNotFound(err) {
			return err
		}
	} else {
		err := c.Clientset.CoreV1().Pods(state.ZarfNamespaceName).Delete(ctx, "injector", metav1.DeleteOptions{})
		if err != nil && !kerrors.IsNotFound(err) {
			return err
		}
		err = c.Clientset.CoreV1().Services(state.ZarfNamespaceName).Delete(ctx, "zarf-injector", metav1.DeleteOptions{})
		if err != nil && !kerrors.IsNotFound(err) {
			return err
		}
	}

	err := c.Clientset.CoreV1().ConfigMaps(state.ZarfNamespaceName).Delete(ctx, "rust-binary", metav1.DeleteOptions{})
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
	err = c.Clientset.CoreV1().ConfigMaps(state.ZarfNamespaceName).DeleteCollection(ctx, metav1.DeleteOptions{}, listOpts)
	if err != nil {
		return err
	}

	// This is needed because labels were not present in payload config maps previously.
	// Without this injector will fail if the config maps exist from a previous Zarf version.
	cmList, err := c.Clientset.CoreV1().ConfigMaps(state.ZarfNamespaceName).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, cm := range cmList.Items {
		if !strings.HasPrefix(cm.Name, "zarf-payload-") {
			continue
		}
		err = c.Clientset.CoreV1().ConfigMaps(state.ZarfNamespaceName).Delete(ctx, cm.Name, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	// TODO: Replace with wait package in the future.
	err = wait.PollUntilContextCancel(ctx, time.Second, true, func(ctx context.Context) (bool, error) {
		podList, err := c.Clientset.CoreV1().Pods(state.ZarfNamespaceName).List(ctx, metav1.ListOptions{
			LabelSelector: "zarf.dev/injector",
		})
		if len(podList.Items) == 0 {
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

	if err := archive.Compress(ctx, tarFileList, tarPath, archive.CompressOpts{}); err != nil {
		return nil, "", fmt.Errorf("failed to compress the payload: %w", err)
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

		cm := v1ac.ConfigMap(fileName, state.ZarfNamespaceName).
			WithLabels(map[string]string{
				"zarf-injector": "payload",
			}).
			WithBinaryData(map[string][]byte{
				fileName: data,
			})
		_, err = c.Clientset.CoreV1().ConfigMaps(state.ZarfNamespaceName).Apply(ctx, cm, metav1.ApplyOptions{Force: true, FieldManager: FieldManagerName})
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

// GetInjectorDaemonsetImage gets the image that is most likely to be accessible from all nodes
// First it simply checks if an image is on every node,
// then it checks for an image with pause in the name as the pause image is required to be accessible.
// Finally, it falls back to the smallest image.
func (c *Cluster) GetInjectorDaemonsetImage(ctx context.Context) (string, error) {
	l := logger.From(ctx)

	var injectorImage string
	err := retry.Do(func() error {
		nodes, err := c.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}

		// Track images across all nodes
		imageNodeCount := make(map[string]int)
		allImages := []corev1.ContainerImage{}
		pauseImages := []corev1.ContainerImage{}
		totalNodes := len(nodes.Items)

		for _, node := range nodes.Items {
			for _, image := range node.Status.Images {
				allImages = append(allImages, image)
				for _, name := range image.Names {
					imageNodeCount[name]++
					img, err := transform.ParseImageRef(name)
					if err != nil {
						return err
					}
					if strings.Contains(img.Name, "pause") {
						pauseImages = append(pauseImages, image)
					}
				}
			}
		}

		for imageName, nodeCount := range imageNodeCount {
			if nodeCount == totalNodes {
				injectorImage = imageName
				return nil
			}
		}

		var targetImages []corev1.ContainerImage
		if len(pauseImages) > 0 {
			targetImages = pauseImages
		} else {
			targetImages = allImages
		}

		if len(targetImages) == 0 {
			return errors.New("no suitable image found on any node")
		}

		// Find the smallest image by size
		smallestImage := targetImages[0]
		for _, image := range targetImages[1:] {
			if image.SizeBytes < smallestImage.SizeBytes {
				smallestImage = image
			}
		}

		if len(smallestImage.Names) == 0 {
			return errors.New("selected image has no names")
		}
		injectorImage = smallestImage.Names[0]
		return nil
	}, retry.Attempts(15), retry.Delay(5*time.Second), retry.Context(ctx), retry.DelayType(retry.FixedDelay))
	if err != nil {
		return "", err
	}
	l.Info("selected image for injector Daemonset", "name", injectorImage)

	return injectorImage, nil
}

func hasBlockingTaints(taints []corev1.Taint) bool {
	for _, taint := range taints {
		if taint.Effect == corev1.TaintEffectNoSchedule || taint.Effect == corev1.TaintEffectNoExecute {
			return true
		}
	}
	return false
}

func buildVolumesAndMounts(payloadCmNames []string) ([]*v1ac.VolumeApplyConfiguration, []*v1ac.VolumeMountApplyConfiguration) {
	executeMode := int32(0777)
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
	return volumes, volumeMounts
}

func buildInjectionPod(nodeName, image string, payloadCmNames []string, shasum string, resReq *v1ac.ResourceRequirementsApplyConfiguration) *v1ac.PodApplyConfiguration {
	pod := v1ac.Pod("injector", state.ZarfNamespaceName).
		WithLabels(map[string]string{
			"app":               "zarf-injector",
			"zarf.dev/injector": "true",
			AgentLabel:          "ignore",
		}).
		WithSpec(buildPodSpec(nodeName, corev1.RestartPolicyNever, image, payloadCmNames, shasum, resReq, v1ac.ContainerPort().WithContainerPort(5000)))
	return pod
}

func buildPodSpec(nodeName string, restartPolicy corev1.RestartPolicy, image string, payloadCmNames []string,
	shasum string, resReq *v1ac.ResourceRequirementsApplyConfiguration, containerPorts *v1ac.ContainerPortApplyConfiguration) *v1ac.PodSpecApplyConfiguration {
	userID := int64(1000)
	groupID := int64(2000)
	fsGroupID := int64(2000)
	volumes, volumeMounts := buildVolumesAndMounts(payloadCmNames)
	podSpec :=
		v1ac.PodSpec().
			WithNodeName(nodeName).
			WithRestartPolicy(restartPolicy).
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
					WithPorts(
						containerPorts,
					).
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
			WithVolumes(volumes...)
	return podSpec
}

func buildInjectionDaemonset(image string, payloadCmNames []string, shasum string, resReq *v1ac.ResourceRequirementsApplyConfiguration, ipFamily state.IPFamily) *v1aa.DaemonSetApplyConfiguration {
	var podSpec *v1ac.PodSpecApplyConfiguration
	if ipFamily == state.IPFamilyIPv6 {
		podSpec = buildPodSpec("", corev1.RestartPolicyAlways, image, payloadCmNames, shasum, resReq, v1ac.ContainerPort().WithContainerPort(5000)).
			WithHostNetwork(true)
	} else {
		podSpec = buildPodSpec("", corev1.RestartPolicyAlways, image, payloadCmNames,
			shasum, resReq, v1ac.ContainerPort().WithContainerPort(5000).WithHostIP("127.0.0.1").WithHostPort(5000))
	}
	return v1aa.DaemonSet("zarf-injector", state.ZarfNamespaceName).
		WithSpec(v1aa.DaemonSetSpec().
			WithSelector(v1am.LabelSelector().
				WithMatchLabels(map[string]string{
					"app": "zarf-injector",
				})).
			WithTemplate(v1ac.PodTemplateSpec().
				WithLabels(map[string]string{
					"app":               "zarf-injector",
					"zarf.dev/injector": "true",
					AgentLabel:          "ignore",
				}).
				WithSpec(podSpec)))
}

// createInjectorNodeportService creates the injector service on an available port different than the registryNodePort service
func (c *Cluster) createInjectorNodeportService(ctx context.Context, registryNodePort int) (*corev1.Service, error) {
	l := logger.From(ctx)
	var svc *corev1.Service
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()
	err := retry.Do(func() error {
		svcAc := v1ac.Service("zarf-injector", state.ZarfNamespaceName).
			WithSpec(v1ac.ServiceSpec().
				WithType(corev1.ServiceTypeNodePort).
				WithPorts(
					v1ac.ServicePort().WithPort(int32(5000)),
				).WithSelector(map[string]string{
				"app": "zarf-injector",
			}))

		var err error
		svc, err = c.Clientset.CoreV1().Services(*svcAc.Namespace).Apply(ctx, svcAc, metav1.ApplyOptions{Force: true, FieldManager: FieldManagerName})
		if err != nil {
			return err
		}

		assignedNodePort := int(svc.Spec.Ports[0].NodePort)
		if assignedNodePort == registryNodePort {
			l.Info("injector service NodePort conflicts with registry NodePort, recreating service", "conflictingPort", assignedNodePort)
			deleteErr := c.Clientset.CoreV1().Services(state.ZarfNamespaceName).Delete(ctx, "zarf-injector", metav1.DeleteOptions{})
			if deleteErr != nil {
				return deleteErr
			}
			return fmt.Errorf("nodePort conflict with registry port %d", registryNodePort)
		}
		return nil
	}, retry.Attempts(10), retry.Delay(500*time.Millisecond), retry.Context(timeoutCtx))
	if err != nil {
		return nil, fmt.Errorf("failed to create the injector nodeport service: %w", err)
	}
	return svc, nil
}
