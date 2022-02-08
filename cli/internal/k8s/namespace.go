package k8s

import (
	"context"
	"github.com/defenseunicorns/zarf/cli/internal/message"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"time"
)

func GetNamespaces() (*corev1.NamespaceList, error) {
	clientset := getClientset()

	metaOptions := metav1.ListOptions{}
	return clientset.CoreV1().Namespaces().List(context.TODO(), metaOptions)
}

func CreateNamespace(name string) (*corev1.Namespace, error) {
	message.Debugf("k8s.CreateNamespace(%s)", name)

	clientset := getClientset()

	namespace := &corev1.Namespace{
		TypeMeta:   metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "Namespace"},
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}

	metaOptions := metav1.GetOptions{}
	createOptions := metav1.CreateOptions{}

	match, err := clientset.CoreV1().Namespaces().Get(context.TODO(), name, metaOptions)

	message.Debug(match)

	if err != nil || match.Name != name {
		return clientset.CoreV1().Namespaces().Create(context.TODO(), namespace, createOptions)
	}

	return match, err
}

func DeleteZarfNamespace() {
	spinner := message.NewProgressSpinner("Deleting the zarf namespace from this cluster")
	defer spinner.Stop()

	clientset := getClientset()
	err := clientset.CoreV1().Namespaces().Delete(context.TODO(), ZarfNamespace, metav1.DeleteOptions{})

	if err != nil && !errors.IsNotFound(err) {
		spinner.Fatalf(err, "the Zarf namespace could not be deleted")
	}

	for {
		spinner.Updatef("Zarf namespace deletion scheduled, waiting for all resources to be removed")
		_, err := clientset.CoreV1().Namespaces().Get(context.TODO(), ZarfNamespace, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			spinner.Successf("Zarf removed from this cluster")
			os.Exit(0)
		}
		time.Sleep(1 * time.Second)
	}
}
