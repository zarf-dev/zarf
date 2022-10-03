package cluster

import (
	"context"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/k8s"
	"github.com/defenseunicorns/zarf/src/pkg/message"
)

func DeleteZarfNamespace() {
	spinner := message.NewProgressSpinner("Deleting the zarf namespace from this cluster")
	defer spinner.Stop()

	k8s.DeleteNamespace(context.TODO(), config.ZarfNamespace)
}
