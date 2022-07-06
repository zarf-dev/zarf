package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/distribution/distribution/v3/configuration"
	"github.com/distribution/distribution/v3/registry"
	_ "github.com/distribution/distribution/v3/registry/auth/htpasswd"             // used for embedded registry
	_ "github.com/distribution/distribution/v3/registry/storage/driver/filesystem" // used for embedded registry
)

const (
	ZarfSeedReadPort    = "5000"
	ZarfSeedWriteTarget = "127.0.0.1:5001"
)

func main() {
	path, seedImage, targetImage := os.Args[1], os.Args[2], os.Args[3]

	// Launch the embedded registry to load the seed images (r/w mode)
	startSeedRegistry(false)

	cranePlatform := crane.WithPlatform(&v1.Platform{OS: "linux", Architecture: runtime.GOARCH})

	log.Printf("Updating image %s\n", seedImage)
	img, err := crane.LoadTag(path, seedImage, cranePlatform)
	if err != nil {
		log.Fatalf("Unable to load the image from the update package: %s", err)
	}

	err = crane.Push(img, targetImage, cranePlatform)
	if err != nil {
		log.Fatalf("Unable to push the image to the registry: %s", err)
	}

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
	log.Printf("packager.startSeedRegistry(%v)\n", readOnly)
	registryConfig := &configuration.Configuration{}

	registryConfig.Log.Level = "debug"
	registryConfig.HTTP.DrainTimeout = 0

	fileStorage := configuration.Parameters{
		"rootdirectory": ".zarf-registry",
	}

	if readOnly {
		// Read-only binds to all addresses
		registryConfig.HTTP.Addr = ":" + ZarfSeedReadPort
		registryConfig.Storage = configuration.Storage{
			"filesystem": fileStorage,
			"maintenance": configuration.Parameters{
				"readonly": map[any]any{
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

	log.Print(registryConfig)

	embeddedRegistry, err := registry.NewRegistry(context.TODO(), registryConfig)
	if err != nil {
		log.Fatalf("Unable to start the embedded registry: %s", err)
	}

	go func() {
		if err := embeddedRegistry.ListenAndServe(); err != nil {
			log.Fatalf("Unable to start the embedded registry: %s", err)
		}
	}()

}
