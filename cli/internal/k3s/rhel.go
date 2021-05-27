package k3s

import (
	log "github.com/sirupsen/logrus"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/shift/pack/cli/internal/utils"
)

func ConfigureRHEL() {
	rpmPath := utils.AssetPath("rpms/*.rpm")
	rancherKeyPath := utils.AssetPath("rpms/rancher.key")

	log.Info("Setting up RHEL-specific dependenices and configs")
	if utils.InvalidPath(rancherKeyPath) {
		log.Fatal("Package missing RHEL dependencies.  Please ensure this package with built with RHEL=7 or RHEL=8 in the env file.  Refer to the README for more details.")
	}

	// @todo: k3s docs recommend disabling this, but we should look at just tuning it appropriately
	utils.ExecCommand([]string{}, "systemctl", "disable", "firewalld", "--now")

	// Import the rancher gpg for RPM install
	utils.ExecCommand([]string{}, "rpm", "--import", rancherKeyPath)

	// Install the rpms, have to pass into a shell so yum doesn't explode with the filename wildcard mathching
	utils.ExecCommand([]string{}, "sh", "-c", "yum localinstall -y --disablerepo=* --exclude container-selinux-1* "+rpmPath)
}
