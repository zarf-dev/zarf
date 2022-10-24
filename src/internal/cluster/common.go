package cluster

import (
	"time"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/k8s"
	"github.com/defenseunicorns/zarf/src/pkg/message"
)

type Cluster struct {
	Kube *k8s.Client
}

var defaultTimeout = 30 * time.Second

var labels = k8s.Labels{
	config.ZarfManagedByLabel: "zarf",
}

func NewClusterOrDie() *Cluster {
	c, err := NewClusterWithWait(defaultTimeout)
	if err != nil {
		message.Fatalf(err, "Failed to connect to cluster")
	}

	return c
}

func NewClusterWithWait(timeout time.Duration) (*Cluster, error) {
	c := &Cluster{}
	c.Kube, _ = k8s.NewK8sClient(message.Debugf, labels)
	return c, c.Kube.WaitForHealthyCluster(timeout)
}

func NewCluster() (*Cluster, error) {
	c := &Cluster{}
	c.Kube, _ = k8s.NewK8sClient(message.Debugf, labels)
	return c, nil
}
