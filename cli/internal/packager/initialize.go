package packager

import (
	"os"

	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/message"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
	_ "github.com/distribution/distribution/v3/registry/auth/htpasswd"             // used for embedde registry
	_ "github.com/distribution/distribution/v3/registry/storage/driver/filesystem" // used for embedded registry
	"github.com/pterm/pterm"
)

func Install() {
	utils.RunPreflightChecks()

	message.Info("Initializing a new zarf cluster")

	// We really need to make sure this is still necessary....
	if utils.IsRHEL() {
		// @todo: k3s docs recommend disabling this, but we should look at just tuning it appropriately
		if _, err := utils.ExecCommand(true, nil, "systemctl", "disable", "firewalld", "--now"); err != nil {
			message.Error(err, "Unable to disable the firewall")
		}
	}

	// Continue running package deploy for all components like any other package
	config.DeployOptions.PackagePath = config.PackageInitName
	Deploy()

	// Cleanup the embedded registry folder
	_ = os.Remove(".zarf-registry")

	message.Info("Installation complete.")

	_ = pterm.DefaultTable.WithHasHeader().WithData(pterm.TableData{
		{"Application", "Username", "Password", "Connect"},
		{"Logging", "zarf-admin", config.GetSecret(config.StateLogging), "zarf connect logging"},
		{"Git", config.ZarfGitPushUser, config.GetSecret(config.StateGitPush), "zarf connect git"},
		{"Registry", "zarf-push-user", config.GetSecret(config.StateRegistryPush), "zarf connect registry"},
	}).Render()

	// All done
	os.Exit(0)
}
