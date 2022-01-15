package k8s

import (
	"fmt"
	"io/ioutil"
	"os"
	"regexp"

	"github.com/defenseunicorns/zarf/cli/internal/message"
	"github.com/defenseunicorns/zarf/cli/internal/template"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// ImageSwap Pre-compute all the replacements for the embedded registry
type ImageSwap struct {
	find    string
	replace string
}

func getRestConfig() *rest.Config {
	homePath, err := os.UserHomeDir()
	if err != nil {
		message.Fatal(nil, "Unable to load the current user's home directory")
	}

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", homePath+"/.kube/config")
	if err != nil {
		message.Fatalf(err, "Unable to connect to the K8s cluster")
	}
	return config
}

func getClientset() *kubernetes.Clientset {
	config := getRestConfig()
	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		message.Fatal(err, "Unable to connect to the K8s cluster")
	}

	return clientset
}

// readFile just reads a file into a byte array.
func readFile(file string) ([]byte, error) {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		message.Debug(err)
		return []byte{}, fmt.Errorf("cannot read file %v, %v", file, err)
	}
	return b, nil
}

func GetContext() (string, error) {
	kubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	)
	kubeconfig.ConfigAccess().GetLoadingPrecedence()
	kubeConf, err := kubeconfig.ConfigAccess().GetStartingConfig()
	if err != nil {
		return "", fmt.Errorf("unable to load the default kube config")
	}

	return kubeConf.CurrentContext, nil
}

// ProcessYamlFilesInPath iterates over all yaml files in a given path and performs Zarf templating + image swapping
func ProcessYamlFilesInPath(path string, componentImages []string) []string {
	message.Debugf("k8s.ProcessYamlFilesInPath(%s, %v)", path, componentImages)

	// Only pull in yml and yaml files
	pattern := regexp.MustCompile(`(?mi)\.ya?ml$`)
	manifests := utils.RecursiveFileList(path, pattern)
	valueTemplate := template.Generate()

	// Match images in the given list and replace if found in the given files
	var imageSwap []ImageSwap
	for _, image := range componentImages {
		imageSwap = append(imageSwap, ImageSwap{
			find:    image,
			replace: utils.SwapHost(image, valueTemplate.GetRegistry()),
		})
	}

	for _, manifest := range manifests {
		message.Debugf("Processing k8s manifest files %s", manifest)
		// Iterate over each image swap to see if it exists in the manifest
		for _, swap := range imageSwap {
			utils.ReplaceText(manifest, swap.find, swap.replace)
		}
		valueTemplate.Apply(manifest)
	}

	return manifests
}
