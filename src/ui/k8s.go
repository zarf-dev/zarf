package ui

import (
	"time"

	"github.com/defenseunicorns/zarf/src/internal/k8s"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/types"
)

// App struct
type K8s struct {
}

// NewApp creates a new App application struct
func NewK8s() *K8s {
	return &K8s{}
}

func (k *K8s) ViewState() types.ZarfState {
	spinner := message.NewProgressSpinner("Gathering cluster information")
	defer spinner.Stop()

	if err := k8s.WaitForHealthyCluster(5 * time.Minute); err != nil {
		spinner.Fatalf(err, "The cluster we are using never reported 'healthy'")
	}

	return k8s.LoadZarfState()
}
