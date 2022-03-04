package k8s

import (
	"context"
	"time"

	"github.com/defenseunicorns/zarf/cli/internal/message"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetNamespaces() (*corev1.NamespaceList, error) {
	clientset := getClientset()

	metaOptions := metav1.ListOptions{}
	return clientset.CoreV1().Namespaces().List(context.TODO(), metaOptions)
}

func CreateNamespace(name string, namespace *corev1.Namespace) (*corev1.Namespace, error) {
	message.Debugf("k8s.CreateNamespace(%s)", name)

	clientset := getClientset()

	if namespace == nil {
		// if only a name was provided create the namespace object
		namespace = &corev1.Namespace{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Namespace",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					// track the creation of this ns by zarf
					"app.kubernetes.io/managed-by": "zarf",
				},
			},
		}
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
	// Get the zarf ns and ignore errors
	namespace, _ := clientset.CoreV1().Namespaces().Get(context.TODO(), ZarfNamespace, metav1.GetOptions{})
	// Remove the k8s finalizer to speed up destroy
	_, _ = clientset.CoreV1().Namespaces().Finalize(context.TODO(), namespace, metav1.UpdateOptions{})

	// Attempt to delete the namespace
	err := clientset.CoreV1().Namespaces().Delete(context.TODO(), ZarfNamespace, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		spinner.Fatalf(err, "the Zarf namespace could not be deleted")
	}

	spinner.Updatef("Zarf namespace deletion scheduled, waiting for all resources to be removed")
	for {
		// Keep checking for the
		_, err := clientset.CoreV1().Namespaces().Get(context.TODO(), ZarfNamespace, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			spinner.Successf("Zarf removed from this cluster")
			return
		}
		time.Sleep(1 * time.Second)
	}
}
