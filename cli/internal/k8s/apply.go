package k8s

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

const applyTimeout = time.Minute * 2

func ApplyManifest(path string) {
	logContext := logrus.WithField("path", path)

	_, kubeClient := connect()

	file, err := os.Open(path)
	if err != nil {
		logContext.Fatal("Unable to read the manifest file")
	}
	defer file.Close()

	resources, err := kubeClient.Build(file, true)
	if err != nil {
		logContext.Info("Could not parse the manifest, sleeping before retrying")
		time.Sleep(30 * time.Second)
		ApplyManifest(path)
		return
	}

	if err := kubeClient.Wait(resources, applyTimeout); err != nil {
		logContext.Warn("Timeout occured waiting for resource, continuing...")
	}

	_, err = kubeClient.Update(resources, resources, true)
	if err != nil {
		logContext.Warn("Unable to apply the manifest file")
	}
}

func debug(format string, v ...interface{}) {
	format = fmt.Sprintf("%s\n", format)
	log.Output(1, fmt.Sprintf(format, v...))
}
