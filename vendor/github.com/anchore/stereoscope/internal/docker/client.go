package docker

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/docker/cli/cli/connhelper"
	"github.com/moby/moby/client"

	"github.com/anchore/go-homedir"
)

func GetClient() (*client.Client, error) {
	var clientOpts = []client.Opt{
		client.FromEnv,
	}

	host := os.Getenv("DOCKER_HOST")
	if strings.HasPrefix(host, "ssh") {
		helper, err := connhelper.GetConnectionHelper(host)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch docker connection helper: %w", err)
		}
		clientOpts = append(clientOpts,
			client.WithHTTPClient(&http.Client{
				Transport: &http.Transport{
					DialContext: helper.Dialer,
				},
			}),
			client.WithHost(helper.Host),
			client.WithDialContext(helper.Dialer),
		)
	}

	if os.Getenv("DOCKER_TLS_VERIFY") != "" && os.Getenv("DOCKER_CERT_PATH") == "" {
		err := os.Setenv("DOCKER_CERT_PATH", "~/.docker")
		if err != nil {
			return nil, fmt.Errorf("failed create docker client: %w", err)
		}
	}

	possibleSocketPaths := possibleSocketPaths(runtime.GOOS)
	for _, socketPath := range possibleSocketPaths {
		dockerClient, err := newClient(socketPath, clientOpts...)
		if err == nil {
			err := checkConnection(dockerClient)
			if err == nil {
				return dockerClient, nil // Successfully connected
			}
		}
	}

	// If both attempts failed
	return nil, fmt.Errorf("failed to connect to Docker daemon. Ensure Docker is running and accessible")
}

func checkConnection(dockerClient *client.Client) error {
	ctx := context.Background()
	_, err := dockerClient.Ping(ctx, client.PingOptions{})
	if err != nil {
		return fmt.Errorf("failed to ping Docker daemon: %w", err)
	}
	return nil
}

func newClient(socket string, opts ...client.Opt) (*client.Client, error) {
	if socket != "" {
		opts = append(opts, client.WithHost(socket))
	}
	return client.New(opts...)
}

func possibleSocketPaths(os string) []string {
	switch os {
	case "darwin":
		hDir, err := homedir.Dir()
		if err != nil {
			return []string{""}
		}
		return []string{
			"", // try the client default first
			fmt.Sprintf("unix://%s/Library/Containers/com.docker.docker/Data/docker.raw.sock", hDir),
		}
	default:
		return []string{""} // try the client default first
	}
}
