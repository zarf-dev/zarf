package podman

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/adrg/xdg"
	"github.com/moby/moby/client"
	"github.com/spf13/afero"

	"github.com/anchore/stereoscope/internal/log"
)

var (
	ErrNoSocketAddress = errors.New("no socket address")
	ErrNoHostAddress   = errors.New("no host address")
)

const defaultSocketPath = "/run/podman/podman.sock"

func ClientOverSSH() (*client.Client, error) {
	var clientOpts []client.Opt

	host, identity := getSSHAddress(afero.NewOsFs(), configPaths)

	if v, found := os.LookupEnv("CONTAINER_HOST"); found && v != "" {
		host = v
	}

	if v, found := os.LookupEnv("CONTAINER_SSHKEY"); found && v != "" {
		identity = v
	}

	passPhrase := ""
	if v, found := os.LookupEnv("CONTAINER_PASSPHRASE"); found {
		passPhrase = v
	}

	sshConf, err := newSSHConf(host, identity, passPhrase)
	if err != nil {
		return nil, err
	}

	httpClient, err := httpClientOverSSH(sshConf)
	if err != nil {
		return nil, fmt.Errorf("making http client: %w", err)
	}

	clientOpts = append(clientOpts, client.WithHTTPClient(httpClient))

	c, err := client.New(clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed create remote client for podman: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.TODO(), time.Second*3)
	defer cancel()
	_, err = c.Ping(ctx, client.PingOptions{})

	return c, err
}

func ClientOverUnixSocket() (*client.Client, error) {
	var clientOpts []client.Opt

	addr, err := getContainerHostAddress(afero.NewOsFs(), configPaths, xdg.RuntimeDir, defaultSocketPath)
	if err != nil {
		return nil, err
	}

	clientOpts = append(clientOpts, client.WithHost(addr))

	c, err := client.New(clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create podman client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.TODO(), time.Second*3)
	defer cancel()
	_, err = c.Ping(ctx, client.PingOptions{})

	return c, err
}

func getContainerHostAddress(fs afero.Fs, configPaths []string, xdgRuntimeDir, defaultSocketPath string) (string, error) {
	var addr string
	if v, found := os.LookupEnv("CONTAINER_HOST"); found && v != "" {
		addr = v
	} else {
		addr = getUnixSocketAddressFromConfig(fs, configPaths)
	}

	if addr != "" {
		return addr, nil
	}

	// in some cases there might not be any config file, in which case we can try guessing (the same way the podman CLI does)
	candidateAddresses := []string{
		// default rootless address for the podman-system-service
		fmt.Sprintf("%s/podman/podman.sock", xdgRuntimeDir),

		// typically accessible to only root, but last ditch effort
		defaultSocketPath,
	}

	for _, candidate := range candidateAddresses {
		log.WithFields("path", candidate).Trace("trying podman socket")
		_, err := fs.Stat(candidate)
		if err == nil {
			addr = fmt.Sprintf("unix://%s", candidate)
			break
		}
	}

	if addr == "" {
		return "", ErrNoSocketAddress
	}

	return addr, nil
}

func GetClient() (*client.Client, error) {
	c, err := ClientOverUnixSocket()
	if err == nil {
		return c, nil
	}
	log.WithFields("error", err).Trace("unable to connect to podman via unix socket")

	return ClientOverSSH()
}
