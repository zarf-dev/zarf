package k3s

import (
	"os"

	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/shift/pack/cli/internal/utils"

	log "github.com/sirupsen/logrus"
)

func Install() {

	utils.RunPreflightChecks()

	log.Info("Installing K3s")

	utils.PlaceAsset("bin/k3s", "/usr/local/bin/k3s")
	utils.PlaceAsset("bin/init-k3s.sh", "/usr/local/bin/init-k3s.sh")
	utils.PlaceAsset("charts", "/var/lib/rancher/k3s/server/static/charts")
	utils.PlaceAsset("manifests", "/var/lib/rancher/k3s/server/manifests")
	utils.PlaceAsset("images", "/var/lib/rancher/k3s/agent/images")

	installer := "/usr/local/bin/init-k3s.sh"
	k3sBinary := "/usr/local/bin/k3s"

	// Ensure k3s tools are executable / limit to root
	os.Chmod(installer, 0700)
	os.Chmod(k3sBinary, 0700)

	envVariables := []string{
		"K3S_KUBECONFIG_MODE=644",
		"INSTALL_K3S_SKIP_DOWNLOAD=true",
	}

	// Install RHEL RPMs if applicable
	if utils.IsRHEL() {
		ConfigureRHEL()
	}

	utils.ExecCommand(envVariables, installer, "--disable=metrics-server")

	// Make the k3s kubeconfig available to other standard K8s tools that bind to the default ~/.kube/config
	err := utils.CreateDirectory("/root/.kube", 0700)
	if err != nil {
		log.Warn("Unable to create the root kube config directory")
	} else {
		err := os.Symlink("/etc/rancher/k3s/k3s.yaml", "/root/.kube/config")
		if err != nil {
			log.Warn("Unable to link the root kube config file to k3s")
		}
	}
}
