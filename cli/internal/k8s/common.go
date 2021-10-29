package k8s

import (
	"fmt"
	"io/ioutil"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func connect() *kubernetes.Clientset {
	kubeconfig := "/root/.kube/config"

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		logrus.Fatal("Unable to connect to the K8s cluster", err.Error())
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logrus.Fatal("Unable to connect to the K8s cluster", err.Error())
	}

	return clientset
}

// readFile just reads a file into a byte array.
func readFile(file string) ([]byte, error) {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		logrus.Debug(err)
		return []byte{}, fmt.Errorf("cannot read file %v, %v", file, err)
	}
	return b, nil
}
