package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/defenseunicorns/zarf/cli/internal/message"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/distribution/distribution/v3/configuration"
	"github.com/distribution/distribution/v3/registry"
	_ "github.com/distribution/distribution/v3/registry/auth/htpasswd"             // used for embedded registry
	_ "github.com/distribution/distribution/v3/registry/storage/driver/filesystem" // used for embedded registry
)

const (
	ZarfSeedReadPort    = "5000"
	ZarfSeedWritePort   = "5001"
	ZarfSeedWriteTarget = "127.0.0.1:5001"
)

func main() {
	// Don't show the zarf logo constantly
	zarfLogo := message.GetLogo()
	_, _ = fmt.Fprintln(os.Stderr, zarfLogo)
	message.SetLogLevel(message.TraceLevel)
	path, seedImages := os.Args[1], os.Args[2:]
	LoadInternalSeedRegistry(path, seedImages)
}

func LoadInternalSeedRegistry(path string, seedImages []string) {
	// Launch the embedded registry to load the seed images (r/w mode)
	startSeedRegistry(false)

	cranePlatform := crane.WithPlatform(&v1.Platform{OS: "linux", Architecture: runtime.GOARCH})

	spinner := message.NewProgressSpinner("Storing images in the zarf registry")
	defer spinner.Stop()

	for _, src := range seedImages {
		spinner.Updatef("Updating image %s", src)
		img, err := crane.LoadTag(path, src, cranePlatform)
		if err != nil {
			spinner.Errorf(err, "Unable to load the image from the update package")
			return
		}

		offlineName := utils.SwapHost(src, ZarfSeedWriteTarget)

		err = crane.Push(img, offlineName, cranePlatform)
		if err != nil {
			spinner.Errorf(err, "Unable to push the image to the registry")
		}
	}

	spinner.Success()

	// Now start the registry read-only and wait for exit
	startSeedRegistry(true)

	// Keep this open until an interrupt signal is received
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

func startSeedRegistry(readOnly bool) {
	message.Debugf("packager.startSeedRegistry(%v)", readOnly)
	registryConfig := &configuration.Configuration{}

	if message.GetLogLevel() >= message.DebugLevel {
		registryConfig.Log.Level = "debug"
	} else {
		registryConfig.Log.AccessLog.Disabled = true
		registryConfig.Log.Formatter = "text"
		registryConfig.Log.Level = "error"
	}

	registryConfig.HTTP.DrainTimeout = 0
	registryConfig.HTTP.Secret = utils.RandomString(20)

	fileStorage := configuration.Parameters{
		"rootdirectory": ".zarf-registry",
	}

	if readOnly {
		// Read-only binds to all addresses
		registryConfig.HTTP.Addr = ":" + ZarfSeedReadPort
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
		registryConfig.HTTP.Addr = ZarfSeedWriteTarget
		registryConfig.Storage = configuration.Storage{
			"filesystem": fileStorage,
		}
	}

	message.Debug(registryConfig)

	embeddedRegistry, err := registry.NewRegistry(context.TODO(), registryConfig)
	if err != nil {
		message.Fatal(err, "Unable to start the embedded registry")
	}

	go func() {
		if err := embeddedRegistry.ListenAndServe(); err != nil {
			message.Fatal(err, "Unable to start the embedded registry")
		}
	}()

}
