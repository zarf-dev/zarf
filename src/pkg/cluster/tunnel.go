// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/defenseunicorns/pkg/helpers"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/k8s"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
)

// Zarf specific connect strings
const (
	ZarfRegistry = "REGISTRY"
	ZarfLogging  = "LOGGING"
	ZarfGit      = "GIT"
	ZarfInjector = "INJECTOR"

	ZarfInjectorName  = "zarf-injector"
	ZarfInjectorPort  = 5000
	ZarfRegistryName  = "zarf-docker-registry"
	ZarfRegistryPort  = 5000
	ZarfGitServerName = "zarf-gitea-http"
	ZarfGitServerPort = 3000
)

// TunnelInfo is a struct that contains the necessary info to create a new k8s.Tunnel
type TunnelInfo struct {
	localPort    int
	remotePort   int
	namespace    string
	resourceType string
	resourceName string
	urlSuffix    string
}

// NewTunnelInfo returns a new TunnelInfo object for connecting to a cluster
func NewTunnelInfo(namespace, resourceType, resourceName, urlSuffix string, localPort, remotePort int) TunnelInfo {
	return TunnelInfo{
		namespace:    namespace,
		resourceType: resourceType,
		resourceName: resourceName,
		urlSuffix:    urlSuffix,
		localPort:    localPort,
		remotePort:   remotePort,
	}
}

// PrintConnectTable will print a table of all Zarf connect matches found in the cluster.
func (c *Cluster) PrintConnectTable(ctx context.Context) error {
	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{{
			Operator: metav1.LabelSelectorOpExists,
			Key:      config.ZarfConnectLabelName,
		}},
	})
	if err != nil {
		return err
	}
	serviceList, err := c.Clientset.CoreV1().Services("").List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return err
	}

	connections := make(types.ConnectStrings)
	for _, svc := range serviceList.Items {
		name := svc.Labels[config.ZarfConnectLabelName]
		// Add the connectString for processing later in the deployment.
		connections[name] = types.ConnectString{
			Description: svc.Annotations[config.ZarfConnectAnnotationDescription],
			URL:         svc.Annotations[config.ZarfConnectAnnotationURL],
		}
	}
	message.PrintConnectStringTable(connections)
	return nil
}

// Connect will establish a tunnel to the specified target.
func (c *Cluster) Connect(ctx context.Context, target string) (*k8s.Tunnel, error) {
	var err error
	zt := TunnelInfo{
		namespace:    ZarfNamespaceName,
		resourceType: k8s.SvcResource,
	}

	switch strings.ToUpper(target) {
	case ZarfRegistry:
		zt.resourceName = ZarfRegistryName
		zt.remotePort = ZarfRegistryPort
		zt.urlSuffix = `/v2/_catalog`

	case ZarfLogging:
		zt.resourceName = "zarf-loki-stack-grafana"
		zt.remotePort = 3000
		// Start the logs with something useful.
		zt.urlSuffix = `/monitor/explore?orgId=1&left=%5B"now-12h","now","Loki",%7B"refId":"Zarf%20Logs","expr":"%7Bnamespace%3D%5C"zarf%5C"%7D"%7D%5D`

	case ZarfGit:
		zt.resourceName = ZarfGitServerName
		zt.remotePort = ZarfGitServerPort

	case ZarfInjector:
		zt.resourceName = ZarfInjectorName
		zt.remotePort = ZarfInjectorPort

	default:
		if target != "" {
			if zt, err = c.checkForZarfConnectLabel(ctx, target); err != nil {
				return nil, fmt.Errorf("problem looking for a zarf connect label in the cluster: %s", err.Error())
			}
		}

		if zt.resourceName == "" {
			return nil, fmt.Errorf("missing resource name")
		}
		if zt.remotePort < 1 {
			return nil, fmt.Errorf("missing remote port")
		}
	}

	return c.ConnectTunnelInfo(ctx, zt)
}

// ConnectTunnelInfo connects to the cluster with the provided TunnelInfo
func (c *Cluster) ConnectTunnelInfo(ctx context.Context, zt TunnelInfo) (*k8s.Tunnel, error) {
	tunnel, err := c.NewTunnel(zt.namespace, zt.resourceType, zt.resourceName, zt.urlSuffix, zt.localPort, zt.remotePort)
	if err != nil {
		return nil, err
	}

	_, err = tunnel.Connect(ctx)
	if err != nil {
		return nil, err
	}

	return tunnel, nil
}

// ConnectToZarfRegistryEndpoint determines if a registry endpoint is in cluster, and if so opens a tunnel to connect to it
func (c *Cluster) ConnectToZarfRegistryEndpoint(ctx context.Context, registryInfo types.RegistryInfo) (string, *k8s.Tunnel, error) {
	registryEndpoint := registryInfo.Address

	var err error
	var tunnel *k8s.Tunnel
	if registryInfo.InternalRegistry {
		// Establish a registry tunnel to send the images to the zarf registry
		if tunnel, err = c.NewTunnel(ZarfNamespaceName, k8s.SvcResource, ZarfRegistryName, "", 0, ZarfRegistryPort); err != nil {
			return "", tunnel, err
		}
	} else {
		serviceList, err := c.Clientset.CoreV1().Services("").List(ctx, metav1.ListOptions{})
		if err != nil {
			return "", nil, err
		}
		namespace, name, _, port, err := serviceInfoFromNodePortURL(serviceList.Items, registryInfo.Address)

		// If this is a service (no error getting svcInfo), create a port-forward tunnel to that resource
		if err == nil {
			if tunnel, err = c.NewTunnel(namespace, k8s.SvcResource, name, "", 0, port); err != nil {
				return "", tunnel, err
			}
		}
	}

	if tunnel != nil {
		_, err = tunnel.Connect(ctx)
		if err != nil {
			return "", tunnel, err
		}
		registryEndpoint = tunnel.Endpoint()
	}

	return registryEndpoint, tunnel, nil
}

// checkForZarfConnectLabel looks in the cluster for a connect name that matches the target
func (c *Cluster) checkForZarfConnectLabel(ctx context.Context, name string) (TunnelInfo, error) {
	var err error
	var zt TunnelInfo

	message.Debugf("Looking for a Zarf Connect Label in the cluster")

	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{
			config.ZarfConnectLabelName: name,
		},
	})
	if err != nil {
		return TunnelInfo{}, err
	}
	listOpts := metav1.ListOptions{LabelSelector: selector.String()}
	serviceList, err := c.Clientset.CoreV1().Services("").List(ctx, listOpts)
	if err != nil {
		return TunnelInfo{}, err
	}

	if len(serviceList.Items) > 0 {
		// If there is a match, use the first one as these are supposed to be unique.
		svc := serviceList.Items[0]

		// Reset based on the matched params.
		zt.resourceType = k8s.SvcResource
		zt.resourceName = svc.Name
		zt.namespace = svc.Namespace
		// Only support a service with a single port.
		zt.remotePort = svc.Spec.Ports[0].TargetPort.IntValue()
		// if targetPort == 0, look for Port (which is required)
		if zt.remotePort == 0 {
			zt.remotePort, err = c.FindPodContainerPort(ctx, svc)
			if err != nil {
				return TunnelInfo{}, err
			}
		}

		// Add the url suffix too.
		zt.urlSuffix = svc.Annotations[config.ZarfConnectAnnotationURL]

		message.Debugf("tunnel connection match: %s/%s on port %d", svc.Namespace, svc.Name, zt.remotePort)
	} else {
		return zt, fmt.Errorf("no matching services found for %s", name)
	}

	return zt, nil
}

// TODO: Refactor to use netip.AddrPort instead of a string for nodePortURL.
func serviceInfoFromNodePortURL(services []corev1.Service, nodePortURL string) (string, string, string, int, error) {
	// Attempt to parse as normal, if this fails add a scheme to the URL (docker registries don't use schemes)
	parsedURL, err := url.Parse(nodePortURL)
	if err != nil {
		parsedURL, err = url.Parse("scheme://" + nodePortURL)
		if err != nil {
			return "", "", "", 0, err
		}
	}

	// Match hostname against localhost ip/hostnames
	hostname := parsedURL.Hostname()
	if hostname != helpers.IPV4Localhost && hostname != "localhost" {
		return "", "", "", 0, fmt.Errorf("node port services should be on localhost")
	}

	// Get the node port from the nodeportURL.
	nodePort, err := strconv.Atoi(parsedURL.Port())
	if err != nil {
		return "", "", "", 0, err
	}
	if nodePort < 30000 || nodePort > 32767 {
		return "", "", "", 0, fmt.Errorf("node port services should use the port range 30000-32767")
	}

	for _, svc := range services {
		if svc.Spec.Type == "NodePort" {
			for _, port := range svc.Spec.Ports {
				if int(port.NodePort) == nodePort {
					return svc.Namespace, svc.Name, svc.Spec.ClusterIP, int(port.Port), nil
				}
			}
		}
	}

	return "", "", "", 0, fmt.Errorf("no matching node port services found")
}
