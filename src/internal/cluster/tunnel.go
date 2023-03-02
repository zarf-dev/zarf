// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

// Forked from https://github.com/gruntwork-io/terratest/blob/v0.38.8/modules/k8s/tunnel.go

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/defenseunicorns/zarf/src/types"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/k8s"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/exec"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// Global lock to synchronize port selections.
var globalMutex sync.Mutex

// Zarf Tunnel Configuration Constants.
const (
	PodResource  = "pod"
	SvcResource  = "svc"
	ZarfRegistry = "REGISTRY"
	ZarfLogging  = "LOGGING"
	ZarfGit      = "GIT"
	ZarfInjector = "INJECTOR"

	// See https://regex101.com/r/OWVfAO/1.
	serviceURLPattern = `^(?P<name>[^\.]+)\.(?P<namespace>[^\.]+)\.svc\.cluster\.local$`
)

// Tunnel is the main struct that configures and manages port forwarding tunnels to Kubernetes resources.
type Tunnel struct {
	kube         *k8s.K8s
	out          io.Writer
	autoOpen     bool
	localPort    int
	remotePort   int
	namespace    string
	resourceType string
	resourceName string
	urlSuffix    string
	attempt      int
	stopChan     chan struct{}
	readyChan    chan struct{}
	spinner      *message.Spinner
}

// ServiceInfo contains information necessary for connecting to a cluster service.
type ServiceInfo struct {
	Namespace string
	Name      string
	Port      int
}

// PrintConnectTable will print a table of all Zarf connect matches found in the cluster.
func (c *Cluster) PrintConnectTable() error {
	list, err := c.Kube.GetServicesByLabelExists(v1.NamespaceAll, config.ZarfConnectLabelName)
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

// ServiceInfoFromNodePortURL takes a nodePortURL and parses it to find the service info for connecting to the cluster. The string is expected to follow the following format:
// Example nodePortURL: 127.0.0.1:{PORT}.
func ServiceInfoFromNodePortURL(nodePortURL string) (*ServiceInfo, error) {
	// Attempt to parse as normal, if this fails add a scheme to the URL (docker registries don't use schemes)
	parsedURL, err := url.Parse(nodePortURL)
	if err != nil {
		parsedURL, err = url.Parse("scheme://" + nodePortURL)
		if err != nil {
			return nil, err
		}
	}

	// Match hostname against localhost ip/hostnames
	hostname := parsedURL.Hostname()
	if hostname != config.IPV4Localhost && hostname != "localhost" {
		return nil, fmt.Errorf("node port services should be on localhost")
	}

	// Get the node port from the nodeportURL.
	nodePort, err := strconv.Atoi(parsedURL.Port())
	if err != nil {
		return nil, err
	}
	if nodePort < 30000 || nodePort > 32767 {
		return nil, fmt.Errorf("node port services should use the port range 30000-32767")
	}

	kube, err := k8s.NewWithWait(message.Debugf, labels, defaultTimeout)
	if err != nil {
		return nil, err
	}

	services, err := kube.GetServices("")
	if err != nil {
		return nil, err
	}

	for _, svc := range services.Items {
		if svc.Spec.Type == "NodePort" {
			for _, port := range svc.Spec.Ports {
				if int(port.NodePort) == nodePort {
					return &ServiceInfo{
						Namespace: svc.Namespace,
						Name:      svc.Name,
						Port:      int(port.Port),
					}, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("no matching node port services found")
}

// ServiceInfoFromServiceURL takes a serviceURL and parses it to find the service info for connecting to the cluster. The string is expected to follow the following format:
// Example serviceURL: http://{SERVICE_NAME}.{NAMESPACE}.svc.cluster.local:{PORT}.
func ServiceInfoFromServiceURL(serviceURL string) (*ServiceInfo, error) {
	parsedURL, err := url.Parse(serviceURL)
	if err != nil {
		return nil, err
	}

	// Get the remote port from the serviceURL.
	remotePort, err := strconv.Atoi(parsedURL.Port())
	if err != nil {
		return nil, err
	}

	// Match hostname against local cluster service format.
	pattern := regexp.MustCompile(serviceURLPattern)
	get, err := utils.MatchRegex(pattern, parsedURL.Hostname())

	// If incomplete match, return an error.
	if err != nil {
		return nil, err
	}

	return &ServiceInfo{
		Namespace: get("namespace"),
		Name:      get("name"),
		Port:      remotePort,
	}, nil
}

// NewTunnel will create a new Tunnel struct.
// Note that if you use 0 for the local port, an open port on the host system
// will be selected automatically, and the Tunnel struct will be updated with the selected port.
func NewTunnel(namespace, resourceType, resourceName string, local, remote int) (*Tunnel, error) {
	message.Debugf("tunnel.NewTunnel(%s, %s, %s, %d, %d)", namespace, resourceType, resourceName, local, remote)

	kube, err := k8s.NewWithWait(message.Debugf, labels, defaultTimeout)
	if err != nil {
		return &Tunnel{}, err
	}

	return &Tunnel{
		out:          io.Discard,
		localPort:    local,
		remotePort:   remote,
		namespace:    namespace,
		resourceType: resourceType,
		resourceName: resourceName,
		stopChan:     make(chan struct{}, 1),
		readyChan:    make(chan struct{}, 1),
		kube:         kube,
	}, nil
}

// NewZarfTunnel will create a new Tunnel struct for the Zarf namespace.
func NewZarfTunnel() (*Tunnel, error) {
	return NewTunnel(ZarfNamespace, SvcResource, "", 0, 0)
}

// EnableAutoOpen will automatically open the established tunnel in the default browser when it is ready.
func (tunnel *Tunnel) EnableAutoOpen() {
	tunnel.autoOpen = true
}

// AddSpinner will add a spinner to the tunnel to show progress.
func (tunnel *Tunnel) AddSpinner(spinner *message.Spinner) {
	tunnel.spinner = spinner
}

// Connect will establish a tunnel to the specified target.
func (tunnel *Tunnel) Connect(target string, blocking bool) error {
	message.Debugf("tunnel.Connect(%s, %#v)", target, blocking)

	switch strings.ToUpper(target) {
	case ZarfRegistry:
		tunnel.resourceName = "zarf-docker-registry"
		tunnel.remotePort = 5000
		tunnel.urlSuffix = `/v2/_catalog`

	case ZarfLogging:
		tunnel.resourceName = "zarf-loki-stack-grafana"
		tunnel.remotePort = 3000
		// Start the logs with something useful.
		tunnel.urlSuffix = `/monitor/explore?orgId=1&left=%5B"now-12h","now","Loki",%7B"refId":"Zarf%20Logs","expr":"%7Bnamespace%3D%5C"zarf%5C"%7D"%7D%5D`

	case ZarfGit:
		tunnel.resourceName = "zarf-gitea-http"
		tunnel.remotePort = 3000

	case ZarfInjector:
		tunnel.resourceName = "zarf-injector"
		tunnel.remotePort = 5000

	default:
		if target != "" {
			if err := tunnel.checkForZarfConnectLabel(target); err != nil {
				message.Errorf(err, "Problem looking for a zarf connect label in the cluster")
			}
		}

		if tunnel.resourceName == "" {
			return fmt.Errorf("missing resource name")
		}
		if tunnel.remotePort < 1 {
			return fmt.Errorf("missing remote port")
		}
	}

	url, err := tunnel.establish()

	// Try to establish the tunnel up to 3 times.
	if err != nil {
		tunnel.attempt++
		// If we have exceeded the number of attempts, exit with an error.
		if tunnel.attempt > 3 {
			return fmt.Errorf("unable to establish tunnel after 3 attempts: %w", err)
		}
		// Otherwise, retry the connection but delay increasing intervals between attempts.
		delay := tunnel.attempt * 10
		message.Debug(err)
		message.Infof("Delay creating tunnel, waiting %d seconds...", delay)
		time.Sleep(time.Duration(delay) * time.Second)
		tunnel.Connect(target, blocking)
	}

	if blocking {
		// Otherwise, if this is blocking it is coming from a user request so try to open the URL, but ignore errors.
		if tunnel.autoOpen {
			if err := exec.LaunchURL(url); err != nil {
				message.Debug(err)
			}
		}

		// Dump the tunnel URL to the console for other tools to use.
		fmt.Print(url)

		// Since this blocking, set the defer now so it closes properly on sigterm.
		defer tunnel.Close()

		// Keep this open until an interrupt signal is received.
		c := make(chan os.Signal)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-c
			os.Exit(0)
		}()

		for {
			runtime.Gosched()
		}
	}

	return nil
}

// Endpoint returns the tunnel ip address and port (i.e. for docker registries)
func (tunnel *Tunnel) Endpoint() string {
	message.Debug("tunnel.Endpoint()")
	return fmt.Sprintf("127.0.0.1:%d", tunnel.localPort)
}

// HTTPEndpoint returns the tunnel endpoint as a HTTP URL string.
func (tunnel *Tunnel) HTTPEndpoint() string {
	message.Debug("tunnel.HTTPEndpoint()")
	return fmt.Sprintf("http://%s", tunnel.Endpoint())
}

// Close disconnects a tunnel connection by closing the StopChan, thereby stopping the goroutine.
func (tunnel *Tunnel) Close() {
	message.Debug("tunnel.Close()")
	close(tunnel.stopChan)
}

func (tunnel *Tunnel) checkForZarfConnectLabel(name string) error {
	message.Debugf("tunnel.checkForZarfConnectLabel(%s)", name)
	var spinner *message.Spinner
	var err error

	spinnerMessage := "Looking for a Zarf Connect Label in the cluster"
	if tunnel.spinner != nil {
		spinner = tunnel.spinner
		spinner.Updatef(spinnerMessage)
	} else {
		spinner = message.NewProgressSpinner(spinnerMessage)
		defer spinner.Stop()
	}

	matches, err := tunnel.kube.GetServicesByLabel("", config.ZarfConnectLabelName, name)
	if err != nil {
		return fmt.Errorf("unable to lookup the service: %w", err)
	}

	if len(matches.Items) > 0 {
		// If there is a match, use the first one as these are supposed to be unique.
		svc := matches.Items[0]

		// Reset based on the matched params.
		tunnel.resourceType = SvcResource
		tunnel.resourceName = svc.Name
		tunnel.namespace = svc.Namespace
		// Only support a service with a single port.
		tunnel.remotePort = svc.Spec.Ports[0].TargetPort.IntValue()

		// Add the url suffix too.
		tunnel.urlSuffix = svc.Annotations[config.ZarfConnectAnnotationURL]

		message.Debugf("tunnel connection match: %s/%s on port %d", svc.Namespace, svc.Name, tunnel.remotePort)
	}

	return nil
}

// establish opens a tunnel to a kubernetes resource, as specified by the provided tunnel struct.
func (tunnel *Tunnel) establish() (string, error) {
	message.Debug("tunnel.Establish()")

	var err error
	var spinner *message.Spinner

	// Track this locally as we may need to retry if the tunnel fails.
	localPort := tunnel.localPort

	// If the local-port is 0, get an available port before continuing. We do this here instead of relying on the
	// underlying port-forwarder library, because the port-forwarder library does not expose the selected local port in a
	// machine-readable manner.
	// Synchronize on the global lock to avoid race conditions with concurrently selecting the same available port,
	// since there is a brief moment between `GetAvailablePort` and `forwarder.ForwardPorts` where the selected port
	// is available for selection again.
	if localPort == 0 {
		message.Debugf("Requested local port is 0. Selecting an open port on host system")
		localPort, err = utils.GetAvailablePort()
		if err != nil {
			return "", fmt.Errorf("unable to find an available port: %w", err)
		}
		message.Debugf("Selected port %d", localPort)
		globalMutex.Lock()
		defer globalMutex.Unlock()
	}

	spinnerMessage := fmt.Sprintf("Opening tunnel %d -> %d for %s/%s in namespace %s",
		localPort,
		tunnel.remotePort,
		tunnel.resourceType,
		tunnel.resourceName,
		tunnel.namespace,
	)

	if tunnel.spinner != nil {
		spinner = tunnel.spinner
		spinner.Updatef(spinnerMessage)
	} else {
		spinner = message.NewProgressSpinner(spinnerMessage)
		defer spinner.Stop()
	}

	kube, err := k8s.NewWithWait(message.Debugf, labels, defaultTimeout)
	if err != nil {
		return "", fmt.Errorf("unable to connect to the cluster: %w", err)
	}

	// Find the pod to port forward to
	podName, err := tunnel.getAttachablePodForResource()
	if err != nil {
		return "", fmt.Errorf("unable to find pod attached to given resource: %w", err)
	}
	message.Debugf("Selected pod %s to open port forward to", podName)

	// Build url to the port forward endpoint.
	// Example: http://localhost:8080/api/v1/namespaces/helm/pods/tiller-deploy-9itlq/portforward.
	postEndpoint := tunnel.kube.Clientset.CoreV1().RESTClient().Post()
	namespace := tunnel.namespace
	portForwardCreateURL := postEndpoint.
		Resource("pods").
		Namespace(namespace).
		Name(podName).
		SubResource("portforward").
		URL()

	message.Debugf("Using URL %s to create portforward", portForwardCreateURL)

	// Construct the spdy client required by the client-go portforward library.
	transport, upgrader, err := spdy.RoundTripperFor(kube.RestConfig)
	if err != nil {
		return "", fmt.Errorf("unable to create the spdy client %w", err)
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", portForwardCreateURL)

	// Construct a new PortForwarder struct that manages the instructed port forward tunnel.
	ports := []string{fmt.Sprintf("%d:%d", localPort, tunnel.remotePort)}
	portforwarder, err := portforward.New(dialer, ports, tunnel.stopChan, tunnel.readyChan, tunnel.out, tunnel.out)
	if err != nil {
		return "", fmt.Errorf("unable to create the port forward: %w", err)
	}

	// Open the tunnel in a goroutine so that it is available in the background. Report errors to the main goroutine via
	// a new channel.
	errChan := make(chan error)
	go func() {
		errChan <- portforwarder.ForwardPorts()
	}()

	// Wait for an error or the tunnel to be ready.
	select {
	case err = <-errChan:
		if tunnel.spinner == nil {
			spinner.Stop()
		}
		return "", fmt.Errorf("unable to start the tunnel: %w", err)
	case <-portforwarder.Ready:
		// Store for endpoint output
		tunnel.localPort = localPort
		url := fmt.Sprintf("http://%s:%d%s", config.IPV4Localhost, localPort, tunnel.urlSuffix)
		msg := fmt.Sprintf("Creating port forwarding tunnel at %s", url)
		if tunnel.spinner == nil {
			spinner.Successf(msg)
		} else {
			spinner.Updatef(msg)
		}
		return url, nil
	}
}

// getAttachablePodForResource will find a pod that can be port forwarded to the provided resource type and return
// the name.
func (tunnel *Tunnel) getAttachablePodForResource() (string, error) {
	message.Debug("tunnel.getAttachablePodForResource()")
	switch tunnel.resourceType {
	case PodResource:
		return tunnel.resourceName, nil
	case SvcResource:
		return tunnel.getAttachablePodForService()
	default:
		return "", fmt.Errorf("unknown resource type: %s", tunnel.resourceType)
	}
}

// getAttachablePodForServiceE will find an active pod associated with the Service and return the pod name.
func (tunnel *Tunnel) getAttachablePodForService() (string, error) {
	message.Debug("tunnel.getAttachablePodForService()")
	service, err := tunnel.kube.GetService(tunnel.namespace, tunnel.resourceName)
	if err != nil {
		return "", fmt.Errorf("unable to find the service: %w", err)
	}
	selectorLabelsOfPods := makeLabels(service.Spec.Selector)

	servicePods := tunnel.kube.WaitForPodsAndContainers(k8s.PodLookup{
		Namespace: tunnel.namespace,
		Selector:  selectorLabelsOfPods,
	}, nil)

	if len(servicePods) < 1 {
		return "", fmt.Errorf("no pods found for service %s", tunnel.resourceName)
	}

	return servicePods[0], nil
}

// makeLabels is a helper to format a map of label key and value pairs into a single string for use as a selector.
func makeLabels(labels map[string]string) string {
	var out []string
	for key, value := range labels {
		out = append(out, fmt.Sprintf("%s=%s", key, value))
	}
	return strings.Join(out, ",")
}
