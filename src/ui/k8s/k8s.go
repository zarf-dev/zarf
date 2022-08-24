package k8s

import (
	"time"

	"github.com/defenseunicorns/zarf/src/internal/k8s"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/types"
)

func ListPods() {

}

func ViewState() types.ZarfState {
	spinner := message.NewProgressSpinner("Gathering cluster information")
	defer spinner.Stop()

	if err := k8s.WaitForHealthyCluster(5 * time.Minute); err != nil {
		spinner.Fatalf(err, "The cluster we are using never reported 'healthy'")
	}

	return types.ZarfState{}
}
