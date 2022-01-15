package packager

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/images"
	"github.com/defenseunicorns/zarf/cli/internal/k8s"
	"github.com/defenseunicorns/zarf/cli/internal/message"
	"github.com/defenseunicorns/zarf/cli/internal/message/tls"
	"github.com/defenseunicorns/zarf/cli/internal/pki"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
	"github.com/distribution/distribution/v3/configuration"
	"github.com/distribution/distribution/v3/registry"
	_ "github.com/distribution/distribution/v3/registry/auth/htpasswd"             // used for embedded registry
	_ "github.com/distribution/distribution/v3/registry/storage/driver/filesystem" // used for embedded registry
)

var registryRunning bool

func startEmbeddedRegistry(readOnly bool) {
	message.Debugf("packager.startEmbeddedRegistry(%v)", readOnly)
	if registryRunning {
		// Already running
		return
	}
	registryRunning = true
	useTLS := !config.IsTLSLocalhost()
	registryConfig := &configuration.Configuration{}

	if message.GetLogLevel() >= message.DebugLevel {
		registryConfig.Log.Level = "debug"
	} else {
		registryConfig.Log.AccessLog.Disabled = true
		registryConfig.Log.Formatter = "text"
		registryConfig.Log.Level = "error"
	}

	registryConfig.HTTP.DrainTimeout = 5 * time.Second
	registryConfig.HTTP.Secret = utils.RandomString(20)

	if useTLS {
		registryConfig.HTTP.TLS.Certificate = config.TLS.CertPublicPath
		registryConfig.HTTP.TLS.Key = config.TLS.CertPrivatePath
	}

	fileStorage := configuration.Parameters{
		"rootdirectory": ".zarf-registry",
	}

	if readOnly {
		if useTLS {
			// Bind to any if using tls
			registryConfig.HTTP.Addr = ":" + config.ZarfSeedPort
		} else {
			// otherwise, force localhost
			registryConfig.HTTP.Addr = fmt.Sprintf("%s:%s", config.IPV4Localhost, config.ZarfSeedPort)
		}
		registryConfig.Storage = configuration.Storage{
			"filesystem": fileStorage,
			"maintenance": configuration.Parameters{
				"readonly": map[interface{}]interface{}{
					"enabled": true,
				},
			},
		}
	} else {
		// Read-write only listen on localhost
		registryConfig.HTTP.Addr = config.ZarfSeedRegistry
		registryConfig.Storage = configuration.Storage{
			"filesystem": fileStorage,
		}
	}

	embeddedRegistry, err := registry.NewRegistry(context.Background(), registryConfig)
	if err != nil {
		message.Fatal(err, "Unable to start the embedded registry")
	}

	go func() {
		if err := embeddedRegistry.ListenAndServe(); err != nil {
			message.Fatal(err, "Unable to start the embedded registry")
		}
	}()

}

func preSeedRegistry(tempPath tempPaths) {
	message.Debugf("package.Seed(%v)", tempPath)

	var distro string
	var err error

	state := k8s.LoadZarfState()

	if state.Secret == "" || state.Distro == k8s.DistroIsUnknown {
		message.Debug("New cluster, no zarf state found")

		if deployingK3s {
			distro = k8s.DistroIsK3s
		} else {
			distro, err = k8s.DetectDistro()
			if err != nil {
				message.Fatal(err, "Unable to connect to the k8s cluster to verify the distro")
			}
		}

		message.Debugf("Detected K8s distro %v", distro)

		// Defaults
		state.Registry.NodePort = "31999"
		state.Secret = utils.RandomString(120)
		state.Distro = distro
		state.ZarfAppliance = deployingK3s
	}

	switch state.Distro {
	case k8s.DistroIsK3s:
		state.StorageClass = "local-path"

	case k8s.DistroIsK3d:
		state.StorageClass = "local-path"
		tarballPath, clusterName := prepareImageForImport(tempPath, "k3d")
		// See https://github.com/kubernetes-sigs/kind/blob/v0.11.1/pkg/cluster/internal/kubeconfig/internal/kubeconfig/helpers.go#L24
		if _, err = utils.ExecCommand(true, nil, "k3d", "image", "import", tarballPath, "--cluster", clusterName); err != nil {
			message.Error(err, "Unable to auto-inject the registry image into K3D")
		}

	case k8s.DistroIsKind:
		state.StorageClass = "standard"
		tarballPath, clusterName := prepareImageForImport(tempPath, "kind")
		// See https://github.com/kubernetes-sigs/kind/blob/v0.11.1/pkg/cluster/internal/kubeconfig/internal/kubeconfig/helpers.go#L24
		if _, err = utils.ExecCommand(true, nil, "kind", "load", "image-archive", tarballPath, "--name", clusterName); err != nil {
			message.Error(err, "Unable to auto-inject the registry image into KIND")
		}

	}

	if config.TLS.Host == "" {
		tls.HandleTLSOptions(config.DeployOptions.Confirm)
		pki.HandlePKI()
	}

	// Start embedded registry
	startEmbeddedRegistry(true)

	// Save the state back to K8s
	if err := k8s.SaveZarfState(state); err != nil {
		message.Fatal(err, "Unable to save the Zarf state data back to the cluster")
	}

	// Load state for the rest of the operations
	config.InitState(state)

	if !config.IsTLSLocalhost() {
		k8s.ReplaceTLSSecret("kube-system", "tls-pem")
		k8s.ReplaceTLSSecret(k8s.ZarfNamespace, "tls-pem")
	}

	registrySecret := config.GetSecret(config.StateRegistryPush)
	// Now that we have what the password will be, we should add the login entry to the system's registry config
	if err := utils.Login(config.ZarfRegistry, config.ZarfRegistryPushUser, registrySecret); err != nil {
		message.Fatal(err, "Unable to add login credentials for the gitops registry")
	}
}

func postSeedRegistry() {
	message.Debug("packager.postSeedRegistry()")
	tunnel := k8s.NewZarfTunnel()
	tunnel.Connect(k8s.ZarfRegistry, false)

	seedImages := config.GetSeedImages()
	for _, image := range seedImages {
		src := utils.SwapHost(image, config.ZarfSeedRegistry)
		dest := utils.SwapHost(image, config.ZarfRegistry)
		images.Copy(src, dest)
	}
	tunnel.Close()
}

func prepareImageForImport(tempPath tempPaths, prefix string) (string, string) {
	message.Debugf("packager.makeSeedTarball(%v)", tempPath)

	// Always use localhost since we're not really doing anything with it
	config.TLS.Host = config.IPV4Localhost

	ctx, err := k8s.GetContext()
	if err != nil {
		message.Error(err, "Unable to auto-inject the registry image into KIND")
		return "", ""
	}
	clusterName := strings.Replace(ctx, prefix+"-", "", 1)

	// Start the registry now for loading the image
	startEmbeddedRegistry(true)

	seedImages := config.GetSeedImages()
	path := tempPath.base + "/seed-registry.tar"
	for idx, image := range seedImages {
		// Pull them all from the seed registry
		seedImages[idx] = utils.SwapHost(image, config.GetSeedRegistry())
	}
	images.PullAll(seedImages, path)
	return path, clusterName
}
