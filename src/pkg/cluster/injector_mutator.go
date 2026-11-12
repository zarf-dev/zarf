// SPDX-License-Identifier: Apache-2.0
// Zarf is a Defense Unicorns project.

package cluster

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/zarf-dev/zarf/src/pkg/state"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/yaml"
	v1ac "k8s.io/client-go/applyconfigurations/core/v1"
)

const injectionTemplateFileName = "injector-manifests.yaml"

// loadInjectionTemplates reads and parses the injector-manifests.yaml template file in dir if it exists.
func loadInjectionTemplates(dir string) (*corev1.Pod, *corev1.Service, error) {
	manifestPath := filepath.Join(dir, injectionTemplateFileName)
	b, err := os.ReadFile(manifestPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("unable to read injector template manifest: %w", err)
	}
	return parseInjectionTemplates(b)
}

// parseInjectionTemplates parses multi-document YAML or JSON manifest bytes into a Pod and Service template.
func parseInjectionTemplates(manifestBytes []byte) (*corev1.Pod, *corev1.Service, error) {
	if len(manifestBytes) == 0 {
		return nil, nil, nil
	}
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(manifestBytes), 4096)
	var (
		pod     *corev1.Pod
		service *corev1.Service
	)
	for {
		var raw map[string]any
		err := decoder.Decode(&raw)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, nil, fmt.Errorf("failed to decode injector manifest template: %w", err)
		}
		if len(raw) == 0 {
			continue
		}
		kind, ok := raw["kind"].(string)
		if !ok {
			return nil, nil, fmt.Errorf("injector template object missing or invalid 'kind'")
		}
		rawJSON, err := json.Marshal(raw)
		if err != nil {
			return nil, nil, err
		}
		switch kind {
		case "Pod":
			if pod != nil {
				return nil, nil, fmt.Errorf("injector template must contain at most one Pod")
			}
			pod = &corev1.Pod{}
			if err := json.Unmarshal(rawJSON, pod); err != nil {
				return nil, nil, fmt.Errorf("unable to unmarshal Pod template: %w", err)
			}
		case "Service":
			if service != nil {
				return nil, nil, fmt.Errorf("injector template must contain at most one Service")
			}
			service = &corev1.Service{}
			if err := json.Unmarshal(rawJSON, service); err != nil {
				return nil, nil, fmt.Errorf("unable to unmarshal Service template: %w", err)
			}
		default:
			return nil, nil, fmt.Errorf("injector template contains unsupported kind %q", kind)
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

// injectionTemplateResources extracts resource requests and limits from an injector container if specified.
func injectionTemplateResources(pod *corev1.Pod, fallback *v1ac.ResourceRequirementsApplyConfiguration) *v1ac.ResourceRequirementsApplyConfiguration {
	if pod == nil {
		return fallback
	}
	for _, container := range pod.Spec.Containers {
		if container.Name == "injector" && (len(container.Resources.Requests) > 0 || len(container.Resources.Limits) > 0) {
			requests := corev1.ResourceList{}
			limits := corev1.ResourceList{}
			if fallback != nil && fallback.Requests != nil {
				requests = fallback.Requests.DeepCopy()
			}
			if fallback != nil && fallback.Limits != nil {
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

// mergeInjectionPodTemplate merges a Pod template onto a base PodApplyConfiguration using StrategicMergePatch.
func mergeInjectionPodTemplate(base *v1ac.PodApplyConfiguration, template *corev1.Pod) (*v1ac.PodApplyConfiguration, error) {
	if template == nil {
		return base, nil
	}
	templateCopy := template.DeepCopy()
	if len(templateCopy.Spec.Containers) == 0 {
		templateCopy.Spec.Containers = []corev1.Container{{Name: "injector"}}
	}
	baseJSON, err := json.Marshal(base)
	if err != nil {
		return nil, err
	}
	templateJSON, err := json.Marshal(templateCopy)
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

// mergeInjectorRuntimePodFields enforces critical runtime invariants that must not be stripped by templates.
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
	for _, volume := range runtime {
		result = append(result, volume)
	}
	return result
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
	for _, mount := range runtime {
		result = append(result, mount)
	}
	return result
}

func containsVolumeMount(mounts []v1ac.VolumeMountApplyConfiguration, name string, mountPath *string) bool {
	for _, mount := range mounts {
		if (mount.Name != nil && *mount.Name == name) || (mountPath != nil && mount.MountPath != nil && *mount.MountPath == *mountPath) {
			return true
		}
	}
	return false
}

// mergeInjectionServiceTemplate merges a Service template onto a base ServiceApplyConfiguration.
func mergeInjectionServiceTemplate(base *v1ac.ServiceApplyConfiguration, template *corev1.Service, opts ZarfInjectorOptions, pkgName string) (*v1ac.ServiceApplyConfiguration, error) {
	if template == nil {
		return base, nil
	}
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
