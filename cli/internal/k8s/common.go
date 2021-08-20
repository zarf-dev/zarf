package k8s

import (
	"fmt"
	"io/ioutil"

	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/kube"
	"k8s.io/client-go/kubernetes"
)

func connect() (*kubernetes.Clientset, kube.Interface) {
	actionConfig := new(action.Configuration)
	settings := cli.New()

	if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), "", debug); err != nil {
		logrus.Fatal("Unable to initialize the K8s client")
	}

	conf, err := actionConfig.RESTClientGetter.ToRESTConfig()
	if err != nil {
		logrus.Fatal("Unable to generate K8s client config")
	}

	clientset, err := kubernetes.NewForConfig(conf)
	if err != nil {
		logrus.Fatal("Unable to generate the K8s client connection")
	}

	return clientset, actionConfig.KubeClient
}

// ReadFile just reads a file into a byte array.
func ReadFile(file string) ([]byte, error) {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return []byte{}, fmt.Errorf("cannot read file %v, %v", file, err)
	}
	return b, nil
}
