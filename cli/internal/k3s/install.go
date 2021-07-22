package k3s

import (
	"os"

	"github.com/sirupsen/logrus"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/config"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/git"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/packager"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/utils"
)

func Install(host string, applianceMode bool, certPublicPath string, certPrivatePath string) {

	utils.RunPreflightChecks()

	logrus.Info("Installing K3s")

	if applianceMode {
		packager.Deploy(config.PackageApplianceName)
	} else {
		packager.Deploy(config.PackageInitName)
	}

	// Install RHEL RPMs if applicable
	if utils.IsRHEL() {
		ConfigureRHEL()
	}

	// Create the K3s systemd service
	createService()

	createK3sSymlinks()

	// Get a random secret for use in the cluster
	gitSecret := utils.RandomString(28)

	// Get a list of all the k3s manifest files
	manifests := utils.RecursiveFileList(config.K3sManifestPath)

	// Iterate through all the manifests and replace any ZARF_SECRET values
	for _, manifest := range manifests {
		utils.ReplaceText(manifest, "###ZARF_SECRET###", gitSecret)
	}

	// Add the secret to git-credentials for push to gitea
	git.CredentialsGenerator(host, "syncuser", gitSecret)

	if certPublicPath != "" && certPrivatePath != "" {
		logrus.WithFields(logrus.Fields{
			"public":  certPublicPath,
			"private": certPrivatePath,
		}).Info("Injecting user-provided keypair for ingress TLS")
		utils.InjectServerCert(certPublicPath, certPrivatePath)
	} else {
		utils.GeneratePKI(host)
	}

	logrus.Info("Installation complete.  You can run \"/usr/local/bin/k9s\" to monitor the status of the deployment.")
	logrus.WithFields(logrus.Fields{
		"Gitea Username":   "syncuser",
		"Grafana Username": "zarf-admin",
		"Password (all)":   gitSecret,
	}).Warn("Credentials stored in ~/.git-credentials")
}

func createK3sSymlinks() {
	logrus.Info("Creating kube config symlink")

	// Make the k3s kubeconfig available to other standard K8s tools that bind to the default ~/.kube/config
	err := utils.CreateDirectory("/root/.kube", 0700)
	if err != nil {
		logrus.Warn("Unable to create the root kube config directory")
	} else {
		// Dont log an error for now since re-runs throw an invalid error
		_ = os.Symlink("/etc/rancher/k3s/k3s.yaml", "/root/.kube/config")
	}

	// Add aliases for k3s
	_ = os.Symlink("/usr/local/bin/k3s", "/usr/local/bin/kubectl")
	_ = os.Symlink("/usr/local/bin/k3s", "/usr/local/bin/ctr")
	_ = os.Symlink("/usr/local/bin/k3s", "/usr/local/bin/crictl")
}

func createService() {
	serviceDefinition := []byte(`[Unit]
Description=Zarf K3s Runner
Documentation=https://repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf
Wants=network-online.target
After=network-online.target

[Install]
WantedBy=multi-user.target

[Service]
Type=notify
EnvironmentFile=-/etc/default/%N
EnvironmentFile=-/etc/sysconfig/%N
KillMode=process
Delegate=yes
# Having non-zero Limit*s causes performance problems due to accounting overhead
# in the kernel. We recommend using cgroups to do container-local accounting.
LimitNOFILE=1048576
LimitNPROC=infinity
LimitCORE=infinity
TasksMax=infinity
TimeoutStartSec=0
Restart=always
RestartSec=5s
ExecStartPre=/bin/sh -xc '! /usr/bin/systemctl is-enabled --quiet nm-cloud-setup.service'
ExecStartPre=-/sbin/modprobe br_netfilter
ExecStartPre=-/sbin/modprobe overlay
ExecStart=/usr/local/bin/k3s server --write-kubeconfig-mode=700
`)

	servicePath := "/etc/systemd/system/k3s.service"

	utils.WriteFile(servicePath, serviceDefinition)

	_ = os.Symlink(servicePath, "/etc/systemd/system/multi-user.target.wants/k3s.service")

	utils.ExecCommand(nil, "systemctl", "daemon-reload")
	utils.ExecCommand(nil, "systemctl", "enable", "--now", "k3s")
}
