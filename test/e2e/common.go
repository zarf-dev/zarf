package test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"runtime"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/kind/pkg/cluster"
	kindcmd "sigs.k8s.io/kind/pkg/cmd"
)

type ZarfE2ETest struct {
	zarfBinPath string

	clusterName          string `default: "test-cluster"`
	kubeconfigPath       string
	kubeconfig           *os.File
	provider             *cluster.Provider
	restConfig           *restclient.Config
	clientset            *kubernetes.Clientset
	clusterAlreadyExists bool
}

func getKubeconfigPath() (string, error) {

	// Check if the $KUBECONFIG env is set
	// TODO: It would probably be good to verify a useable kubeconfig lives here
	kubeconfigEnv := os.Getenv("KUBECONFIG")
	if kubeconfigEnv != "" {
		return kubeconfigEnv, nil
	}

	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	configBaseDir := path.Join(userHomeDir, ".kube")
	if err := os.MkdirAll(configBaseDir, 0700); err != nil {
		return "", err
	}

	kubeconfigPath := path.Join(configBaseDir, "config")
	_, err = os.OpenFile(kubeconfigPath, os.O_RDWR|os.O_CREATE, 0755)
	return kubeconfigPath, err
}

func (e2e *ZarfE2ETest) setUp() error {

	// Determine what the name of the zarfBinary should be
	e2e.zarfBinPath = path.Join("../../build", getCLIName())

	var err error
	// Create or get the kubeconfig
	e2e.kubeconfigPath, err = getKubeconfigPath()
	if err != nil {
		return err
	}

	// Set up a KinD cluster if necessary
	e2e.provider = cluster.NewProvider(cluster.ProviderWithLogger(kindcmd.NewLogger()))
	nodes, err := e2e.provider.ListNodes(e2e.clusterName)
	if len(nodes) > 0 {
		// There already is a cluster up!! yay!!
		e2e.clusterAlreadyExists = true
	} else {
		err = e2e.provider.Create(
			e2e.clusterName,
			cluster.CreateWithNodeImage(""),
			cluster.CreateWithRetain(false),
			cluster.CreateWithWaitForReady(time.Duration(0)),
			cluster.CreateWithKubeconfigPath(e2e.kubeconfigPath),
			cluster.CreateWithDisplayUsage(false),
		)
	}

	// Get config and client for the k8s cluster
	e2e.restConfig, err = clientcmd.BuildConfigFromFlags("", e2e.kubeconfigPath)
	if err != nil {
		return err
	}
	e2e.clientset, err = kubernetes.NewForConfig(e2e.restConfig)
	if err != nil {
		return err
	}

	// Wait for the cluster to have pods before we let the test suite run
	// TODO: Pretty sure there's a cleaner way to do this
	attempt := 0
	for attempt < 10 {
		pods, err := e2e.clientset.CoreV1().Pods("kube-system").List(context.TODO(), metav1.ListOptions{})
		if err == nil && len(pods.Items) >= 0 {
			fmt.Printf("💥 Cluster %s ready. You can access it by setting:\nexport KUBECONFIG='%s'\n", e2e.clusterName, e2e.kubeconfigPath)
			break
		}

		time.Sleep(1 * time.Second)
		attempt++
		if attempt > 15 {
			return errors.New("unable to connect to KinD cluster for e2e tests")
		}
	}

	return err
}

func (e2e *ZarfE2ETest) tearDown() error {
	// Delete the cluster and kubeconfig file
	provider := cluster.NewProvider(cluster.ProviderWithLogger(kindcmd.NewLogger()))
	err := provider.Delete(e2e.clusterName, e2e.kubeconfigPath)
	os.Remove(e2e.kubeconfigPath)

	return err
}

func getCLIName() string {
	var binaryName string
	if runtime.GOOS == "linux" {
		binaryName = "zarf"
	} else if runtime.GOOS == "darwin" {
		if runtime.GOARCH == "arm64" {
			binaryName = "zarf-mac-apple"
		} else {
			binaryName = "zarf-mac-intel"
		}
	}
	return binaryName
}
