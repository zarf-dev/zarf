// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package injector runs the injector
package injector

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/internal/healthchecks"
	"github.com/zarf-dev/zarf/src/internal/packager/images"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/state"
	v1ac "k8s.io/client-go/applyconfigurations/core/v1"
)

// StartInjection initializes a Zarf injection into the cluster.
func StartInjection(ctx context.Context, tmpDir string, pCfg images.PushConfig, registryNodePort int, pkgName string) error {
	l := logger.From(ctx)
	start := time.Now()
	// Stop any previous running injection before starting.
	err := StopInjection(ctx, pCfg.Cluster)
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
	injectorImage, injectorNodeName, err := getInjectorImageAndNode(ctx, pCfg.Cluster, resReq)
	if err != nil {
		return err
	}

	b, err := os.ReadFile(filepath.Join(tmpDir, "zarf-injector"))
	if err != nil {
		return err
	}
	cm := v1ac.ConfigMap("rust-binary", state.ZarfNamespaceName).
		WithBinaryData(map[string][]byte{
			"zarf-injector": b,
		}).
		WithLabels(map[string]string{
			cluster.PackageLabel: pkgName,
		})
	_, err = pCfg.Cluster.Clientset.CoreV1().ConfigMaps(*cm.Namespace).Apply(ctx, cm, metav1.ApplyOptions{Force: true, FieldManager: cluster.FieldManagerName})
	if err != nil {
		return err
	}

	svc, err := createInjectorNodeportService(ctx, pCfg.Cluster, registryNodePort, pkgName)
	if err != nil {
		return err
	}
	// TODO: Remove use of passing data through global variables.
	config.ZarfSeedPort = fmt.Sprintf("%d", svc.Spec.Ports[0].NodePort)

	pod := buildInjectionPod(injectorNodeName, injectorImage, resReq, pkgName)
	_, err = pCfg.Cluster.Clientset.CoreV1().Pods(*pod.Namespace).Apply(ctx, pod, metav1.ApplyOptions{Force: true, FieldManager: cluster.FieldManagerName})
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
	err = healthchecks.Run(waitCtx, pCfg.Cluster.Watcher, []v1alpha1.NamespacedObjectKindReference{podRef})
	if err != nil {
		return err
	}

	pCfg.RegistryInfo = state.RegistryInfo{
		Address: fmt.Sprintf("http://%s.%s.svc.cluster.local:5000", svc.Name, svc.Namespace),
	}
	err = images.Push(ctx, pCfg)
	if err != nil {
		return err
	}

	l.Debug("done with injection", "duration", time.Since(start))
	return nil
}

// StopInjection handles cleanup once the seed registry is up.
func StopInjection(ctx context.Context, c *cluster.Cluster) error {
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

// getImagesAndNodesForInjection checks for images on schedulable nodes within a cluster.
func getInjectorImageAndNode(ctx context.Context, c *cluster.Cluster, resReq *v1ac.ResourceRequirementsApplyConfiguration) (string, string, error) {
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

func buildInjectionPod(nodeName, image string, resReq *v1ac.ResourceRequirementsApplyConfiguration, pkgName string) *v1ac.PodApplyConfiguration {
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

	pod := v1ac.Pod("injector", state.ZarfNamespaceName).
		WithLabels(map[string]string{
			"app":                "zarf-injector",
			cluster.AgentLabel:   "ignore",
			cluster.PackageLabel: pkgName,
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
						WithCommand("/zarf-init/zarf-injector").
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
func createInjectorNodeportService(ctx context.Context, c *cluster.Cluster, registryNodePort int, pkgName string) (*corev1.Service, error) {
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
			})).WithLabels(map[string]string{
			cluster.PackageLabel: pkgName,
		})

		var err error
		svc, err = c.Clientset.CoreV1().Services(*svcAc.Namespace).Apply(ctx, svcAc, metav1.ApplyOptions{Force: true, FieldManager: cluster.FieldManagerName})
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
