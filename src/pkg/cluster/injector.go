// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/avast/retry-go/v4"
	"github.com/google/go-containerregistry/pkg/crane"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
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
	kubeyaml "k8s.io/apimachinery/pkg/util/yaml"
	v1ac "k8s.io/client-go/applyconfigurations/core/v1"
	componenthelpers "k8s.io/component-helpers/resource"
	"sigs.k8s.io/yaml"
)

var zarfImageRegex = regexp.MustCompile(`(?m)^(127\.0\.0\.1|\[::1\]):`)

const injectionTemplateFileName = "injector-nodeport.yaml"

// ZarfInjectorOptions represents the options used by injector pod
type ZarfInjectorOptions struct {
	// RegistryNodePort, using uint16 allows for only valid ports; 0 lets Kubernetes choose
	RegistryNodePort uint16
	// InjectorNodePort, using uint16 allows for only valid ports; 0 lets Kubernetes choose
	InjectorNodePort uint16
	// IPFamily determines the injector listen address; defaults to IPv4 when unset
	IPFamily state.IPFamily
}

// StartInjection initializes a Zarf injection into the cluster.
// Returns the image used for the injector pod and the node port the injector service is exposed on.
func (c *Cluster) StartInjection(ctx context.Context, tmpDir, imagesDir string, injectorSeedSrcs []string, pkgName string, architecture string, opts ZarfInjectorOptions) (string, int, error) {
	l := logger.From(ctx)
	start := time.Now()

	// The injector breaks if the same image is added multiple times
	injectorSeedSrcs = helpers.Unique(injectorSeedSrcs)

	// Stop any previous running injection before starting.
	err := c.StopInjection(ctx)
	if err != nil {
		return "", 0, err
	}

	l.Info("creating Zarf injector resources")
	podTemplate, serviceTemplate, err := loadInjectionTemplates(tmpDir)
	if err != nil {
		return "", 0, err
	}

	svc, err := c.createInjectorNodeportService(ctx, pkgName, opts, serviceTemplate)
	if err != nil {
		return "", 0, err
	}

	payloadCmNames, shasum, err := c.CreateInjectorConfigMaps(ctx, tmpDir, imagesDir, injectorSeedSrcs, pkgName)
	if err != nil {
		return "", 0, err
	}

	resReq := v1ac.ResourceRequirements().
		WithRequests(corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(".5"),
			corev1.ResourceMemory: resource.MustParse("64Mi"),
		}).
		WithLimits(corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1"),
			corev1.ResourceMemory: resource.MustParse("256Mi"),
		})
	if podTemplate != nil {
		resReq = injectionTemplateResources(podTemplate, resReq)
	}
	var scheduling *corev1.PodSpec
	if podTemplate != nil {
		scheduling = &podTemplate.Spec
	}
	injectorImage, injectorNodeName, err := c.getInjectorImageAndNode(ctx, resReq, architecture, scheduling)
	if err != nil {
		return "", 0, err
	}

	pod := buildInjectionPod(injectorNodeName, injectorImage, payloadCmNames, shasum, resReq, pkgName, opts.IPFamily)
	if podTemplate != nil {
		pod, err = mergeInjectionPodTemplate(pod, podTemplate)
		if err != nil {
			return "", 0, err
		}
	}
	_, err = c.Clientset.CoreV1().Pods(*pod.Namespace).Apply(ctx, pod, metav1.ApplyOptions{Force: true, FieldManager: FieldManagerName})
	if err != nil {
		return "", 0, fmt.Errorf("error creating pod in cluster: %w", err)
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
		return "", 0, err
	}

	l.Debug("done with injection", "duration", time.Since(start))
	return injectorImage, int(svc.Spec.Ports[0].NodePort), nil
}

// CreateInjectorConfigMaps creates the required configmaps to run the injector
func (c *Cluster) CreateInjectorConfigMaps(ctx context.Context, tmpDir, imagesDir string, injectorSeedSrcs []string, pkgName string) ([]string, string, error) {
	payloadCmNames, shasum, err := c.createPayloadConfigMaps(ctx, tmpDir, imagesDir, injectorSeedSrcs, pkgName)
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
		}).
		WithLabels(map[string]string{
			PackageLabel: pkgName,
		})
	_, err = c.Clientset.CoreV1().ConfigMaps(*cm.Namespace).Apply(ctx, cm, metav1.ApplyOptions{Force: true, FieldManager: FieldManagerName})
	if err != nil {
		return nil, "", err
	}
	return payloadCmNames, shasum, nil
}

// StopInjection handles cleanup once the seed registry is up.
func (c *Cluster) StopInjection(ctx context.Context) error {
	start := time.Now()
	l := logger.From(ctx)
	l.Debug("deleting injector resources")
	err := c.Clientset.CoreV1().Pods(state.ZarfNamespaceName).Delete(ctx, "injector", metav1.DeleteOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		return err
	}
	err = c.Clientset.CoreV1().Services(state.ZarfNamespaceName).Delete(ctx, "zarf-injector", metav1.DeleteOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		return err
	}
	_, err = retryInjectorRequest(ctx, "delete injector binary ConfigMap", func() error {
		return c.Clientset.CoreV1().ConfigMaps(state.ZarfNamespaceName).Delete(ctx, "rust-binary", metav1.DeleteOptions{})
	})
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
	_, err = retryInjectorRequest(ctx, "delete injector payload ConfigMaps", func() error {
		return c.Clientset.CoreV1().ConfigMaps(state.ZarfNamespaceName).DeleteCollection(ctx, metav1.DeleteOptions{}, listOpts)
	})
	if err != nil {
		return err
	}

	// TODO: Replace with wait package in the future.
	err = wait.PollUntilContextCancel(ctx, time.Second, true, func(ctx context.Context) (bool, error) {
		_, err := c.Clientset.CoreV1().Pods(state.ZarfNamespaceName).Get(ctx, "injector", metav1.GetOptions{})
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

func (c *Cluster) createPayloadConfigMaps(ctx context.Context, tmpDir, imagesDir string, injectorSeedSrcs []string, pkgName string) ([]string, string, error) {
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
				PackageLabel:    pkgName,
			}).
			WithBinaryData(map[string][]byte{
				fileName: data,
			})
		_, err = c.Clientset.CoreV1().ConfigMaps(state.ZarfNamespaceName).Apply(ctx, cm, metav1.ApplyOptions{Force: true, FieldManager: FieldManagerName})
		if err != nil {
			return nil, "", err
		}
		cmNames = append(cmNames, fileName)

		// Give the control plane a 250ms buffer between each configmap.
		time.Sleep(250 * time.Millisecond)
	}
	return cmNames, shasum, nil
}

// getImagesAndNodesForInjection checks for images on schedulable nodes within a cluster.
func (c *Cluster) getInjectorImageAndNode(ctx context.Context, resReq *v1ac.ResourceRequirementsApplyConfiguration, architecture string, schedulingOpts ...*corev1.PodSpec) (string, string, error) {
	l := logger.From(ctx)
	var scheduling *corev1.PodSpec
	if len(schedulingOpts) > 0 {
		scheduling = schedulingOpts[0]
	}

	// List all nodes and running pods once
	nodeList, err := c.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", "", err
	}
	podList, err := c.Clientset.CoreV1().Pods(corev1.NamespaceAll).List(ctx, metav1.ListOptions{
		FieldSelector: "status.phase=Running",
	})
	if err != nil {
		return "", "", err
	}

	podsByNode := make(map[string][]corev1.Pod)
	for _, pod := range podList.Items {
		if pod.Spec.NodeName == "" {
			continue
		}
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}
		podsByNode[pod.Spec.NodeName] = append(podsByNode[pod.Spec.NodeName], pod)
	}

	type nodeFallback struct {
		image string
		node  string
	}
	var fallback *nodeFallback

	for _, node := range nodeList.Items {
		if !nodeMatchesPodScheduling(node, scheduling) {
			l.Debug("skipping node: pod scheduling constraints", "node", node.Name)
			continue
		}

		if node.Status.NodeInfo.Architecture != "" && node.Status.NodeInfo.Architecture != architecture {
			continue
		}

		availCPU := node.Status.Allocatable.Cpu().DeepCopy()
		availMem := node.Status.Allocatable.Memory().DeepCopy()
		var candidateNoCreds, candidateFallback string

		for _, pod := range podsByNode[node.Name] {
			podReqs := componenthelpers.AggregateContainerRequests(&pod, componenthelpers.PodResourcesOptions{})
			if cpuReq := podReqs.Cpu(); cpuReq != nil {
				availCPU.Sub(*cpuReq)
			}
			if memReq := podReqs.Memory(); memReq != nil {
				availMem.Sub(*memReq)
			}

			noCreds := len(pod.Spec.ImagePullSecrets) == 0
			// Collect candidate images (containers, init, ephemeral), preferring pods without imagePullSecrets
			for _, ctn := range pod.Spec.Containers {
				if zarfImageRegex.MatchString(ctn.Image) {
					continue
				}
				if candidateFallback == "" {
					candidateFallback = ctn.Image
				}
				if noCreds && candidateNoCreds == "" {
					candidateNoCreds = ctn.Image
				}
			}
			for _, ctn := range pod.Spec.InitContainers {
				if zarfImageRegex.MatchString(ctn.Image) {
					continue
				}
				if candidateFallback == "" {
					candidateFallback = ctn.Image
				}
				if noCreds && candidateNoCreds == "" {
					candidateNoCreds = ctn.Image
				}
			}
			for _, ctn := range pod.Spec.EphemeralContainers {
				if zarfImageRegex.MatchString(ctn.Image) {
					continue
				}
				if candidateFallback == "" {
					candidateFallback = ctn.Image
				}
				if noCreds && candidateNoCreds == "" {
					candidateNoCreds = ctn.Image
				}
			}
		}

		l.Debug("calculated available resources",
			"node", node.Name,
			"cpu", availCPU.String(),
			"mem", availMem.String(),
		)

		if availCPU.Cmp(*resReq.Requests.Cpu()) < 0 || availMem.Cmp(*resReq.Requests.Memory()) < 0 {
			l.Debug("skipping node: insufficient resources",
				"node", node.Name,
				"requiredCpu", resReq.Requests.Cpu().String(),
				"requiredMem", resReq.Requests.Memory().String(),
				"availCpu", availCPU.String(),
				"availMem", availMem.String(),
			)
			continue
		}

		if candidateNoCreds != "" {
			l.Debug("selected image for injector", "node", node.Name, "image", candidateNoCreds)
			return candidateNoCreds, node.Name, nil
		}
		if candidateFallback != "" && fallback == nil {
			l.Debug("found fallback image, continuing search for image without pull credentials", "node", node.Name, "image", candidateFallback)
			fallback = &nodeFallback{image: candidateFallback, node: node.Name}
			continue
		}

		l.Debug("no suitable image found on node", "node", node.Name)
	}

	if fallback != nil {
		l.Debug("selected fallback image for injector", "node", fallback.node, "image", fallback.image)
		return fallback.image, fallback.node, nil
	}
	return "", "", fmt.Errorf("no suitable injector image or node exists")
}

// nodeMatchesPodScheduling implements the scheduling constraints that influence whether an
// injector workload can run on a node. Preferred affinity is intentionally excluded because it
// does not make a node ineligible.
func nodeMatchesPodScheduling(node corev1.Node, podSpec *corev1.PodSpec) bool {
	if node.Spec.Unschedulable {
		return false
	}
	if podSpec == nil {
		return !hasBlockingTaints(node.Spec.Taints)
	}
	for key, value := range podSpec.NodeSelector {
		if node.Labels[key] != value {
			return false
		}
	}
	if podSpec.Affinity != nil && podSpec.Affinity.NodeAffinity != nil {
		required := podSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
		if required != nil && !nodeMatchesSelector(node, required) {
			return false
		}
	}
	return taintsTolerated(node.Spec.Taints, podSpec.Tolerations)
}

func nodeMatchesSelector(node corev1.Node, selector *corev1.NodeSelector) bool {
	for _, term := range selector.NodeSelectorTerms {
		matches := true
		for _, requirement := range term.MatchExpressions {
			if !nodeMatchesRequirement(node.Labels, requirement) {
				matches = false
				break
			}
		}
		if matches {
			for _, requirement := range term.MatchFields {
				if requirement.Key != "metadata.name" || !nodeMatchesRequirement(map[string]string{"metadata.name": node.Name}, requirement) {
					matches = false
					break
				}
			}
		}
		if matches {
			return true
		}
	}
	return false
}

func nodeMatchesRequirement(labels map[string]string, requirement corev1.NodeSelectorRequirement) bool {
	value, exists := labels[requirement.Key]
	switch requirement.Operator {
	case corev1.NodeSelectorOpIn:
		return exists && containsString(requirement.Values, value)
	case corev1.NodeSelectorOpNotIn:
		return !exists || !containsString(requirement.Values, value)
	case corev1.NodeSelectorOpExists:
		return exists
	case corev1.NodeSelectorOpDoesNotExist:
		return !exists
	case corev1.NodeSelectorOpGt, corev1.NodeSelectorOpLt:
		if !exists || len(requirement.Values) != 1 {
			return false
		}
		actual, actualErr := strconv.ParseInt(value, 10, 64)
		expected, expectedErr := strconv.ParseInt(requirement.Values[0], 10, 64)
		if actualErr != nil || expectedErr != nil {
			return false
		}
		return (requirement.Operator == corev1.NodeSelectorOpGt && actual > expected) ||
			(requirement.Operator == corev1.NodeSelectorOpLt && actual < expected)
	default:
		return false
	}
}

func taintsTolerated(taints []corev1.Taint, tolerations []corev1.Toleration) bool {
	for _, taint := range taints {
		if taint.Effect != corev1.TaintEffectNoSchedule && taint.Effect != corev1.TaintEffectNoExecute {
			continue
		}
		tolerated := false
		for _, toleration := range tolerations {
			if tolerationMatchesTaint(toleration, taint) {
				tolerated = true
				break
			}
		}
		if !tolerated {
			return false
		}
	}
	return true
}

func tolerationMatchesTaint(toleration corev1.Toleration, taint corev1.Taint) bool {
	if toleration.Effect != "" && toleration.Effect != taint.Effect {
		return false
	}
	if toleration.Key != "" && toleration.Key != taint.Key {
		return false
	}
	switch toleration.Operator {
	case "", corev1.TolerationOpEqual:
		return toleration.Value == taint.Value
	case corev1.TolerationOpExists:
		return true
	default:
		return false
	}
}

func containsString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

// GetInjectorDaemonsetImage gets the image that is most likely to be accessible from all nodes
// It first grabs the latest version pause image with semver 3 or 4, under 1MiB, and with pause in the name.
// If there are no valid pause images then it grabs the smallest image.
func (c *Cluster) GetInjectorDaemonsetImage(ctx context.Context) (string, error) {
	l := logger.From(ctx)

	var injectorImage string
	err := retry.Do(func() error {
		nodes, err := c.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}

		// Track images across all nodes
		allImages := []corev1.ContainerImage{}
		validPauseImages := []pauseImageInfo{}

		for _, node := range nodes.Items {
			for _, image := range node.Status.Images {
				zarfImage := false
				for _, name := range image.Names {
					if zarfImageRegex.MatchString(name) {
						zarfImage = true
					}
				}
				if zarfImage {
					continue
				}

				allImages = append(allImages, image)
				for _, name := range image.Names {
					if pauseInfo := determinePauseImage(name, image.SizeBytes); pauseInfo != nil {
						validPauseImages = append(validPauseImages, *pauseInfo)
					}
				}
			}
		}

		if len(validPauseImages) > 0 {
			// Find the latest (highest) version pause image
			latestPause := validPauseImages[0]
			for _, pauseImg := range validPauseImages[1:] {
				if pauseImg.version.GreaterThan(latestPause.version) {
					latestPause = pauseImg
				}
			}
			injectorImage = latestPause.name
			return nil
		}

		// Fallback to smallest image if no valid pause images
		if len(allImages) == 0 {
			return errors.New("no suitable image found on any node")
		}

		// Find the smallest image by size
		smallestImage := allImages[0]
		for _, image := range allImages[1:] {
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
	l.Info("selected image for injector DaemonSet", "name", injectorImage)

	return injectorImage, nil
}

// GetInjectorDaemonsetImageForPod selects an image that is resident on every node
// eligible for the DaemonSet's PodSpec. An optional override must satisfy the same
// requirement so that a DaemonSet cannot strand an injector pod on a node.
func (c *Cluster) GetInjectorDaemonsetImageForPod(ctx context.Context, podSpec *corev1.PodSpec, override string) (string, error) {
	l := logger.From(ctx)
	var injectorImage string
	err := retry.Do(func() error {
		nodes, err := c.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}

		common := map[string]int64{}
		eligibleNodes := 0
		for _, node := range nodes.Items {
			if !nodeMatchesPodScheduling(node, podSpec) {
				continue
			}
			eligibleNodes++
			nodeImages := residentImages(node.Status.Images)
			if eligibleNodes == 1 {
				common = nodeImages
				continue
			}
			for name, size := range common {
				nodeSize, found := nodeImages[name]
				if !found {
					delete(common, name)
					continue
				}
				if nodeSize > size {
					common[name] = nodeSize
				}
			}
		}

		if eligibleNodes == 0 {
			return retry.Unrecoverable(errors.New("no nodes are eligible for the injector DaemonSet"))
		}
		if len(common) == 0 {
			return retry.Unrecoverable(fmt.Errorf("no common resident image exists across %d eligible injector DaemonSet nodes", eligibleNodes))
		}
		if override != "" {
			if _, found := common[override]; !found {
				return retry.Unrecoverable(fmt.Errorf("requested injector image %q is not resident on every eligible injector DaemonSet node", override))
			}
			injectorImage = override
			return nil
		}

		var latestPause *pauseImageInfo
		var smallestName string
		var smallestSize int64
		for name, size := range common {
			if pause := determinePauseImage(name, size); pause != nil && (latestPause == nil || pause.version.GreaterThan(latestPause.version)) {
				latestPause = pause
			}
			if smallestName == "" || size < smallestSize {
				smallestName = name
				smallestSize = size
			}
		}
		if latestPause != nil {
			injectorImage = latestPause.name
			return nil
		}
		injectorImage = smallestName
		return nil
	}, retry.Attempts(15), retry.Delay(5*time.Second), retry.Context(ctx), retry.DelayType(retry.FixedDelay))
	if err != nil {
		return "", err
	}
	l.Info("selected common image for injector DaemonSet", "name", injectorImage)
	return injectorImage, nil
}

func residentImages(images []corev1.ContainerImage) map[string]int64 {
	result := map[string]int64{}
	for _, image := range images {
		isZarfImage := false
		for _, name := range image.Names {
			if zarfImageRegex.MatchString(name) {
				isZarfImage = true
				break
			}
		}
		if isZarfImage {
			continue
		}
		for _, name := range image.Names {
			result[name] = image.SizeBytes
		}
	}
	return result
}

type pauseImageInfo struct {
	name    string
	version *semver.Version
	size    int64
}

// determinePauseImage helps us judge if an image is likely to be a pause image with the following criteria:
// - Name contains "pause"
// - Semver version 3.x or 4.x
// - Size is less than 1 MiB (1048576 bytes)
func determinePauseImage(imageName string, sizeBytes int64) *pauseImageInfo {
	if !strings.Contains(imageName, "pause") {
		return nil
	}
	// The pause image is currently ~300 KB. Feels relatively safe to assume it will be continue to be less than 1mib
	// This helps avoid images that coincidentally have pause in the name
	OneMiB := int64(1024 * 1024)
	if sizeBytes > OneMiB {
		return nil
	}

	img, err := transform.ParseImageRef(imageName)
	if err != nil {
		return nil
	}

	ver, err := semver.NewVersion(img.Tag)
	if err != nil {
		return nil
	}

	// The pause image is currently on 3.11. It was upgraded to version 3, seven years ago
	// Feels safe to assume it will be version 3 or 4 for the foreseeable future, and we can update this when a new version comes out.
	if ver.Major() != 3 && ver.Major() != 4 {
		return nil
	}

	return &pauseImageInfo{
		name:    imageName,
		version: ver,
		size:    sizeBytes,
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

func buildInjectionPod(nodeName, image string, payloadCmNames []string, shasum string, resReq *v1ac.ResourceRequirementsApplyConfiguration, pkgName string, ipFamily state.IPFamily) *v1ac.PodApplyConfiguration {
	listenHost := "0.0.0.0"
	if ipFamily == state.IPFamilyIPv6 {
		listenHost = "[::]"
	}
	listenAddr := fmt.Sprintf("%s:%d", listenHost, 5000)

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

	pod := v1ac.Pod("injector", state.ZarfNamespaceName).
		WithLabels(map[string]string{
			"app":        "zarf-injector",
			AgentLabel:   "ignore",
			PackageLabel: pkgName,
		}).
		WithSpec(
			v1ac.PodSpec().
				// The injector doesn't handle sigterm to avoid extra dependencies, so we set it to 1
				WithTerminationGracePeriodSeconds(1).
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
						WithCommand("/zarf-init/zarf-injector", shasum, listenAddr).
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

// loadInjectionTemplates reads the optional staged injector resources. The template is deliberately
// package-owned: older packages omit it and continue through the programmatic fallback.
func loadInjectionTemplates(tmpDir string) (*corev1.Pod, *corev1.Service, error) {
	path := filepath.Join(tmpDir, injectionTemplateFileName)
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("unable to read injector template %s: %w", path, err)
	}

	reader := kubeyaml.NewYAMLReader(bufio.NewReader(bytes.NewReader(b)))
	var pod *corev1.Pod
	var service *corev1.Service
	for {
		doc, readErr := reader.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return nil, nil, fmt.Errorf("unable to read injector template document: %w", readErr)
		}
		if len(bytes.TrimSpace(doc)) == 0 {
			continue
		}
		jsonDoc, err := yaml.YAMLToJSON(doc)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to parse injector template document: %w", err)
		}
		var kind metav1.TypeMeta
		if err := json.Unmarshal(jsonDoc, &kind); err != nil {
			return nil, nil, fmt.Errorf("unable to identify injector template resource: %w", err)
		}
		switch kind.Kind {
		case "Pod":
			if pod != nil {
				return nil, nil, fmt.Errorf("injector template must contain exactly one Pod")
			}
			pod = &corev1.Pod{}
			if err := json.Unmarshal(jsonDoc, pod); err != nil {
				return nil, nil, fmt.Errorf("unable to decode injector Pod template: %w", err)
			}
		case "Service":
			if service != nil {
				return nil, nil, fmt.Errorf("injector template must contain exactly one Service")
			}
			service = &corev1.Service{}
			if err := json.Unmarshal(jsonDoc, service); err != nil {
				return nil, nil, fmt.Errorf("unable to decode injector Service template: %w", err)
			}
		default:
			return nil, nil, fmt.Errorf("injector template contains unsupported kind %q", kind.Kind)
		}
	}
	if pod == nil || service == nil {
		return nil, nil, fmt.Errorf("injector template must contain one Pod and one Service")
	}
	foundInjector := false
	for _, container := range pod.Spec.Containers {
		if container.Name == "injector" {
			foundInjector = true
			break
		}
	}
	if !foundInjector {
		return nil, nil, fmt.Errorf("injector Pod template must contain a container named %q", "injector")
	}
	return pod, service, nil
}

func injectionTemplateResources(pod *corev1.Pod, fallback *v1ac.ResourceRequirementsApplyConfiguration) *v1ac.ResourceRequirementsApplyConfiguration {
	for _, container := range pod.Spec.Containers {
		if container.Name == "injector" && (len(container.Resources.Requests) > 0 || len(container.Resources.Limits) > 0) {
			requests := corev1.ResourceList{}
			limits := corev1.ResourceList{}
			if fallback.Requests != nil {
				requests = fallback.Requests.DeepCopy()
			}
			if fallback.Limits != nil {
				limits = fallback.Limits.DeepCopy()
			}
			for name, quantity := range container.Resources.Requests {
				requests[name] = quantity
			}
			for name, quantity := range container.Resources.Limits {
				limits[name] = quantity
			}
			return v1ac.ResourceRequirements().WithRequests(requests).WithLimits(limits)
		}
	}
	return fallback
}

func mergeInjectionPodTemplate(base *v1ac.PodApplyConfiguration, template *corev1.Pod) (*v1ac.PodApplyConfiguration, error) {
	baseJSON, err := json.Marshal(base)
	if err != nil {
		return nil, err
	}
	templateJSON, err := json.Marshal(template)
	if err != nil {
		return nil, err
	}
	mergedJSON, err := strategicpatch.StrategicMergePatch(baseJSON, templateJSON, corev1.Pod{})
	if err != nil {
		return nil, fmt.Errorf("unable to merge injector Pod template: %w", err)
	}
	merged := &v1ac.PodApplyConfiguration{}
	if err := json.Unmarshal(mergedJSON, merged); err != nil {
		return nil, fmt.Errorf("unable to decode merged injector Pod template: %w", err)
	}
	mergeInjectorRuntimePodFields(merged, base)
	mergedJSON, err = json.Marshal(merged)
	if err != nil {
		return nil, err
	}
	result := &v1ac.PodApplyConfiguration{}
	if err := json.Unmarshal(mergedJSON, result); err != nil {
		return nil, fmt.Errorf("unable to encode merged injector Pod template: %w", err)
	}
	return result, nil
}

func mergeInjectorRuntimePodFields(pod, runtimePod *v1ac.PodApplyConfiguration) {
	pod.Name = runtimePod.Name
	pod.Namespace = runtimePod.Namespace
	if pod.Labels == nil {
		pod.Labels = map[string]string{}
	}
	for key, value := range runtimePod.Labels {
		pod.Labels[key] = value
	}
	if pod.Spec == nil {
		pod.Spec = v1ac.PodSpec()
	}
	pod.Spec.NodeName = runtimePod.Spec.NodeName
	pod.Spec.RestartPolicy = runtimePod.Spec.RestartPolicy
	pod.Spec.Volumes = mergeRuntimeVolumes(pod.Spec.Volumes, runtimePod.Spec.Volumes)

	for _, runtimeContainer := range runtimePod.Spec.Containers {
		for i := range pod.Spec.Containers {
			container := &pod.Spec.Containers[i]
			if container.Name == nil || runtimeContainer.Name == nil || *container.Name != *runtimeContainer.Name {
				continue
			}
			container.Image = runtimeContainer.Image
			container.ImagePullPolicy = runtimeContainer.ImagePullPolicy
			container.WorkingDir = runtimeContainer.WorkingDir
			container.Command = append([]string(nil), runtimeContainer.Command...)
			container.VolumeMounts = mergeRuntimeVolumeMounts(container.VolumeMounts, runtimeContainer.VolumeMounts)
			break
		}
	}
}

func mergeRuntimeVolumes(template, runtime []v1ac.VolumeApplyConfiguration) []v1ac.VolumeApplyConfiguration {
	result := make([]v1ac.VolumeApplyConfiguration, 0, len(template)+len(runtime))
	for _, volume := range template {
		if volume.Name == nil || !containsVolume(runtime, *volume.Name) {
			result = append(result, volume)
		}
	}
	return append(result, runtime...)
}

func containsVolume(volumes []v1ac.VolumeApplyConfiguration, name string) bool {
	for _, volume := range volumes {
		if volume.Name != nil && *volume.Name == name {
			return true
		}
	}
	return false
}

func mergeRuntimeVolumeMounts(template, runtime []v1ac.VolumeMountApplyConfiguration) []v1ac.VolumeMountApplyConfiguration {
	result := make([]v1ac.VolumeMountApplyConfiguration, 0, len(template)+len(runtime))
	for _, mount := range template {
		if mount.Name != nil && !containsVolumeMount(runtime, *mount.Name, mount.MountPath) {
			result = append(result, mount)
		}
	}
	return append(result, runtime...)
}

func containsVolumeMount(mounts []v1ac.VolumeMountApplyConfiguration, name string, mountPath *string) bool {
	for _, mount := range mounts {
		if (mount.Name != nil && *mount.Name == name) || (mountPath != nil && mount.MountPath != nil && *mount.MountPath == *mountPath) {
			return true
		}
	}
	return false
}

// createInjectorNodeportService creates the injector service on an available port different than the registryNodePort service
func (c *Cluster) createInjectorNodeportService(ctx context.Context, pkgName string, opts ZarfInjectorOptions, template *corev1.Service) (*corev1.Service, error) {
	l := logger.From(ctx)
	var svc *corev1.Service
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()
	portConfiguration := v1ac.ServicePort().WithPort(int32(5000))
	if opts.InjectorNodePort != 0 {
		portConfiguration.WithNodePort(int32(opts.InjectorNodePort))
	}
	err := retry.Do(func() error {
		svcAc := v1ac.Service("zarf-injector", state.ZarfNamespaceName).
			WithSpec(v1ac.ServiceSpec().
				WithType(corev1.ServiceTypeNodePort).
				WithPorts(
					portConfiguration,
				).WithSelector(map[string]string{
				"app": "zarf-injector",
			})).WithLabels(map[string]string{
			PackageLabel: pkgName,
		})
		if template != nil {
			var mergeErr error
			svcAc, mergeErr = mergeInjectionServiceTemplate(svcAc, template, opts, pkgName)
			if mergeErr != nil {
				return mergeErr
			}
		}

		var err error
		svc, err = c.Clientset.CoreV1().Services(*svcAc.Namespace).Apply(ctx, svcAc, metav1.ApplyOptions{Force: true, FieldManager: FieldManagerName})
		if err != nil {
			return err
		}

		assignedNodePort := int(svc.Spec.Ports[0].NodePort)
		if assignedNodePort == int(opts.RegistryNodePort) {
			l.Info("injector service NodePort conflicts with registry NodePort, recreating service", "conflictingPort", assignedNodePort)
			deleteErr := c.Clientset.CoreV1().Services(state.ZarfNamespaceName).Delete(ctx, "zarf-injector", metav1.DeleteOptions{})
			if deleteErr != nil {
				return deleteErr
			}
			return fmt.Errorf("nodePort conflict with registry port %d", opts.RegistryNodePort)
		}
		return nil
	}, retry.Attempts(10), retry.Delay(500*time.Millisecond), retry.Context(timeoutCtx))
	if err != nil {
		return nil, fmt.Errorf("failed to create the injector nodeport service: %w", err)
	}
	return svc, nil
}

func mergeInjectionServiceTemplate(base *v1ac.ServiceApplyConfiguration, template *corev1.Service, opts ZarfInjectorOptions, pkgName string) (*v1ac.ServiceApplyConfiguration, error) {
	baseJSON, err := json.Marshal(base)
	if err != nil {
		return nil, err
	}
	templateJSON, err := json.Marshal(template)
	if err != nil {
		return nil, err
	}
	mergedJSON, err := strategicpatch.StrategicMergePatch(baseJSON, templateJSON, corev1.Service{})
	if err != nil {
		return nil, fmt.Errorf("unable to merge injector Service template: %w", err)
	}
	merged := &corev1.Service{}
	if err := json.Unmarshal(mergedJSON, merged); err != nil {
		return nil, fmt.Errorf("unable to decode merged injector Service template: %w", err)
	}
	merged.Name = "zarf-injector"
	merged.Namespace = state.ZarfNamespaceName
	if merged.Labels == nil {
		merged.Labels = map[string]string{}
	}
	merged.Labels[PackageLabel] = pkgName
	merged.Spec.Type = corev1.ServiceTypeNodePort
	if merged.Spec.Selector == nil {
		merged.Spec.Selector = map[string]string{}
	}
	merged.Spec.Selector["app"] = "zarf-injector"
	if len(merged.Spec.Ports) == 0 {
		merged.Spec.Ports = []corev1.ServicePort{{Port: 5000}}
	}
	merged.Spec.Ports[0].Port = 5000
	if opts.InjectorNodePort != 0 {
		merged.Spec.Ports[0].NodePort = int32(opts.InjectorNodePort)
	} else {
		merged.Spec.Ports[0].NodePort = 0
	}
	mergedJSON, err = json.Marshal(merged)
	if err != nil {
		return nil, err
	}
	result := &v1ac.ServiceApplyConfiguration{}
	if err := json.Unmarshal(mergedJSON, result); err != nil {
		return nil, err
	}
	return result, nil
}

// retryInjectorRequest retries headerless transient Kubernetes API throttles for injector payload cleanup.
func retryInjectorRequest(ctx context.Context, operation string, request func() error) (retried bool, err error) {
	l := logger.From(ctx)
	err = retry.Do(func() error {
		requestErr := request()
		if requestErr == nil {
			return nil
		}
		if !kerrors.IsTooManyRequests(requestErr) || hasServerRetryDelay(requestErr) {
			return retry.Unrecoverable(requestErr)
		}
		return requestErr
	},
		retry.Attempts(uint(config.ZarfDefaultRetries)),
		retry.Delay(config.ZarfDefaultRetryDelay),
		retry.MaxDelay(config.ZarfDefaultRetryMaxDelay),
		retry.DelayType(func(n uint, requestErr error, retryConfig *retry.Config) time.Duration {
			delay := retry.CombineDelay(retry.BackOffDelay, retry.RandomDelay)(n, requestErr, retryConfig)
			retried = true
			l.Warn("retrying transient Kubernetes API request",
				"operation", operation,
				"attempt", n+1,
				"maxAttempts", config.ZarfDefaultRetries,
				"nextDelay", delay,
				"error", requestErr,
			)
			return delay
		}),
		retry.LastErrorOnly(true),
		retry.Context(ctx),
	)
	return retried, err
}

func hasServerRetryDelay(err error) bool {
	_, ok := kerrors.SuggestsClientDelay(err)
	return ok
}
