package k3s

import (
	"os"

	"github.com/sirupsen/logrus"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/git"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/utils"
)

const k3sManifestPath = "/var/lib/rancher/k3s/server/manifests"

func Install(host string) {

	utils.RunPreflightChecks()

	logrus.Info("Installing K3s")

	utils.PlaceAsset("bin/k3s", "/usr/local/bin/k3s")
	utils.PlaceAsset("bin/k9s", "/usr/local/bin/k9s")
	utils.PlaceAsset("bin/init-k3s.sh", "/usr/local/bin/init-k3s.sh")
	utils.PlaceAsset("charts", "/var/lib/rancher/k3s/server/static/charts")
	utils.PlaceAsset("manifests", k3sManifestPath)
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

	utils.ExecCommand(envVariables, installer, "")

	// Get a random secret for use in the cluster
	gitSecret := utils.RandomString(28)

	// Get a list of all the k3s manifest files
	manifests := utils.RecursiveFileList(k3sManifestPath)

	// Iterate through all the manifests and replace any ZARF_SECRET values 
	for _, manifest := range manifests {
		utils.ReplaceText(manifest, "###ZARF_SECRET###", gitSecret)
	}

	// Add the secret to git-credentials for push to gitea
	git.CredentialsGenerator(host, "syncuser", gitSecret)

	// Make the k3s kubeconfig available to other standard K8s tools that bind to the default ~/.kube/config
	err := utils.CreateDirectory("/root/.kube", 0700)
	if err != nil {
		logrus.Warn("Unable to create the root kube config directory")
	} else {
		// Dont log an error for now since re-runs throw an invalid error
		_ = os.Symlink("/etc/rancher/k3s/k3s.yaml", "/root/.kube/config")
	}

	logrus.Info("Installation complete.  You can run \"/usr/loca/bin/k9s\" to monitor the status of the deployment.")
	logrus.Info("The login for gitea can be found in ~/.git-credentials")
}
