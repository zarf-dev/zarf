// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cluster

import (
	"context"
	"net"
	"net/url"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/zarf-dev/zarf/src/internal/dns"
	"github.com/zarf-dev/zarf/src/pkg/state"
)

const zarfRegistryProxyName = "zarf-registry-proxy"

// ResolveRegistryMode determines whether address points to the deployed Zarf registry and returns
// the corresponding access mode and host-facing port. Addresses that do not match the Zarf
// registry resources are external.
func (c *Cluster) ResolveRegistryMode(ctx context.Context, address string) (state.RegistryMode, int, error) {
	svc, err := c.Clientset.CoreV1().Services(state.ZarfNamespaceName).Get(ctx, ZarfRegistryName, metav1.GetOptions{})
	if kerrors.IsNotFound(err) {
		return state.RegistryModeExternal, 0, nil
	}
	if err != nil {
		return "", 0, err
	}

	hostname, requestedPort, err := registryAddressHostPort(address)
	if err != nil {
		return state.RegistryModeExternal, 0, nil
	}
	if !dns.IsLocalhost(hostname) {
		return state.RegistryModeExternal, 0, nil
	}

	if svc.Spec.Type == corev1.ServiceTypeNodePort {
		for _, port := range svc.Spec.Ports {
			if int(port.NodePort) == requestedPort {
				return state.RegistryModeNodePort, requestedPort, nil
			}
		}
		return state.RegistryModeExternal, 0, nil
	}

	if svc.Spec.Type != corev1.ServiceTypeClusterIP {
		return state.RegistryModeExternal, 0, nil
	}

	proxy, err := c.Clientset.AppsV1().DaemonSets(state.ZarfNamespaceName).Get(ctx, zarfRegistryProxyName, metav1.GetOptions{})
	if kerrors.IsNotFound(err) {
		return state.RegistryModeExternal, 0, nil
	}
	if err != nil {
		return "", 0, err
	}
	if registryProxyExposesPort(proxy, requestedPort) {
		return state.RegistryModeProxy, requestedPort, nil
	}
	return state.RegistryModeExternal, 0, nil
}

func registryAddressHostPort(address string) (string, int, error) {
	// url.Parse misreads a bare "localhost:31999" as scheme:opaque (empty host), so fall back to
	// the raw string and let net.SplitHostPort do the splitting.
	rawHost := address
	if u, err := url.Parse(address); err == nil && u.Host != "" {
		rawHost = u.Host
	}
	hostname, portString, err := net.SplitHostPort(rawHost)
	if err != nil {
		return "", 0, err
	}
	port, err := strconv.Atoi(portString)
	return hostname, port, err
}

func registryProxyExposesPort(proxy *appsv1.DaemonSet, requestedPort int) bool {
	for _, container := range proxy.Spec.Template.Spec.Containers {
		for _, port := range container.Ports {
			if proxy.Spec.Template.Spec.HostNetwork && int(port.ContainerPort) == requestedPort {
				return true
			}
			if !proxy.Spec.Template.Spec.HostNetwork && int(port.HostPort) == requestedPort &&
				(port.HostIP == "" || dns.IsLocalhost(port.HostIP)) {
				return true
			}
		}
	}
	return false
}
