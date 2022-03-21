package clusters

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	k3dCluster "github.com/rancher/k3d/v5/cmd/cluster"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	kindCluster "sigs.k8s.io/kind/pkg/cluster"
	kindcmd "sigs.k8s.io/kind/pkg/cmd"
)

// DistroToUse is an "enum" for helping determine which k8s distro to run the tests on.
type DistroToUse int

const (
	// DistroUnknown is the "enum" representation for when we don't know what distro the user wants.
	DistroUnknown DistroToUse = iota
	// DistroProvided is the "enum" representation for when the user wants to use the k8s cluster that is already present.
	DistroProvided
	// DistroKind is the "enum" representation for when the user wants the test suite to set up its own KinD cluster.
	DistroKind
	// DistroK3d is the "enum" representation for when the user wants the test suite to set up its own K3d cluster.
	DistroK3d
	// DistroK3s is the "enum" representation for when the user wants the test suite to use Zarf's built-in K3s cluster.
	DistroK3s

	kindClusterName = "kind-zarf-test"
	k3dClusterName  = "k3d-zarf-test"
)

// GetDistroToUseFromString decides which cluster the user wants based on the string value passed from an environment
// variable.
func GetDistroToUseFromString(s string) (DistroToUse, error) {
	match := map[string]DistroToUse{
		"provided": DistroProvided,
		"kind":     DistroKind,
		"k3d":      DistroK3d,
		"k3s":      DistroK3s,
	}
	if distroToUse, ok := match[s]; ok {
		return distroToUse, nil
	} else {
		return DistroUnknown, fmt.Errorf("\"%v\" is not a valid value for determining which k8s distro to use", s)
	}
}

// CreateClusterWithTemporaryKubeconfig creates a temporary Kubeconfig file, updates the process's KUBECONFIG env var
// to point at the new file, and creates a kubernetes cluster. It returns the path to the temporary Kubeconfig file,
// or an error if something went wrong. The caller is responsible for destroying the cluster and deleting the temporary
// kubeconfig file. The KUBECONFIG env var doesn't need to be reset since it was only changed for the current process.
func CreateClusterWithTemporaryKubeconfig(distroToUse DistroToUse) (string, error) {
	// Create the temporary Kubeconfig file
	var clusterName string
	switch distroToUse {
	case DistroKind:
		clusterName = kindClusterName
	case DistroK3d:
		clusterName = k3dClusterName
	}
	tempKubeconfigFile, err := ioutil.TempFile("", clusterName+"*.yaml")
	if err != nil {
		return "", err
	}

	// Set the KUBECONFIG env var to the new file
	err = os.Setenv("KUBECONFIG", tempKubeconfigFile.Name())
	if err != nil {
		return "", err
	}

	// Create the cluster
	switch distroToUse {
	case DistroKind:
		err = createKindCluster(tempKubeconfigFile.Name())
		if err != nil {
			return "", err
		}
	case DistroK3d:
		err = createK3dClusterUsingCurrentKubeconfig()
		if err != nil {
			return "", err
		}
	}
	return tempKubeconfigFile.Name(), nil
}

// DeleteClusterAndTemporaryKubeconfig is intended to be deferred at the end of TestMain execution so we can clean up
// the cluster and temp file that we made.
func DeleteClusterAndTemporaryKubeconfig(distroToUse DistroToUse, tempKubeconfigFilePath string) error {
	// Defer the temp file delete so it always happens no matter what
	defer func(name string) {
		_ = os.Remove(name)
	}(tempKubeconfigFilePath)

	// Delete the cluster
	switch distroToUse {
	case DistroKind:
		err := deleteKindCluster(tempKubeconfigFilePath)
		if err != nil {
			return err
		}
	case DistroK3d:
		err := deleteK3dCluster()
		if err != nil {
			return err
		}
	}
	return nil
}

// TryValidateClusterIsRunning establishes a valid connection to the currently configured cluster, then returns,
// throwing an error if anything went wrong.
func TryValidateClusterIsRunning() error {
	clientSet, err := GetClientSet()
	if err != nil {
		return fmt.Errorf("unable to connect to the cluster: %w", err)
	}
	pods, err := clientSet.CoreV1().Pods("kube-system").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("unable to list running pods: %w", err)
	}
	if len(pods.Items) < 1 {
		return fmt.Errorf("established connection to cluster, but no pods were found")
	}

	return nil
}

// GetConfig gets the currently configured Kubeconfig, or throws an error if something went wrong.
func GetConfig() (*rest.Config, error) {
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return nil, err
	}
	return config, nil
}

// GetClientSet establishes a connection with the currently configured k8s cluster and returns the ClientSet object
// that is needed to interact with it.
func GetClientSet() (*kubernetes.Clientset, error) {
	config, err := GetConfig()
	if err != nil {
		return nil, err
	}

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return clientSet, nil
}

// createKindCluster creates a KinD cluster given the path to use to create a kubeconfig file.
// It returns an error if something went wrong.
func createKindCluster(kubeconfigFile string) error {
	provider := kindCluster.NewProvider(kindCluster.ProviderWithLogger(kindcmd.NewLogger()))
	err := provider.Create(
		kindClusterName,
		kindCluster.CreateWithNodeImage(""),
		kindCluster.CreateWithRetain(false),
		kindCluster.CreateWithWaitForReady(time.Duration(0)),
		kindCluster.CreateWithKubeconfigPath(kubeconfigFile),
		kindCluster.CreateWithDisplayUsage(false))
	if err != nil {
		return err
	}
	// The cluster needs a bit more time to become healthy
	err = waitForHealthyCluster()
	if err != nil {
		return err
	}
	return nil
}

// deleteKindCluster deletes the KinD cluster that was created at the beginning of the test suite, and removes the
// cluster configuration from the provided Kubeconfig file. It does not delete the file itself.
func deleteKindCluster(kubeconfigFile string) error {
	provider := kindCluster.NewProvider(kindCluster.ProviderWithLogger(kindcmd.NewLogger()))
	err := provider.Delete(kindClusterName, kubeconfigFile)
	if err != nil {
		return err
	}
	return nil
}

// createK3dClusterUsingCurrentKubeconfig creates a K3d cluster. It will automatically modify the currently configured
// Kubeconfig file and switch the current context to the new cluster. It returns an error if something went wrong.
func createK3dClusterUsingCurrentKubeconfig() error {
	// Guide on how to do args and flags:
	// https://stackoverflow.com/questions/49848898/cobra-how-to-set-flags-programmatically-in-tests/50880663#50880663
	createClusterCommand := k3dCluster.NewCmdClusterCreate()
	createClusterCommand.SetArgs([]string{
		k3dClusterName,
		"--kubeconfig-update-default=true",
		"--kubeconfig-switch-context=true",
		"--wait=true",
		"--timeout=120s",
	})
	err := createClusterCommand.ExecuteContext(context.TODO())
	if err != nil {
		return err
	}
	// The cluster needs a bit more time to become healthy
	err = waitForHealthyCluster()
	if err != nil {
		return err
	}

	return nil
}

// deleteK3dCluster deletes the K3d cluster that was created at the beginning of the test suite. It uses the currently
// configured Kubeconfig file (from the KUBECONFIG env var).
func deleteK3dCluster() error {
	deleteClusterCommand := k3dCluster.NewCmdClusterDelete()
	deleteClusterCommand.SetArgs([]string{
		k3dClusterName,
	})
	err := deleteClusterCommand.ExecuteContext(context.TODO())
	if err != nil {
		return err
	}

	return nil
}

// waitForHealthyCluster can be used to wait until the cluster is healthy if it needs a little extra time to spin up.
func waitForHealthyCluster() error {
	attempt := 0
	success := false
	for attempt < 15 {
		err := TryValidateClusterIsRunning()
		if err == nil {
			success = true
			break
		} else {
			time.Sleep(1 * time.Second)
			attempt++
		}
	}
	if success {
		return nil
	} else {
		return fmt.Errorf("the cluster did not become healthy in time")
	}
}
