// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"context"
	"fmt"
	"strings"

	"github.com/defenseunicorns/zarf/src/types"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/k8s"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	v1 "k8s.io/api/core/v1"
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
	list, err := c.GetServicesByLabelExists(ctx, v1.NamespaceAll, config.ZarfConnectLabelName)
	if err != nil {
		return err
	}

	connections := make(types.ConnectStrings)

	for _, svc := range list.Items {
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
		svcInfo, err := c.ServiceInfoFromNodePortURL(ctx, registryInfo.Address)

		// If this is a service (no error getting svcInfo), create a port-forward tunnel to that resource
		if err == nil {
			if tunnel, err = c.NewTunnel(svcInfo.Namespace, k8s.SvcResource, svcInfo.Name, "", 0, svcInfo.Port); err != nil {
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

	matches, err := c.GetServicesByLabel(ctx, "", config.ZarfConnectLabelName, name)
	if err != nil {
		return zt, fmt.Errorf("unable to lookup the service: %w", err)
	}

	if len(matches.Items) > 0 {
		// If there is a match, use the first one as these are supposed to be unique.
		svc := matches.Items[0]

		// Reset based on the matched params.
		zt.resourceType = k8s.SvcResource
		zt.resourceName = svc.Name
		zt.namespace = svc.Namespace
		// Only support a service with a single port.
		zt.remotePort = svc.Spec.Ports[0].TargetPort.IntValue()
		// if targetPort == 0, look for Port (which is required)
		if zt.remotePort == 0 {
			zt.remotePort = c.FindPodContainerPort(ctx, svc)
		}

		// Add the url suffix too.
		zt.urlSuffix = svc.Annotations[config.ZarfConnectAnnotationURL]

		message.Debugf("tunnel connection match: %s/%s on port %d", svc.Namespace, svc.Name, zt.remotePort)
	} else {
		return zt, fmt.Errorf("no matching services found for %s", name)
	}

	return zt, nil
}
