package packager

import (
	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/git"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
	"github.com/sirupsen/logrus"
)

type InstallOptions struct {
	Confirmed  bool
	Components string
	Generate   bool
}

func Install(options *InstallOptions) {
	utils.RunPreflightChecks()

	logrus.Info("Initializing a new zarf cluster")

	// Generate or create the zarf secret
	gitSecret := git.GetOrCreateZarfSecret()
	logrus.Debug("gitSecret", gitSecret)

	// Now that we have what the password will be, we should add the login entry to the system's registry config
	if err := utils.Login(config.GetTargetEndpoint(), config.ZarfGitUser, gitSecret); err != nil {
		logrus.Debug(err)
		logrus.Fatal("Unable to add login credentials for the gitops registry")
	}

	// We really need to make sure this is still necessary....
	if utils.IsRHEL() {
		// @todo: k3s docs recommend disabling this, but we should look at just tuning it appropriately
		if _, err := utils.ExecCommand(true, nil, "systemctl", "disable", "firewalld", "--now"); err != nil {
			logrus.Debug(err)
			logrus.Warn("Unable to disable the firewall")
		}
	}

	// Continue running package deploy for all components like any other package
	Deploy(config.PackageInitName, options.Confirmed, options.Components)

	logrus.Info("Installation complete.  You can run \"/usr/local/bin/k9s\" to monitor the status of the deployment.")
	logrus.WithFields(logrus.Fields{
		"Gitea Username (if installed)": config.ZarfGitUser,
		"Grafana Username":              "zarf-admin",
		"Password (all)":                gitSecret,
	}).Warn("Credentials stored in ~/.git-credentials")
}
