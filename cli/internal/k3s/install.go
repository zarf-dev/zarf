package k3s

import (
	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/git"
	"github.com/defenseunicorns/zarf/cli/internal/packager"
	"github.com/defenseunicorns/zarf/cli/internal/pki"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
	"github.com/sirupsen/logrus"
)

type InstallOptions struct {
	PKI        pki.PKIConfig
	Confirmed  bool
	Components string
}

func Install(options *InstallOptions) {
	utils.RunPreflightChecks()

	logrus.Info("Initializing a new zarf cluster")

	// Genereate or create the zarf secret
	gitSecret := git.GetOrCreateZarfSecret()

	// Convert to htpassword for the embedded registry
	zarfHtPassword, err := utils.GetHtpasswdString(config.ZarfGitUser, gitSecret)
	if err != nil {
		logrus.Debug(err)
		logrus.Fatal("Unable to define `htpasswd` string for the Zarf user")
	}

	// Write the htpassword to the embedded registry target file
	utils.WriteFile("/etc/zarf-registry-htpasswd", []byte(zarfHtPassword))

	// Now that we have what the password will be, we should add the login entry to the system's registry config
	err1 := utils.Login(config.ZarfLocalIP+":45000", config.ZarfGitUser, gitSecret)
	err2 := utils.Login(config.ZarfLocalIP, config.ZarfGitUser, gitSecret)
	if err1 != nil || err2 != nil {
		logrus.Debug(err1)
		logrus.Debug(err2)
		logrus.Fatal("Unable to add login credentials for the gitops registry")
	}

	// We really need to make sure this is still necessary....
	if utils.IsRHEL() {
		// @todo: k3s docs recommend disabling this, but we should look at just tuning it appropriately
		_, err := utils.ExecCommand(true, nil, "systemctl", "disable", "firewalld", "--now")
		if err != nil {
			logrus.Debug(err)
			logrus.Warn("Unable to disable the firewall")
		}
	}

	// Continue running package deploy for all components like any other package
	packager.Deploy(config.PackageInitName, options.Confirmed, options.Components)

	pki.InjectServerCert(options.PKI)

	logrus.Info("Installation complete.  You can run \"/usr/local/bin/k9s\" to monitor the status of the deployment.")
	logrus.WithFields(logrus.Fields{
		"Gitea Username (if installed)": config.ZarfGitUser,
		"Grafana Username":              "zarf-admin",
		"Password (all)":                gitSecret,
	}).Warn("Credentials stored in ~/.git-credentials")
}
