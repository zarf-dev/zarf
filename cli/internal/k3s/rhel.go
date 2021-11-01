package k3s

import (
	"github.com/defenseunicorns/zarf/cli/internal/utils"
	"github.com/sirupsen/logrus"
)

func configureRHEL() {
	// @todo: k3s docs recommend disabling this, but we should look at just tuning it appropriately
	_, err := utils.ExecCommand(nil, "systemctl", "disable", "firewalld", "--now")
	if err != nil {
		logrus.Debug(err)
		logrus.Warn("Unable to disable the firewall")
	}
}
