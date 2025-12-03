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

	"github.com/Masterminds/semver/v3"
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
	"github.com/zarf-dev/zarf/src/internal/healthchecks"
	"github.com/zarf-dev/zarf/src/pkg/archive"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	v1ac "k8s.io/client-go/applyconfigurations/core/v1"
	componenthelpers "k8s.io/component-helpers/resource"
)

var zarfImageRegex = regexp.MustCompile(`(?m)^(127\.0\.0\.1|\[::1\]):`)

// ZarfInjectorOptions represents the options used by injector pod
type ZarfInjectorOptions struct {
	ImagesDir        string
	InjectorSeedSrcs []string
	PkgName          string
	Architecture     string
	// Linux/Windows allowable port-ranges are 1-65535, so using a unsigned int 16 enforces the use of a port in that range
	RegistryNodePort uint16
	InjectorNodePort uint16
}

// Validate ensures that required stuc fields are populated with expected values
// Required fields
// - ImagesDir, path to folder containing the images
// - PkgName, name of the package used as a label selector by the pod
// - Architecture, used to schedule the injector only on a node of the right cpu architecture
// Non-required fields
// - InjectorSeedSrcs, tbd
// - RegistryNodePort, with using uint16 allows for only the valid ports, this includes 0 as it will allow Kubernetes to choose the node port for us
// - InjectorNodePort, with using uint16 allows for only the valid ports, this includes 0 as it will allow Kubernetes to choose the node port for us
func (i *ZarfInjectorOptions) Validate() error {
	if i.ImagesDir == "" {
		return fmt.Errorf("a path to the image directory must be provided")
	}

	if i.PkgName == "" {
		return fmt.Errorf("a package name is required by the injector")
	}

	if i.Architecture == "" {
		return fmt.Errorf("an architecture must be provided")
	}

	return nil
}

// StartInjection initializes a Zarf injection into the cluster
func (c *Cluster) StartInjection(ctx context.Context, tmpDir string, opts ZarfInjectorOptions) (int, error) {
	l := logger.From(ctx)
	start := time.Now()

	err := opts.Validate()
	if err != nil {
		return 0, err
	}

	// The injector breaks if the same image is added multiple times
	opts.InjectorSeedSrcs = helpers.Unique(opts.InjectorSeedSrcs)

	// Stop any previous running injection before starting.
	err = c.StopInjection(ctx)
	if err != nil {
		return 0, err
	}

	l.Info("creating Zarf injector resources")

	svc, err := c.createInjectorNodeportService(ctx, opts)
	if err != nil {
		return 0, err
	}

	payloadCmNames, shasum, err := c.CreateInjectorConfigMaps(ctx, tmpDir, opts)
	if err != nil {
		return 0, err
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
	injectorImage, injectorNodeName, err := c.getInjectorImageAndNode(ctx, resReq, opts)
	if err != nil {
		return 0, err
	}

	pod := buildInjectionPod(injectorNodeName, injectorImage, payloadCmNames, shasum, resReq, opts)
	_, err = c.Clientset.CoreV1().Pods(*pod.Namespace).Apply(ctx, pod, metav1.ApplyOptions{Force: true, FieldManager: FieldManagerName})
	if err != nil {
		return 0, fmt.Errorf("error creating pod in cluster: %w", err)
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
		return 0, err
	}

	l.Debug("done with injection", "duration", time.Since(start))
	return int(svc.Spec.Ports[0].NodePort), nil
}

// CreateInjectorConfigMaps creates the required configmaps to run the injector
func (c *Cluster) CreateInjectorConfigMaps(ctx context.Context, tmpDir string, opts ZarfInjectorOptions) ([]string, string, error) {
	err := opts.Validate()
	if err != nil {
		return nil, "", err
	}

	payloadCmNames, shasum, err := c.createPayloadConfigMaps(ctx, tmpDir, opts)
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
			PackageLabel: opts.PkgName,
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
	err = c.Clientset.CoreV1().ConfigMaps(state.ZarfNamespaceName).Delete(ctx, "rust-binary", metav1.DeleteOptions{})
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

func (c *Cluster) createPayloadConfigMaps(ctx context.Context, tmpDir string, opts ZarfInjectorOptions) ([]string, string, error) {
	l := logger.From(ctx)
	tarPath := filepath.Join(tmpDir, "payload.tar.gz")
	seedImagesDir := filepath.Join(tmpDir, "seed-images")
	if err := helpers.CreateDirectory(seedImagesDir, helpers.ReadWriteExecuteUser); err != nil {
		return nil, "", fmt.Errorf("unable to create the seed images directory: %w", err)
	}

	localReferenceToDigest := map[string]string{}
	for _, src := range opts.InjectorSeedSrcs {
		ref, err := transform.ParseImageRef(src)
		if err != nil {
			return nil, "", fmt.Errorf("failed to create ref for image %s: %w", src, err)
		}
		img, err := utils.LoadOCIImage(opts.ImagesDir, ref)
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
				PackageLabel:    opts.PkgName,
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
func (c *Cluster) getInjectorImageAndNode(ctx context.Context, resReq *v1ac.ResourceRequirementsApplyConfiguration, opts ZarfInjectorOptions) (string, string, error) {
	l := logger.From(ctx)

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

	// Evaluate nodes one by one, return early when suitable
	for _, node := range nodeList.Items {
		if hasBlockingTaints(node.Spec.Taints) {
			l.Debug("skipping node: blocking taints", "node", node.Name)
			continue
		}

		if node.Status.NodeInfo.Architecture != "" && node.Status.NodeInfo.Architecture != opts.Architecture {
			continue
		}

		availCPU := node.Status.Allocatable.Cpu().DeepCopy()
		availMem := node.Status.Allocatable.Memory().DeepCopy()
		var candidateImage string

		for _, pod := range podsByNode[node.Name] {
			podReqs := componenthelpers.AggregateContainerRequests(&pod, componenthelpers.PodResourcesOptions{})
			if cpuReq := podReqs.Cpu(); cpuReq != nil {
				availCPU.Sub(*cpuReq)
			}
			if memReq := podReqs.Memory(); memReq != nil {
				availMem.Sub(*memReq)
			}

			// Collect candidate images (containers, init, ephemeral)
			for _, ctn := range pod.Spec.Containers {
				if candidateImage == "" && !zarfImageRegex.MatchString(ctn.Image) {
					candidateImage = ctn.Image
				}
			}
			for _, ctn := range pod.Spec.InitContainers {
				if candidateImage == "" && !zarfImageRegex.MatchString(ctn.Image) {
					candidateImage = ctn.Image
				}
			}
			for _, ctn := range pod.Spec.EphemeralContainers {
				if candidateImage == "" && !zarfImageRegex.MatchString(ctn.Image) {
					candidateImage = ctn.Image
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
				"requiredCPU", resReq.Requests.Cpu().String(),
				"requiredMem", resReq.Requests.Memory().String(),
				"availCPU", availCPU.String(),
				"availMem", availMem.String(),
			)
			continue
		}

		if candidateImage != "" {
			l.Debug("selected image for injector", "node", node.Name, "image", candidateImage)
			return candidateImage, node.Name, nil
		}

		l.Debug("no suitable image found on node", "node", node.Name)
	}

	return "", "", fmt.Errorf("no suitable injector image or node exists")
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

func buildInjectionPod(nodeName, image string, payloadCmNames []string, shasum string, resReq *v1ac.ResourceRequirementsApplyConfiguration, opts ZarfInjectorOptions) *v1ac.PodApplyConfiguration {
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
			PackageLabel: opts.PkgName,
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
				WithNodeSelector(map[string]string{
					"kubernetes.io/arch": opts.Architecture,
				}).
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

// createInjectorNodeportService creates the injector service on an available port different than the registryNodePort service
func (c *Cluster) createInjectorNodeportService(ctx context.Context, opts ZarfInjectorOptions) (*corev1.Service, error) {
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
			PackageLabel: opts.PkgName,
		})

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
