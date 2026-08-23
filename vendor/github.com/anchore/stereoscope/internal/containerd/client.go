package containerd

import (
	"fmt"
	"os"

	"github.com/adrg/xdg"
	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/defaults"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/spf13/afero"

	"github.com/anchore/stereoscope/internal/log"
)

var ErrNoSocketAddress = fmt.Errorf("no socket address")

func GetClient() (*client.Client, error) {
	client, err := client.New(Address())
	if err != nil {
		return nil, err
	}
	return client, nil
}

func Address() string {
	address, err := getAddress(afero.NewOsFs(), xdg.RuntimeDir, defaults.DefaultAddress)
	if err != nil {
		return ""
	}
	return address
}

func Namespace() string {
	namespace := os.Getenv("CONTAINERD_NAMESPACE")
	if namespace == "" {
		namespace = namespaces.Default
	}

	return namespace
}

func getAddress(fs afero.Fs, xdgRuntimeDir, defaultSocketPath string) (string, error) {
	var addr string
	if v, found := os.LookupEnv("CONTAINERD_ADDRESS"); found && v != "" {
		addr = v
	}

	if addr != "" {
		return addr, nil
	}

	candidateAddresses := []string{
		// default rootless address
		rootlessSocketPath(fs, xdgRuntimeDir),

		// typically accessible to only root, but last ditch effort
		defaultSocketPath,
	}

	for _, candidate := range candidateAddresses {
		if candidate == "" {
			continue
		}
		log.WithFields("path", candidate).Trace("trying containerd socket")
		_, err := fs.Stat(candidate)
		if err == nil {
			addr = candidate
			break
		}
	}

	if addr == "" {
		return "", ErrNoSocketAddress
	}

	return addr, nil
}

func rootlessSocketPath(fs afero.Fs, xdgRuntimeDir string) string {
	// look for rootless address (fallback to default if not found)
	//export CONTAINERD_ADDRESS=/proc/$(cat $XDG_RUNTIME_DIR/containerd-rootless/child_pid)/root/run/containerd/containerd.sock

	p := fmt.Sprintf("%s/containerd-rootless/child_pid", xdgRuntimeDir)
	if _, err := fs.Stat(p); err != nil {
		return ""
	}

	by, err := afero.ReadFile(fs, p)
	if err != nil {
		return ""
	}

	if len(by) == 0 {
		return ""
	}

	return fmt.Sprintf("/proc/%s/root/run/containerd/containerd.sock", string(by))
}
