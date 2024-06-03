// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package k8s provides a client for interacting with a Kubernetes cluster.
package k8s

// Forked from https://github.com/gruntwork-io/terratest/blob/v0.38.8/modules/k8s/tunnel.go

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"

	"github.com/defenseunicorns/pkg/helpers"
)

// Global lock to synchronize port selections.
var globalMutex sync.Mutex

// Zarf Tunnel Configuration Constants.
const (
	PodResource = "pod"
	SvcResource = "svc"
)

// Tunnel is the main struct that configures and manages port forwarding tunnels to Kubernetes resources.
type Tunnel struct {
	kube         *K8s
	out          io.Writer
	localPort    int
	remotePort   int
	namespace    string
	resourceType string
	resourceName string
	urlSuffix    string
	attempt      int
	stopChan     chan struct{}
	readyChan    chan struct{}
	errChan      chan error
}

// NewTunnel will create a new Tunnel struct.
// Note that if you use 0 for the local port, an open port on the host system
// will be selected automatically, and the Tunnel struct will be updated with the selected port.
func (k *K8s) NewTunnel(namespace, resourceType, resourceName, urlSuffix string, local, remote int) (*Tunnel, error) {
	return &Tunnel{
		out:          io.Discard,
		localPort:    local,
		remotePort:   remote,
		namespace:    namespace,
		resourceType: resourceType,
		resourceName: resourceName,
		urlSuffix:    urlSuffix,
		stopChan:     make(chan struct{}, 1),
		readyChan:    make(chan struct{}, 1),
		kube:         k,
	}, nil
}

// Wrap takes a function that returns an error and wraps it to check for tunnel errors as well.
func (tunnel *Tunnel) Wrap(function func() error) error {
	var err error
	funcErrChan := make(chan error)

	go func() {
		funcErrChan <- function()
	}()

	select {
	case err = <-funcErrChan:
		return err
	case err = <-tunnel.ErrChan():
		return err
	}
}

// Connect will establish a tunnel to the specified target.
func (tunnel *Tunnel) Connect(ctx context.Context) (string, error) {
	url, err := tunnel.establish(ctx)

	// Try to establish the tunnel up to 3 times.
	if err != nil {
		tunnel.attempt++

		// If we have exceeded the number of attempts, exit with an error.
		if tunnel.attempt > 3 {
			return "", fmt.Errorf("unable to establish tunnel after 3 attempts: %w", err)
		}

		// Otherwise, retry the connection but delay increasing intervals between attempts.
		delay := tunnel.attempt * 10
		tunnel.kube.Log("%s", err.Error())
		tunnel.kube.Log("Delay creating tunnel, waiting %d seconds...", delay)

		timer := time.NewTimer(0)
		defer timer.Stop()

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-timer.C:
			url, err = tunnel.Connect(ctx)
			if err != nil {
				return "", err
			}

			timer.Reset(time.Duration(delay) * time.Second)
		}
	}

	return url, nil
}

// Endpoint returns the tunnel ip address and port (i.e. for docker registries)
func (tunnel *Tunnel) Endpoint() string {
	return fmt.Sprintf("%s:%d", helpers.IPV4Localhost, tunnel.localPort)
}

// ErrChan returns the tunnel's error channel
func (tunnel *Tunnel) ErrChan() chan error {
	return tunnel.errChan
}

// HTTPEndpoint returns the tunnel endpoint as a HTTP URL string.
func (tunnel *Tunnel) HTTPEndpoint() string {
	return fmt.Sprintf("http://%s", tunnel.Endpoint())
}

// FullURL returns the tunnel endpoint as a HTTP URL string with the urlSuffix appended.
func (tunnel *Tunnel) FullURL() string {
	return fmt.Sprintf("%s%s", tunnel.HTTPEndpoint(), tunnel.urlSuffix)
}

// Close disconnects a tunnel connection by closing the StopChan, thereby stopping the goroutine.
func (tunnel *Tunnel) Close() {
	close(tunnel.stopChan)
}

// establish opens a tunnel to a kubernetes resource, as specified by the provided tunnel struct.
func (tunnel *Tunnel) establish(ctx context.Context) (string, error) {
	var err error

	// Track this locally as we may need to retry if the tunnel fails.
	localPort := tunnel.localPort

	// If the local-port is 0, get an available port before continuing. We do this here instead of relying on the
	// underlying port-forwarder library, because the port-forwarder library does not expose the selected local port in a
	// machine-readable manner.
	// Synchronize on the global lock to avoid race conditions with concurrently selecting the same available port,
	// since there is a brief moment between `GetAvailablePort` and `forwarder.ForwardPorts` where the selected port
	// is available for selection again.
	if localPort == 0 {
		tunnel.kube.Log("Requested local port is 0. Selecting an open port on host system")
		localPort, err = helpers.GetAvailablePort()
		if err != nil {
			return "", fmt.Errorf("unable to find an available port: %w", err)
		}
		tunnel.kube.Log("Selected port %d", localPort)
		globalMutex.Lock()
		defer globalMutex.Unlock()
	}

	message := fmt.Sprintf("Opening tunnel %d -> %d for %s/%s in namespace %s",
		localPort,
		tunnel.remotePort,
		tunnel.resourceType,
		tunnel.resourceName,
		tunnel.namespace,
	)

	tunnel.kube.Log(message)

	// Find the pod to port forward to
	podName, err := tunnel.getAttachablePodForResource(ctx)
	if err != nil {
		return "", fmt.Errorf("unable to find pod attached to given resource: %w", err)
	}
	tunnel.kube.Log("Selected pod %s to open port forward to", podName)

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

	tunnel.kube.Log("Using URL %s to create portforward", portForwardCreateURL)

	// Construct the spdy client required by the client-go portforward library.
	transport, upgrader, err := spdy.RoundTripperFor(tunnel.kube.RestConfig)
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
		return "", fmt.Errorf("unable to start the tunnel: %w", err)
	case <-portforwarder.Ready:
		// Store for endpoint output
		tunnel.localPort = localPort
		url := tunnel.FullURL()

		// Store the error channel to listen for errors
		tunnel.errChan = errChan

		tunnel.kube.Log("Creating port forwarding tunnel at %s", url)
		return url, nil
	}
}

// getAttachablePodForResource will find a pod that can be port forwarded to the provided resource type and return
// the name.
func (tunnel *Tunnel) getAttachablePodForResource(ctx context.Context) (string, error) {
	switch tunnel.resourceType {
	case PodResource:
		return tunnel.resourceName, nil
	case SvcResource:
		return tunnel.getAttachablePodForService(ctx)
	default:
		return "", fmt.Errorf("unknown resource type: %s", tunnel.resourceType)
	}
}

// getAttachablePodForService will find an active pod associated with the Service and return the pod name.
func (tunnel *Tunnel) getAttachablePodForService(ctx context.Context) (string, error) {
	service, err := tunnel.kube.Clientset.CoreV1().Services(tunnel.namespace).Get(ctx, tunnel.resourceName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("unable to find the service: %w", err)
	}
	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{MatchLabels: service.Spec.Selector})
	if err != nil {
		return "", err
	}

	servicePods := tunnel.kube.WaitForPodsAndContainers(
		ctx,
		PodLookup{
			Namespace: tunnel.namespace,
			Selector:  selector.String(),
		},
		nil,
	)

	if len(servicePods) < 1 {
		return "", fmt.Errorf("no pods found for service %s", tunnel.resourceName)
	}
	return servicePods[0].Name, nil
}
