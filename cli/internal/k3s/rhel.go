package k3s

import (
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/utils"
)

func ConfigureRHEL() {
	// @todo: k3s docs recommend disabling this, but we should look at just tuning it appropriately
	utils.ExecCommand([]string{}, "systemctl", "disable", "firewalld", "--now")
}
