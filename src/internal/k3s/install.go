package k3s

import (
	"os"

	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/shift/cli/src/internal/utils"

	log "github.com/sirupsen/logrus"
)

func Install() {

	log.Info("Installing K3s")

	utils.RunPreflightChecks()

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
	utils.ExecCommand([]string{}, "sh", "-c", k3sBinary+" kubectl completion bash >/etc/bash_completion.d/kubectl")
}
