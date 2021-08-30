package k8s

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/releaseutil"
	"k8s.io/apimachinery/pkg/util/wait"
)

const applyTimeout = time.Minute * 2
const waitInterval = 10 * time.Second

func ApplyManifest(manifest releaseutil.Manifest) {
	logContext := logrus.WithFields(logrus.Fields{
		"path": manifest.Name,
		"kind": manifest.Head.Kind,
		"name": manifest.Head.Metadata.Name,
	})

	logContext.Info("Applying K8s resource")

	_, kubeClient := connect()

	manifestContent := strings.NewReader(manifest.Content)

	stopChannel := make(chan struct{})

	wait.Until(func() {

		resources, err := kubeClient.Build(manifestContent, true)
		if err != nil {
			return
		}

		_, err = kubeClient.Update(resources, resources, true)
		if err != nil {
			logContext.Warn("Unable to apply the manifest file")
			return
		}

		if waitErr := kubeClient.Wait(resources, applyTimeout); waitErr != nil {
			logContext.Warn(waitErr)
			return
		}

		close(stopChannel)

	}, waitInterval, stopChannel)

}

func debug(format string, v ...interface{}) {
	format = fmt.Sprintf("%s\n", format)
	log.Output(1, fmt.Sprintf(format, v...))
}
