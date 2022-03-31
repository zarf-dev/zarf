package helm

import (
	"regexp"

	"github.com/defenseunicorns/zarf/src/internal/message"
	"helm.sh/helm/v3/pkg/action"
)

func Destroy(purgeAllZarfInstallations bool) {
	spinner := message.NewProgressSpinner("Removing Zarf-installed charts")
	defer spinner.Stop()

	// Initially load the actionConfig without a namespace
	actionConfig, err := createActionConfig("", spinner)
	if err != nil {
		// Don't fatal since this is a removal action
		spinner.Errorf(err, "Unable to initialize the K8s client")
		return
	}

	// Match a name that begins with "zarf-"
	// Explanation: https://regex101.com/r/3yzKZy/1
	zarfPrefix := regexp.MustCompile(`(?m)^zarf-`)

	// Get a list of all releases in all namespaces
	list := action.NewList(actionConfig)
	list.All = true
	list.AllNamespaces = true
	// Uninstall in reverse order
	list.ByDate = true
	list.SortReverse = true
	releases, err := list.Run()
	if err != nil {
		// Don't fatal since this is a removal action
		spinner.Errorf(err, "Unable to get the list of installed charts")
	}

	// Iterate over all releases
	for _, release := range releases {
		if !purgeAllZarfInstallations && release.Namespace != "zarf" {
			// Don't process releases outside the zarf namespace unless purgae all is true
			continue
		}
		// Filter on zarf releases
		if zarfPrefix.MatchString(release.Name) {
			spinner.Updatef("Uninstalling helm chart %s/%s", release.Namespace, release.Name)
			// Establish a new actionConfig for the namespace
			actionConfig, _ = createActionConfig(release.Namespace, spinner)
			// Perform the uninstall
			response, err := uninstallChart(actionConfig, release.Name)
			message.Debug(response)
			if err != nil {
				// Don't fatal since this is a removal action
				spinner.Errorf(err, "Unable to uninstall the chart")
			}
		}
	}

	spinner.Success()
}
