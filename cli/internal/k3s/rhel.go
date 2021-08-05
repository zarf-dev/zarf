package k3s

import (
	"github.com/sirupsen/logrus"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/utils"
)

func configureRHEL() {
	// @todo: k3s docs recommend disabling this, but we should look at just tuning it appropriately
	_, err := utils.ExecCommand(nil, "systemctl", "disable", "firewalld", "--now")
	if err != nil {
		logrus.Warn("Unable to disable the firewall")
	}
}
