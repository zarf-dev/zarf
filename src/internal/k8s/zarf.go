package k8s

import (
	"context"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/message"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func StripZarfLabelsAndSecretsFromNamespaces() {
	spinner := message.NewProgressSpinner("Removing zarf metadata & secrets from existing namespaces not managed by Zarf")
	defer spinner.Stop()

	clientSet := getClientset()
	deleteOptions := metav1.DeleteOptions{}
	listOptions := metav1.ListOptions{
		LabelSelector: config.ZarfManagedByLabel + "=zarf",
	}

	if namespaces, err := GetNamespaces(); err != nil {
		spinner.Errorf(err, "Unable to get k8s namespaces")
	} else {
		for _, namespace := range namespaces.Items {
			if _, ok := namespace.Labels["zarf.dev/agent"]; ok {
				spinner.Updatef("Removing Zarf Agent label for namespace %s", namespace.Name)
				delete(namespace.Labels, "zarf.dev/agent")
				if _, err = UpdateNamespace(&namespace); err != nil {
					// This is not a hard failure, but we should log it
					spinner.Errorf(err, "Unable to update the namespace labels for %s", namespace.Name)
				}
			}

			for _, namespace := range namespaces.Items {
				spinner.Updatef("Removing Zarf secrets for namespace %s", namespace.Name)
				err := clientSet.CoreV1().
					Secrets(namespace.Name).
					DeleteCollection(context.TODO(), deleteOptions, listOptions)
				if err != nil {
					spinner.Errorf(err, "Unable to delete secrets from namespace %s", namespace.Name)
				}
			}
		}
	}

	spinner.Success()
}
