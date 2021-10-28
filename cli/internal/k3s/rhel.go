package k3s

import (
	"github.com/defenseunicorns/zarf/cli/internal/log"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
)

func configureRHEL() {
	// @todo: k3s docs recommend disabling this, but we should look at just tuning it appropriately
	_, err := utils.ExecCommand(nil, "systemctl", "disable", "firewalld", "--now")
	if err != nil {
		log.Logger.Warn("Unable to disable the firewall")
	}
}
