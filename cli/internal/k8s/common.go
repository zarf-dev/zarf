package k8s

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func getRestConfig() *rest.Config {

	homePath, err := os.UserHomeDir()
	if err != nil {
		logrus.Fatal("Unable to load the current user's home directory")
	}

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", homePath+"/.kube/config")
	if err != nil {
		logrus.Fatal("Unable to connect to the K8s cluster", err.Error())
	}
	return config
}

func getClientset() *kubernetes.Clientset {

	config := getRestConfig()
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
